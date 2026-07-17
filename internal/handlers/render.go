package handlers

import (
	"fmt"
	"hash/crc32"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	assets "github.com/KodeWorks-1/techport"
)

var tmplFuncs = template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"deref": func(v *float64) float64 {
		if v == nil {
			return 0
		}
		return *v
	},
	// pct renders the discount percentage for a compare-at price.
	"pct": func(price float64, compare *float64) int {
		if compare == nil || *compare <= price || *compare == 0 {
			return 0
		}
		return int((1 - price / *compare) * 100)
	},
	// assetVer busts browser/CDN caches when the stylesheet changes. Falls
	// back to a checksum of the embedded stylesheet when there is no disk.
	"assetVer": func() int64 {
		if fi, err := os.Stat(filepath.Join("static", "css", "app.css")); err == nil {
			return fi.ModTime().Unix()
		}
		return embeddedCSSVer
	},
	// paragraphs splits description text into lines for <p>-per-line rendering.
	"paragraphs": func(s string) []string { return strings.Split(s, "\n") },
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
	funcs template.FuncMap
	mu    sync.Mutex
	cache map[string]*template.Template
}

func NewRenderer(dev, demo bool, extra template.FuncMap) *Renderer {
	funcs := template.FuncMap{}
	for k, v := range tmplFuncs {
		funcs[k] = v
	}
	// demoMode lets templates hide auth UI (logout/login/signup) in demo mode.
	funcs["demoMode"] = func() bool { return demo }
	for k, v := range extra {
		funcs[k] = v
	}
	return &Renderer{dev: dev, funcs: funcs, cache: map[string]*template.Template{}}
}

var embeddedCSSVer = func() int64 {
	b, err := fs.ReadFile(assets.FS, "static/css/app.css")
	if err != nil {
		return 0
	}
	return int64(crc32.ChecksumIEEE(b))
}()

// layoutFor picks the storefront or admin layout by page path
// ("admin/x.html" lives in views/admin/, everything else in views/pages/).
func layoutFor(page string) (layoutFile, pageFile string) {
	if rest, ok := strings.CutPrefix(page, "admin/"); ok {
		return "admin-layout.html", "views/admin/" + rest
	}
	return "layout.html", "views/pages/" + page
}

func (r *Renderer) load(page string) (*template.Template, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.dev {
		if t, ok := r.cache[page]; ok {
			return t, nil
		}
	}
	layout, pageFile := layoutFor(page)

	var t *template.Template
	var err error
	if r.dev {
		// disk parse for live template reload
		files := []string{"views/" + layout}
		partials, _ := filepath.Glob(filepath.Join("views", "partials", "*.html"))
		files = append(files, partials...)
		files = append(files, pageFile)
		t, err = template.New(layout).Funcs(r.funcs).ParseFiles(files...)
	} else {
		files := []string{"views/" + layout}
		partials, _ := fs.Glob(assets.FS, "views/partials/*.html")
		files = append(files, partials...)
		files = append(files, pageFile)
		t, err = template.New(layout).Funcs(r.funcs).ParseFS(assets.FS, files...)
	}
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", page, err)
	}
	r.cache[page] = t
	return t, nil
}

// RenderPartial renders a named template from views/partials (no layout),
// for HTMX fragment swaps.
func (r *Renderer) RenderPartial(w http.ResponseWriter, name string, data any) {
	t, err := r.load("home.html") // any page works: partials are parsed into every set
	if err != nil {
		slog.Error("partial load failed", "name", name, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("partial render failed", "name", name, "err", err)
	}
}

func (r *Renderer) Render(w http.ResponseWriter, page string, data any) {
	t, err := r.load(page)
	if err != nil {
		slog.Error("template load failed", "page", page, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	layout, _ := layoutFor(page)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, layout, data); err != nil {
		slog.Error("template render failed", "page", page, "err", err)
	}
}
