package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/KodeWorks-1/techport/internal/models"
	"github.com/KodeWorks-1/techport/internal/services"
)

type homeSection struct {
	Category models.Category
	Products []models.ProductCard
}

type jfyData struct {
	Cards    []models.ProductCard
	NextPage int
	HasMore  bool
}

type homeData struct {
	Categories []services.CategoryCard
	Deals      []models.ProductCard
	Popular    []models.ProductCard
	Fresh      []models.ProductCard
	Sections   []homeSection
	JFY        jfyData
}

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	categories, err := h.catalog.CategoriesWithImage(ctx)
	if err != nil {
		slog.Error("home: categories", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	deals, err := h.catalog.Deals(ctx, 8)
	if err != nil {
		slog.Error("home: deals", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(deals) == 0 {
		if deals, err = h.catalog.Featured(ctx, 8); err != nil {
			slog.Error("home: featured fallback", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	popular, err := h.catalog.Popular(ctx, 4)
	if err != nil {
		slog.Error("home: popular", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	fresh, err := h.catalog.Latest(ctx, 4)
	if err != nil {
		slog.Error("home: latest", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	jfyCards, jfyMore, err := h.catalog.JustForYou(ctx, 1, 12)
	if err != nil {
		slog.Error("home: jfy", "err", err)
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
		Deals:      deals,
		Popular:    popular,
		Fresh:      fresh,
		Sections:   sections,
		JFY:        jfyData{Cards: jfyCards, NextPage: 2, HasMore: jfyMore},
	})
}

// JustForYou serves subsequent feed pages for the htmx load-more button.
func (h *Handlers) JustForYou(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 2 {
		page = 2
	}
	cards, hasMore, err := h.catalog.JustForYou(r.Context(), page, 12)
	if err != nil {
		slog.Error("jfy page", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.renderer.RenderPartial(w, "jfy-tail", jfyData{Cards: cards, NextPage: page + 1, HasMore: hasMore})
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
