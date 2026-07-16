package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/KodeWorks-1/techport/internal/services"
)

const adminCookie = "tp_admin"

const adminEmailKey ctxKey = 1

// requireAdmin gates /admin routes behind a valid admin session.
func (h *Handlers) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(adminCookie)
		if err != nil {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		email, err := h.adminAuth.AdminByToken(r.Context(), c.Value)
		if err != nil {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), adminEmailKey, email)))
	})
}

func adminEmail(r *http.Request) string {
	e, _ := r.Context().Value(adminEmailKey).(string)
	return e
}

type adminLoginData struct {
	Email string
	Error string
}

func (h *Handlers) AdminLoginPage(w http.ResponseWriter, r *http.Request) {
	h.renderer.Render(w, "admin/login.html", adminLoginData{})
}

func (h *Handlers) AdminLogin(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	token, err := h.adminAuth.Login(r.Context(), email, r.FormValue("password"))
	if errors.Is(err, services.ErrBadCredentials) {
		w.WriteHeader(http.StatusUnauthorized)
		h.renderer.Render(w, "admin/login.html", adminLoginData{Email: email, Error: "Invalid email or password."})
		return
	}
	if err != nil {
		slog.Error("admin login", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *Handlers) AdminLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(adminCookie); err == nil {
		if err := h.adminAuth.Logout(r.Context(), c.Value); err != nil {
			slog.Warn("admin logout", "err", err)
		}
	}
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

/* ---------- dashboard ---------- */

type adminDashboardData struct {
	Email    string
	Stats    services.Stats
	LowStock []services.LowStockRow
	Recent   []services.AdminOrderRow
}

func (h *Handlers) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := h.admin.Stats(ctx)
	if err != nil {
		slog.Error("admin dashboard: stats", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	low, err := h.admin.LowStock(ctx, 5, 10)
	if err != nil {
		slog.Error("admin dashboard: low stock", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	recent, err := h.admin.Orders(ctx, "", "", 10)
	if err != nil {
		slog.Error("admin dashboard: recent", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.renderer.Render(w, "admin/dashboard.html", adminDashboardData{
		Email: adminEmail(r), Stats: stats, LowStock: low, Recent: recent,
	})
}

/* ---------- orders ---------- */

type adminOrdersData struct {
	Orders   []services.AdminOrderRow
	Filter   string
	Search   string
	Statuses []string
}

func (h *Handlers) AdminOrders(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("status")
	search := r.URL.Query().Get("q")
	orders, err := h.admin.Orders(r.Context(), filter, search, 200)
	if err != nil {
		slog.Error("admin orders", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.renderer.Render(w, "admin/orders.html", adminOrdersData{
		Orders: orders, Filter: filter, Search: search, Statuses: services.ValidOrderStatuses,
	})
}

type adminOrderData struct {
	services.OrderDetail
	Statuses []string
}

func (h *Handlers) AdminOrder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d, err := h.orders.ByID(r.Context(), id)
	if errors.Is(err, services.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("admin order", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.renderer.Render(w, "admin/order.html", adminOrderData{OrderDetail: d, Statuses: services.ValidOrderStatuses})
}

func (h *Handlers) AdminOrderStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = h.admin.SetOrderStatus(r.Context(), id,
		r.FormValue("status"), r.FormValue("note"), adminEmail(r))
	if err != nil && !errors.Is(err, services.ErrNotFound) {
		slog.Error("admin order status", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/orders/"+chi.URLParam(r, "id"), http.StatusSeeOther)
}
