package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/KodeWorks-1/techport/internal/services"
)

type checkoutData struct {
	cartData
	Methods []services.PaymentMethod
	Values  map[string]string
	Errors  map[string]string
}

func (h *Handlers) checkoutData(r *http.Request, values, errs map[string]string) (checkoutData, error) {
	cd, err := h.cartData(r)
	if err != nil {
		return checkoutData{}, err
	}
	methods, err := h.settings.PaymentMethods(r.Context())
	if err != nil {
		return checkoutData{}, err
	}
	if values == nil {
		values = map[string]string{"payment_method": "cod"}
		if u, ok := h.currentUser(r); ok {
			values["name"] = u.Name
			values["phone"] = u.Phone
			values["address"] = u.Address
			values["city"] = u.City
		}
	}
	return checkoutData{cartData: cd, Methods: methods, Values: values, Errors: errs}, nil
}

func (h *Handlers) Checkout(w http.ResponseWriter, r *http.Request) {
	data, err := h.checkoutData(r, nil, nil)
	if err != nil {
		slog.Error("checkout: load", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(data.Cart.Items) == 0 {
		http.Redirect(w, r, "/cart", http.StatusSeeOther)
		return
	}
	h.renderer.Render(w, "checkout.html", data)
}

var phoneRe = regexp.MustCompile(`^(?:\+?92|0)?(3\d{9})$`)

// normalizePhone returns a Pakistani mobile number as 03XXXXXXXXX, or "" if invalid.
func normalizePhone(raw string) string {
	cleaned := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(raw))
	m := phoneRe.FindStringSubmatch(cleaned)
	if m == nil {
		return ""
	}
	return "0" + m[1]
}

func (h *Handlers) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	values := map[string]string{
		"name":           strings.TrimSpace(r.FormValue("name")),
		"phone":          strings.TrimSpace(r.FormValue("phone")),
		"address":        strings.TrimSpace(r.FormValue("address")),
		"city":           strings.TrimSpace(r.FormValue("city")),
		"notes":          strings.TrimSpace(r.FormValue("notes")),
		"payment_method": r.FormValue("payment_method"),
	}

	errs := map[string]string{}
	if len(values["name"]) < 3 {
		errs["name"] = "Please enter your full name"
	}
	phone := normalizePhone(values["phone"])
	if phone == "" {
		errs["phone"] = "Enter a valid Pakistani mobile number, e.g. 03XX XXXXXXX"
	}
	if len(values["address"]) < 10 {
		errs["address"] = "Please enter your complete delivery address"
	}
	if values["city"] == "" {
		errs["city"] = "Please enter your city"
	}

	methods, err := h.settings.PaymentMethods(r.Context())
	if err != nil {
		slog.Error("checkout: methods", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	methodOK := false
	for _, m := range methods {
		if m.Key == values["payment_method"] && m.Enabled {
			methodOK = true
		}
	}
	if !methodOK {
		errs["payment_method"] = "This payment method is not available yet — please choose Cash on Delivery"
	}

	if len(errs) > 0 {
		data, derr := h.checkoutData(r, values, errs)
		if derr != nil {
			slog.Error("checkout: reload", "err", derr)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderer.Render(w, "checkout.html", data)
		return
	}

	var userID *int64
	user, loggedIn := h.currentUser(r)
	if loggedIn {
		userID = &user.ID
	}
	code, err := h.orders.Place(r.Context(), sessionID(r), services.PlaceOrderInput{
		CustomerName:  values["name"],
		Phone:         phone,
		Address:       values["address"],
		City:          values["city"],
		PaymentMethod: values["payment_method"],
		Notes:         values["notes"],
		UserID:        userID,
	})
	switch {
	case errors.Is(err, services.ErrEmptyCart):
		http.Redirect(w, r, "/cart", http.StatusSeeOther)
		return
	case errors.Is(err, services.ErrStockChanged):
		errs["form"] = "Sorry — an item in your cart just went out of stock. Please review your cart."
		data, derr := h.checkoutData(r, values, errs)
		if derr != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusConflict)
		h.renderer.Render(w, "checkout.html", data)
		return
	case err != nil:
		slog.Error("checkout: place order", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if loggedIn {
		if err := h.users.RememberAddress(r.Context(), user.ID, values["address"], values["city"]); err != nil {
			slog.Warn("checkout: remember address", "err", err)
		}
	}
	http.Redirect(w, r, "/order/"+code+"?placed=1", http.StatusSeeOther)
}

var orderSteps = []string{"pending", "confirmed", "shipped", "delivered"}

type orderStep struct {
	Key     string
	Label   string
	Done    bool
	Current bool
}

type orderData struct {
	services.OrderDetail
	Steps        []orderStep
	Placed       bool
	Terminated   bool // cancelled or returned
	WhatsAppLink string
}

func (h *Handlers) orderView(w http.ResponseWriter, r *http.Request, d services.OrderDetail, placed bool) {
	labels := map[string]string{
		"pending": "Order placed", "confirmed": "Confirmed",
		"shipped": "Shipped", "delivered": "Delivered",
	}
	idx := 0
	for i, s := range orderSteps {
		if s == d.Status {
			idx = i
		}
	}
	terminated := d.Status == "cancelled" || d.Status == "returned"
	steps := make([]orderStep, len(orderSteps))
	for i, s := range orderSteps {
		steps[i] = orderStep{Key: s, Label: labels[s], Done: !terminated && i <= idx, Current: !terminated && i == idx}
	}

	waNumber, err := h.settings.WhatsAppNumber(r.Context())
	if err != nil {
		slog.Warn("order: whatsapp setting", "err", err)
	}
	waLink := ""
	if waNumber != "" {
		msg := "Hi TechPort! I have a question about my order " + d.Code
		waLink = "https://wa.me/" + strings.TrimPrefix(waNumber, "+") + "?text=" + url.QueryEscape(msg)
	}

	h.renderer.Render(w, "order.html", orderData{
		OrderDetail:  d,
		Steps:        steps,
		Placed:       placed,
		Terminated:   terminated,
		WhatsAppLink: waLink,
	})
}

// OrderPage shows an order to its owner (same session) or to anyone who can
// present the phone number it was placed with.
func (h *Handlers) OrderPage(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(chi.URLParam(r, "code")))
	d, err := h.orders.ByCode(r.Context(), code)
	if errors.Is(err, services.ErrNotFound) {
		h.renderer.Render(w, "track.html", trackData{Code: code, Error: "No order found with that code."})
		return
	}
	if err != nil {
		slog.Error("order: lookup", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	allowed := d.SessionID == sessionID(r) || normalizePhone(r.URL.Query().Get("phone")) == d.Phone
	if !allowed {
		if u, ok := h.currentUser(r); ok && (u.Phone == d.Phone || (d.UserID != nil && *d.UserID == u.ID)) {
			allowed = true
		}
	}
	if !allowed {
		h.renderer.Render(w, "track.html", trackData{
			Code:  code,
			Error: "Enter the phone number used for this order to view it.",
		})
		return
	}
	h.orderView(w, r, d, r.URL.Query().Get("placed") == "1")
}

type trackData struct {
	Code  string
	Phone string
	Error string
}

// Track renders the lookup form; with code+phone params it verifies and
// shows the order.
func (h *Handlers) Track(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("code")))
	phoneRaw := r.URL.Query().Get("phone")
	if code == "" {
		h.renderer.Render(w, "track.html", trackData{})
		return
	}

	d, err := h.orders.ByCode(r.Context(), code)
	if errors.Is(err, services.ErrNotFound) {
		h.renderer.Render(w, "track.html", trackData{Code: code, Phone: phoneRaw, Error: "No order found with that code."})
		return
	}
	if err != nil {
		slog.Error("track: lookup", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if normalizePhone(phoneRaw) != d.Phone && d.SessionID != sessionID(r) {
		h.renderer.Render(w, "track.html", trackData{Code: code, Phone: phoneRaw, Error: "That phone number doesn't match this order."})
		return
	}
	h.orderView(w, r, d, false)
}
