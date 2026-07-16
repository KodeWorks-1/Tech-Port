package handlers

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	assets "github.com/KodeWorks-1/techport"
	"github.com/KodeWorks-1/techport/internal/services"
)

type Handlers struct {
	catalog   *services.Catalog
	cart      *services.Cart
	orders    *services.Orders
	users     *services.Users
	settings  *services.Settings
	admin     *services.Admin
	adminAuth *services.AdminAuth
	renderer  *Renderer
}

func New(catalog *services.Catalog, cart *services.Cart, orders *services.Orders,
	users *services.Users, settings *services.Settings, admin *services.Admin,
	adminAuth *services.AdminAuth, renderer *Renderer) *Handlers {
	return &Handlers{catalog: catalog, cart: cart, orders: orders, users: users,
		settings: settings, admin: admin, adminAuth: adminAuth, renderer: renderer}
}

func (h *Handlers) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	var staticRoot http.FileSystem = http.Dir("static")
	if !h.renderer.dev {
		if sub, err := fs.Sub(assets.FS, "static"); err == nil {
			staticRoot = http.FS(sub)
		}
	}
	staticSrv := http.StripPrefix("/static/", http.FileServer(staticRoot))
	r.Handle("/static/*", h.cacheStatic(staticSrv))
	uploads := http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads")))
	r.Handle("/uploads/*", h.cacheStatic(uploads))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Group(func(r chi.Router) {
		r.Use(h.session)

		r.Get("/", h.Home)
		r.Get("/jfy", h.JustForYou)
		r.Get("/search", h.Search)
		r.Get("/c/{slug}", h.Category)
		r.Get("/p/{slug}", h.Product)

		r.Get("/login", h.LoginPage)
		r.Post("/login", h.Login)
		r.Get("/register", h.RegisterPage)
		r.Post("/register", h.Register)
		r.Post("/logout", h.Logout)
		r.Get("/account/menu", h.AccountMenu)
		r.Group(func(r chi.Router) {
			r.Use(h.requireUser)
			r.Get("/account", h.AccountPage)
			r.Post("/account", h.AccountSave)
			r.Get("/account/orders", h.MyOrders)
		})

		r.Get("/cart", h.CartPage)
		r.Get("/cart/count", h.CartCount)
		r.Post("/cart/items", h.CartAdd)
		r.Post("/cart/items/{id}", h.CartSetQty)

		r.Get("/checkout", h.Checkout)
		r.Post("/checkout", h.PlaceOrder)
		r.Get("/order/{code}", h.OrderPage)
		r.Get("/track", h.Track)

		r.Get("/about", h.StaticPage("about.html"))
		r.Get("/warranty-returns", h.StaticPage("warranty-returns.html"))
	})

	r.Get("/robots.txt", h.Robots)
	r.Get("/sitemap.xml", h.Sitemap)

	r.Route("/admin", func(r chi.Router) {
		r.Get("/login", h.AdminLoginPage)
		r.Post("/login", h.AdminLogin)

		r.Group(func(r chi.Router) {
			r.Use(h.requireAdmin)

			r.Get("/", h.AdminDashboard)
			r.Post("/logout", h.AdminLogout)

			r.Get("/orders", h.AdminOrders)
			r.Get("/orders/{id}", h.AdminOrder)
			r.Post("/orders/{id}/status", h.AdminOrderStatus)

			r.Get("/products", h.AdminProducts)
			r.Post("/products/{id}/toggle", h.AdminProductToggle)
			r.Get("/products/new", h.AdminProductNew)
			r.Post("/products", h.AdminProductCreate)
			r.Get("/products/{id}", h.AdminProductEdit)
			r.Post("/products/{id}", h.AdminProductUpdate)
			r.Post("/products/{id}/variants", h.AdminVariantAdd)
			r.Post("/variants/{id}", h.AdminVariantUpdate)
			r.Post("/products/{id}/images", h.AdminImageUpload)
			r.Post("/images/{id}/delete", h.AdminImageDelete)

			r.Get("/categories", h.AdminCategories)
			r.Post("/categories", h.AdminCategoryCreate)

			r.Get("/settings", h.AdminSettingsPage)
			r.Post("/settings", h.AdminSettingsSave)
		})
	})

	return r
}

func (h *Handlers) cacheStatic(next http.Handler) http.Handler {
	cache := "public, max-age=86400"
	if h.renderer.dev {
		cache = "no-cache"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", cache)
		next.ServeHTTP(w, r)
	})
}
