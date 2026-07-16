package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/KodeWorks-1/techport/internal/services"
)

type cartData struct {
	Cart     services.CartView
	Shipping services.ShippingConfig
}

func (d cartData) ShippingFee() float64 {
	if d.Cart.Subtotal >= d.Shipping.FreeAbove {
		return 0
	}
	return d.Shipping.Flat
}

func (d cartData) Total() float64 { return d.Cart.Subtotal + d.ShippingFee() }

// AmountToFreeShipping is > 0 when the cart hasn't reached the free threshold.
func (d cartData) AmountToFreeShipping() float64 { return d.Shipping.FreeAbove - d.Cart.Subtotal }

func (h *Handlers) cartData(r *http.Request) (cartData, error) {
	view, err := h.cart.View(r.Context(), sessionID(r))
	if err != nil {
		return cartData{}, err
	}
	shipping, err := h.settings.Shipping(r.Context())
	if err != nil {
		return cartData{}, err
	}
	return cartData{Cart: view, Shipping: shipping}, nil
}

func (h *Handlers) CartPage(w http.ResponseWriter, r *http.Request) {
	data, err := h.cartData(r)
	if err != nil {
		slog.Error("cart: view", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.renderer.Render(w, "cart.html", data)
}

// CartAdd handles add-to-cart from product pages. Responds with no content;
// HX-Trigger events update the badge and pop a toast.
func (h *Handlers) CartAdd(w http.ResponseWriter, r *http.Request) {
	variantID, err := strconv.ParseInt(r.FormValue("variant_id"), 10, 64)
	if err != nil {
		http.Error(w, "bad variant", http.StatusBadRequest)
		return
	}
	qty, _ := strconv.Atoi(r.FormValue("qty"))

	err = h.cart.Add(r.Context(), sessionID(r), variantID, qty)
	if errors.Is(err, services.ErrOutOfStock) {
		w.Header().Set("HX-Trigger", `{"toast":"Sorry, this item is out of stock"}`)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		slog.Error("cart: add", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", `{"cart-updated":true,"toast":"Added to cart"}`)
	w.WriteHeader(http.StatusNoContent)
}

// CartSetQty updates a line's quantity (0 removes) and re-renders the cart box.
func (h *Handlers) CartSetQty(w http.ResponseWriter, r *http.Request) {
	itemID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad item", http.StatusBadRequest)
		return
	}
	qty, err := strconv.Atoi(r.FormValue("qty"))
	if err != nil {
		http.Error(w, "bad qty", http.StatusBadRequest)
		return
	}
	if err := h.cart.SetQty(r.Context(), sessionID(r), itemID, qty); err != nil {
		slog.Error("cart: set qty", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data, err := h.cartData(r)
	if err != nil {
		slog.Error("cart: reload", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", `{"cart-updated":true}`)
	h.renderer.RenderPartial(w, "cart-box", data)
}

func (h *Handlers) CartCount(w http.ResponseWriter, r *http.Request) {
	n, err := h.cart.Count(r.Context(), sessionID(r))
	if err != nil {
		slog.Error("cart: count", "err", err)
		n = 0
	}
	fmt.Fprint(w, n)
}
