package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/KodeWorks-1/techport/internal/services"
)

// currentUser returns the logged-in customer, or ok=false. In demo mode a
// session with no user is silently attached to the demo account first, so
// every visitor is always logged in.
func (h *Handlers) currentUser(r *http.Request) (services.User, bool) {
	u, err := h.users.BySession(r.Context(), sessionID(r))
	if err == nil {
		return u, true
	}
	if !errors.Is(err, services.ErrNotFound) {
		slog.Error("current user", "err", err)
		return services.User{}, false
	}
	if h.demo && h.demoUserID != 0 {
		if err := h.users.AttachSession(r.Context(), sessionID(r), h.demoUserID); err != nil {
			slog.Error("demo auto-login", "err", err)
			return services.User{}, false
		}
		if u, err := h.users.BySession(r.Context(), sessionID(r)); err == nil {
			return u, true
		}
	}
	return services.User{}, false
}

// requireUser gates account pages behind login.
func (h *Handlers) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := h.currentUser(r); !ok {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AccountMenu renders the header account area (htmx partial).
func (h *Handlers) AccountMenu(w http.ResponseWriter, r *http.Request) {
	u, ok := h.currentUser(r)
	h.renderer.RenderPartial(w, "account-menu", struct {
		LoggedIn bool
		Name     string
	}{ok, firstName(u.Name)})
}

func firstName(full string) string {
	if i := strings.IndexByte(full, ' '); i > 0 {
		return full[:i]
	}
	return full
}

/* ---------- login / register ---------- */

type authData struct {
	Values map[string]string
	Errors map[string]string
	Next   string
}

func safeNext(raw string) string {
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return raw
	}
	return "/"
}

func (h *Handlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentUser(r); ok {
		http.Redirect(w, r, "/account", http.StatusSeeOther)
		return
	}
	h.renderer.Render(w, "login.html", authData{Values: map[string]string{}, Errors: map[string]string{}, Next: safeNext(r.URL.Query().Get("next"))})
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	next := safeNext(r.FormValue("next"))
	values := map[string]string{"phone": strings.TrimSpace(r.FormValue("phone"))}
	errs := map[string]string{}

	phone := normalizePhone(values["phone"])
	if phone == "" {
		errs["phone"] = "Enter a valid Pakistani mobile number"
	}
	password := r.FormValue("password")

	var userID int64
	if len(errs) == 0 {
		var err error
		userID, err = h.users.Login(r.Context(), phone, password)
		if errors.Is(err, services.ErrBadCredentials) {
			errs["form"] = "Wrong phone number or password."
		} else if err != nil {
			slog.Error("login", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	if len(errs) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderer.Render(w, "login.html", authData{Values: values, Errors: errs, Next: next})
		return
	}

	if err := h.users.AttachSession(r.Context(), sessionID(r), userID); err != nil {
		slog.Error("login: attach", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (h *Handlers) RegisterPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentUser(r); ok {
		http.Redirect(w, r, "/account", http.StatusSeeOther)
		return
	}
	h.renderer.Render(w, "register.html", authData{Values: map[string]string{}, Errors: map[string]string{}, Next: safeNext(r.URL.Query().Get("next"))})
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	next := safeNext(r.FormValue("next"))
	values := map[string]string{
		"name":  strings.TrimSpace(r.FormValue("name")),
		"phone": strings.TrimSpace(r.FormValue("phone")),
		"email": strings.TrimSpace(r.FormValue("email")),
	}
	errs := map[string]string{}
	if len(values["name"]) < 3 {
		errs["name"] = "Please enter your full name"
	}
	phone := normalizePhone(values["phone"])
	if phone == "" {
		errs["phone"] = "Enter a valid Pakistani mobile number, e.g. 03XX XXXXXXX"
	}
	if len(r.FormValue("password")) < 6 {
		errs["password"] = "Password must be at least 6 characters"
	}

	if len(errs) == 0 {
		userID, err := h.users.Register(r.Context(), values["name"], phone, r.FormValue("password"), values["email"])
		if errors.Is(err, services.ErrPhoneTaken) {
			errs["phone"] = "This number is already registered — try logging in instead"
		} else if err != nil {
			slog.Error("register", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		} else {
			if err := h.users.AttachSession(r.Context(), sessionID(r), userID); err != nil {
				slog.Error("register: attach", "err", err)
			}
			http.Redirect(w, r, next, http.StatusSeeOther)
			return
		}
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	h.renderer.Render(w, "register.html", authData{Values: values, Errors: errs, Next: next})
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if !h.demo {
		if err := h.users.DetachSession(r.Context(), sessionID(r)); err != nil {
			slog.Warn("logout", "err", err)
		}
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

/* ---------- account pages ---------- */

type accountData struct {
	User   services.User
	Saved  bool
	Errors map[string]string
}

func (h *Handlers) AccountPage(w http.ResponseWriter, r *http.Request) {
	u, _ := h.currentUser(r)
	h.renderer.Render(w, "account.html", accountData{
		User: u, Saved: r.URL.Query().Get("saved") == "1", Errors: map[string]string{},
	})
}

func (h *Handlers) AccountSave(w http.ResponseWriter, r *http.Request) {
	u, _ := h.currentUser(r)
	name := strings.TrimSpace(r.FormValue("name"))
	if len(name) < 3 {
		u.Email = strings.TrimSpace(r.FormValue("email"))
		u.Address = strings.TrimSpace(r.FormValue("address"))
		u.City = strings.TrimSpace(r.FormValue("city"))
		u.Name = name
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderer.Render(w, "account.html", accountData{User: u, Errors: map[string]string{"name": "Please enter your full name"}})
		return
	}
	err := h.users.UpdateProfile(r.Context(), u.ID, name,
		strings.TrimSpace(r.FormValue("email")),
		strings.TrimSpace(r.FormValue("address")),
		strings.TrimSpace(r.FormValue("city")))
	if err != nil {
		slog.Error("account save", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/account?saved=1", http.StatusSeeOther)
}

type myOrdersData struct {
	User services.User
	Live []services.OrderSummary // pending / confirmed / shipped
	Past []services.OrderSummary // delivered / cancelled / returned
}

func (h *Handlers) MyOrders(w http.ResponseWriter, r *http.Request) {
	u, _ := h.currentUser(r)
	orders, err := h.orders.ForUser(r.Context(), u.ID, u.Phone)
	if err != nil {
		slog.Error("my orders", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := myOrdersData{User: u}
	for _, o := range orders {
		switch o.Status {
		case "pending", "confirmed", "shipped":
			data.Live = append(data.Live, o)
		default:
			data.Past = append(data.Past, o)
		}
	}
	h.renderer.Render(w, "account-orders.html", data)
}
