package handlers

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var tmplFuncs = template.FuncMap{
	// money renders "Rs. 3,000" from a float rupee amount.
	"money": func(v float64) string {
		s := strconv.FormatFloat(v, 'f', 0, 64)
		var b strings.Builder
		for i, r := range s {
			if i > 0 && (len(s)-i)%3 == 0 && r != '-' {
				b.WriteByte(',')
			}
			b.WriteRune(r)
		}
		return "Rs. " + b.String()
	},
	"moneyPtr": func(v *float64) string {
		if v == nil {
			return ""
		}
		s := strconv.FormatFloat(*v, 'f', 0, 64)
		var b strings.Builder
		for i, r := range s {
			if i > 0 && (len(s)-i)%3 == 0 {
				b.WriteByte(',')
			}
			b.WriteRune(r)
		}
		return "Rs. " + b.String()
	},
}

// Renderer renders a page template inside the shared layout. In dev mode
// templates are re-parsed on every request so edits show up on refresh.
type Renderer struct {
	dev   bool
	mu    sync.Mutex
	cache map[string]*template.Template
}

func NewRenderer(dev bool) *Renderer {
	return &Renderer{dev: dev, cache: map[string]*template.Template{}}
}

func (r *Renderer) load(page string) (*template.Template, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.dev {
		if t, ok := r.cache[page]; ok {
			return t, nil
		}
	}
	files := []string{filepath.Join("views", "layout.html")}
	partials, _ := filepath.Glob(filepath.Join("views", "partials", "*.html"))
	files = append(files, partials...)
	files = append(files, filepath.Join("views", "pages", page))

	t, err := template.New("layout.html").Funcs(tmplFuncs).ParseFiles(files...)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", page, err)
	}
	r.cache[page] = t
	return t, nil
}

func (r *Renderer) Render(w http.ResponseWriter, page string, data any) {
	t, err := r.load(page)
	if err != nil {
		slog.Error("template load failed", "page", page, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("template render failed", "page", page, "err", err)
	}
}
