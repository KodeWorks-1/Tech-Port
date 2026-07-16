package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/KodeWorks-1/techport/internal/models"
	"github.com/KodeWorks-1/techport/internal/services"
)

const productsPerPage = 24

type categoryData struct {
	Category   models.Category
	Categories []models.Category
	Products   []models.ProductCard
	Total      int
	Sort       string
	Page       int
	TotalPages int
}

func (d categoryData) PrevPage() int { return d.Page - 1 }
func (d categoryData) NextPage() int { return d.Page + 1 }

func (h *Handlers) Category(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cat, err := h.catalog.CategoryBySlug(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, services.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("category: lookup", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sort := r.URL.Query().Get("sort")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	products, total, err := h.catalog.ProductsByCategory(ctx, cat.ID, sort, page, productsPerPage)
	if err != nil {
		slog.Error("category: products", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	categories, err := h.catalog.Categories(ctx)
	if err != nil {
		slog.Error("category: categories", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	totalPages := (total + productsPerPage - 1) / productsPerPage
	h.renderer.Render(w, "category.html", categoryData{
		Category:   cat,
		Categories: categories,
		Products:   products,
		Total:      total,
		Sort:       sort,
		Page:       page,
		TotalPages: totalPages,
	})
}
