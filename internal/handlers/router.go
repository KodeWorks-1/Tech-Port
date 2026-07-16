package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/KodeWorks-1/techport/internal/services"
)

type Handlers struct {
	catalog  *services.Catalog
	renderer *Renderer
}

func New(catalog *services.Catalog, renderer *Renderer) *Handlers {
	return &Handlers{catalog: catalog, renderer: renderer}
}

func (h *Handlers) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	fs := http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
	r.Handle("/static/*", cacheStatic(fs))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	r.Get("/", h.Home)

	return r
}

func cacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		next.ServeHTTP(w, r)
	})
}
