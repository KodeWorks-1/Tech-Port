package handlers

import (
	"log/slog"
	"net/http"

	"github.com/KodeWorks-1/techport/internal/models"
)

type homeData struct {
	Categories []models.Category
	Featured   []models.ProductCard
	Latest     []models.ProductCard
}

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	categories, err := h.catalog.Categories(ctx)
	if err != nil {
		slog.Error("home: categories", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	featured, err := h.catalog.Featured(ctx, 8)
	if err != nil {
		slog.Error("home: featured", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	latest, err := h.catalog.Latest(ctx, 8)
	if err != nil {
		slog.Error("home: latest", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.renderer.Render(w, "home.html", homeData{
		Categories: categories,
		Featured:   featured,
		Latest:     latest,
	})
}
