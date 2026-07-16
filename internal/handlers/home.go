package handlers

import (
	"log/slog"
	"net/http"

	"github.com/KodeWorks-1/techport/internal/models"
	"github.com/KodeWorks-1/techport/internal/services"
)

type homeSection struct {
	Category models.Category
	Products []models.ProductCard
}

type homeData struct {
	Categories []services.CategoryCard
	Featured   []models.ProductCard
	Sections   []homeSection
}

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	categories, err := h.catalog.CategoriesWithImage(ctx)
	if err != nil {
		slog.Error("home: categories", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	featured, err := h.catalog.Featured(ctx, 4)
	if err != nil {
		slog.Error("home: featured", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// "Top Selling" rows for the first few categories that have products.
	var sections []homeSection
	for _, c := range categories {
		if len(sections) == 3 {
			break
		}
		products, _, err := h.catalog.ProductsByCategory(ctx, c.ID, "", 1, 4)
		if err != nil {
			slog.Error("home: section", "category", c.Slug, "err", err)
			continue
		}
		if len(products) == 0 {
			continue
		}
		sections = append(sections, homeSection{Category: c.Category, Products: products})
	}

	h.renderer.Render(w, "home.html", homeData{
		Categories: categories,
		Featured:   featured,
		Sections:   sections,
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
