package handlers

import (
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/KodeWorks-1/techport/internal/models"
	"github.com/KodeWorks-1/techport/internal/services"
)

type productData struct {
	services.ProductDetail
	Related      []models.ProductCard
	VariantsJSON template.JS
	SchemaJSON   template.JS
	WhatsAppLink string
	HasOptions   bool
	CanonicalURL string
	ImageURL     string
}

// variantJSON is the client-side shape the Alpine variant picker consumes.
type variantJSON struct {
	ID    int64   `json:"id"`
	Label string  `json:"label"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

func (h *Handlers) Product(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	detail, err := h.catalog.ProductBySlug(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, services.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("product: lookup", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	related, err := h.catalog.Related(ctx, detail.CategoryID, detail.ID, 4)
	if err != nil {
		slog.Error("product: related", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	hasOptions := false
	vjs := make([]variantJSON, 0, len(detail.Variants))
	for _, v := range detail.Variants {
		label := "Standard"
		if len(v.Options) > 0 {
			hasOptions = true
			var parts []string
			for _, val := range v.Options {
				parts = append(parts, val)
			}
			label = strings.Join(parts, " / ")
		}
		vjs = append(vjs, variantJSON{ID: v.ID, Label: label, Price: v.Price, Stock: v.Stock})
	}
	raw, err := json.Marshal(vjs)
	if err != nil {
		slog.Error("product: marshal variants", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	waNumber, err := h.settings.WhatsAppNumber(ctx)
	if err != nil {
		slog.Warn("product: whatsapp setting", "err", err)
	}
	waLink := ""
	if waNumber != "" {
		msg := "Hi TechPort! I want to order: " + detail.Title
		waLink = "https://wa.me/" + strings.TrimPrefix(waNumber, "+") + "?text=" + url.QueryEscape(msg)
	}

	base := baseURL(r)
	canonical := base + "/p/" + detail.Slug
	imageURL := ""
	if len(detail.Images) > 0 {
		imageURL = base + detail.Images[0].Path
	}
	inStock := false
	for _, v := range detail.Variants {
		if v.Stock > 0 {
			inStock = true
		}
	}
	availability := "https://schema.org/OutOfStock"
	if inStock {
		availability = "https://schema.org/InStock"
	}
	schema := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Product",
		"name":        detail.Title,
		"description": detail.Description,
		"image":       imageURL,
		"brand":       map[string]any{"@type": "Brand", "name": detail.Brand},
		"offers": map[string]any{
			"@type":         "Offer",
			"url":           canonical,
			"price":         detail.BasePrice,
			"priceCurrency": "PKR",
			"availability":  availability,
		},
	}
	schemaRaw, err := json.Marshal(schema)
	if err != nil {
		slog.Error("product: marshal schema", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.renderer.Render(w, "product.html", productData{
		ProductDetail: detail,
		Related:       related,
		VariantsJSON:  template.JS(raw),
		SchemaJSON:    template.JS(schemaRaw),
		WhatsAppLink:  waLink,
		HasOptions:    hasOptions,
		CanonicalURL:  canonical,
		ImageURL:      imageURL,
	})
}
