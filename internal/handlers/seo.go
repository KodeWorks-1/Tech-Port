package handlers

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// baseURL reconstructs the external origin (Cloudflare/proxy aware).
func baseURL(r *http.Request) string {
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS == nil {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}

func (h *Handlers) Robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nDisallow: /admin\nDisallow: /cart\nDisallow: /checkout\nDisallow: /order/\nSitemap: %s/sitemap.xml\n", baseURL(r))
}

type sitemapURL struct {
	Loc string `xml:"loc"`
}

type urlset struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

func (h *Handlers) Sitemap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	base := baseURL(r)

	set := urlset{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	set.URLs = append(set.URLs, sitemapURL{base + "/"})

	cats, err := h.catalog.Categories(ctx)
	if err != nil {
		slog.Error("sitemap: categories", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	for _, c := range cats {
		set.URLs = append(set.URLs, sitemapURL{base + "/c/" + c.Slug})
	}
	products, err := h.catalog.Latest(ctx, 5000)
	if err != nil {
		slog.Error("sitemap: products", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	for _, p := range products {
		set.URLs = append(set.URLs, sitemapURL{base + "/p/" + p.Slug})
	}
	for _, page := range []string{"/about", "/warranty-returns", "/track"} {
		set.URLs = append(set.URLs, sitemapURL{base + page})
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	if err := xml.NewEncoder(w).Encode(set); err != nil {
		slog.Error("sitemap: encode", "err", err)
	}
}

// StaticPage renders a fixed content page (about, warranty).
func (h *Handlers) StaticPage(page string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		waNumber, err := h.settings.WhatsAppNumber(r.Context())
		if err != nil {
			slog.Warn("static page: whatsapp", "err", err)
		}
		waLink := ""
		if waNumber != "" {
			waLink = "https://wa.me/" + strings.TrimPrefix(waNumber, "+")
		}
		h.renderer.Render(w, page, struct{ WhatsAppLink string }{waLink})
	}
}
