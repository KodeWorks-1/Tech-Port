package handlers

import (
	"log/slog"
	"net/http"

	"github.com/KodeWorks-1/techport/internal/models"
	"github.com/KodeWorks-1/techport/internal/services"
)

type homeData struct {
	Categories []services.CategoryCard
	Popular    []models.ProductCard
	Fresh      []models.ProductCard
}

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	categories, err := h.catalog.CategoriesWithImage(ctx)
	if err != nil {
		slog.Error("home: categories", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	popular, err := h.catalog.Popular(ctx, 8)
	if err != nil {
		slog.Error("home: popular", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	fresh, err := h.catalog.Latest(ctx, 8)
	if err != nil {
		slog.Error("home: latest", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.renderer.Render(w, "home.html", homeData{
		Categories: categories,
		Popular:    popular,
		Fresh:      fresh,
	})
}

type searchData struct {
	Query    string
	Products []models.ProductCard
}

func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	var products []models.ProductCard
	if len(q) >= 2 {
		var err error
		products, err = h.catalog.Search(r.Context(), q, 48)
		if err != nil {
			slog.Error("search", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	h.renderer.Render(w, "search.html", searchData{Query: q, Products: products})
}
