package handlers

import (
	"context"
	"html/template"
	"log/slog"
	"sync"
	"time"

	"github.com/KodeWorks-1/techport/internal/models"
	"github.com/KodeWorks-1/techport/internal/services"
)

// NavFuncs exposes site-wide header/footer data (categories, phone number)
// to every template via cached template funcs, so page handlers don't have
// to thread it through their data structs. The returned invalidate func
// drops the cache so admin edits (settings, categories) show up instantly.
func NavFuncs(catalog *services.Catalog, settings *services.Settings) (template.FuncMap, func()) {
	var mu sync.Mutex
	var cats []models.Category
	var phone string
	var fetched time.Time

	refresh := func() ([]models.Category, string) {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(fetched) < time.Minute && cats != nil {
			return cats, phone
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if c, err := catalog.Categories(ctx); err == nil {
			cats = c
		} else {
			slog.Warn("nav: categories", "err", err)
		}
		if p, err := settings.WhatsAppNumber(ctx); err == nil {
			phone = p
		} else {
			slog.Warn("nav: phone", "err", err)
		}
		fetched = time.Now()
		return cats, phone
	}

	invalidate := func() {
		mu.Lock()
		fetched = time.Time{}
		mu.Unlock()
	}
	return template.FuncMap{
		"navCategories": func() []models.Category { c, _ := refresh(); return c },
		"storePhone":    func() string { _, p := refresh(); return p },
	}, invalidate
}
