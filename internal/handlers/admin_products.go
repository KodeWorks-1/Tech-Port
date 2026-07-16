package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/KodeWorks-1/techport/internal/models"
	"github.com/KodeWorks-1/techport/internal/services"
)

/* ---------- product list + form ---------- */

func (h *Handlers) AdminProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.admin.Products(r.Context())
	if err != nil {
		slog.Error("admin products", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.renderer.Render(w, "admin/products.html", struct{ Products []services.AdminProductRow }{products})
}

type adminProductFormData struct {
	IsNew      bool
	Product    services.ProductDetail
	Categories []models.Category
	SpecsText  string
	Error      string
}

func (h *Handlers) adminProductForm(w http.ResponseWriter, r *http.Request, data adminProductFormData) {
	cats, err := h.catalog.Categories(r.Context())
	if err != nil {
		slog.Error("admin product form: categories", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data.Categories = cats
	h.renderer.Render(w, "admin/product-form.html", data)
}

func (h *Handlers) AdminProductNew(w http.ResponseWriter, r *http.Request) {
	h.adminProductForm(w, r, adminProductFormData{IsNew: true,
		Product: services.ProductDetail{Product: models.Product{Active: true}}})
}

func (h *Handlers) AdminProductEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	p, err := h.admin.ProductByID(r.Context(), id)
	if errors.Is(err, services.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("admin product edit", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var specLines []string
	for k, v := range p.Specs {
		specLines = append(specLines, k+": "+v)
	}
	h.adminProductForm(w, r, adminProductFormData{Product: p, SpecsText: strings.Join(specLines, "\n")})
}

// parseProductInput reads the shared product form fields.
func parseProductInput(r *http.Request) (services.ProductInput, error) {
	price, err := strconv.ParseFloat(r.FormValue("base_price"), 64)
	if err != nil || price < 0 {
		return services.ProductInput{}, errors.New("enter a valid price")
	}
	var compareAt *float64
	if s := strings.TrimSpace(r.FormValue("compare_at_price")); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || v < 0 {
			return services.ProductInput{}, errors.New("enter a valid compare-at price")
		}
		compareAt = &v
	}
	catID, err := strconv.ParseInt(r.FormValue("category_id"), 10, 64)
	if err != nil {
		return services.ProductInput{}, errors.New("choose a category")
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if len(title) < 3 {
		return services.ProductInput{}, errors.New("enter a product title")
	}

	specs := map[string]string{}
	for _, line := range strings.Split(r.FormValue("specs"), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k != "" && v != "" {
			specs[k] = v
		}
	}

	return services.ProductInput{
		Title:          title,
		Brand:          strings.TrimSpace(r.FormValue("brand")),
		CategoryID:     catID,
		Description:    strings.TrimSpace(r.FormValue("description")),
		Specs:          specs,
		BasePrice:      price,
		CompareAtPrice: compareAt,
		Active:         r.FormValue("active") == "on",
		Featured:       r.FormValue("featured") == "on",
	}, nil
}

func (h *Handlers) AdminProductCreate(w http.ResponseWriter, r *http.Request) {
	in, err := parseProductInput(r)
	if err != nil {
		h.adminProductForm(w, r, adminProductFormData{IsNew: true, Error: err.Error(),
			Product: services.ProductDetail{Product: models.Product{Active: true}}})
		return
	}
	stock, _ := strconv.Atoi(r.FormValue("stock"))
	id, err := h.admin.CreateProduct(r.Context(), in, stock)
	if err != nil {
		slog.Error("admin product create", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/products/%d", id), http.StatusSeeOther)
}

func (h *Handlers) AdminProductUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	in, perr := parseProductInput(r)
	if perr != nil {
		p, err := h.admin.ProductByID(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		h.adminProductForm(w, r, adminProductFormData{Product: p, SpecsText: r.FormValue("specs"), Error: perr.Error()})
		return
	}
	if err := h.admin.UpdateProduct(r.Context(), id, in); err != nil {
		slog.Error("admin product update", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/products/%d", id), http.StatusSeeOther)
}

/* ---------- variants ---------- */

// parseOptions turns "Color=Black; Size=XL" into a map.
func parseOptions(s string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(s, ";") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

func (h *Handlers) AdminVariantAdd(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	stock, _ := strconv.Atoi(r.FormValue("stock"))
	if err := h.admin.AddVariant(r.Context(), productID, parseOptions(r.FormValue("options")), price, stock); err != nil {
		slog.Error("admin variant add", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/products/%d", productID), http.StatusSeeOther)
}

func (h *Handlers) AdminVariantUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	back := "/admin/products/" + r.FormValue("product_id")

	if r.FormValue("delete") == "1" {
		if err := h.admin.DeleteVariant(r.Context(), id); err != nil {
			slog.Error("admin variant delete", "err", err)
		}
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	stock, _ := strconv.Atoi(r.FormValue("stock"))
	if err := h.admin.UpdateVariant(r.Context(), id, parseOptions(r.FormValue("options")), price, stock); err != nil {
		slog.Error("admin variant update", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

/* ---------- images ---------- */

var allowedImageExt = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".svg": true}

func (h *Handlers) AdminImageUpload(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	back := fmt.Sprintf("/admin/products/%d", productID)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "file too large (max 10MB)", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedImageExt[ext] {
		http.Error(w, "only jpg, png, webp or svg images", http.StatusBadRequest)
		return
	}

	buf := make([]byte, 8)
	rand.Read(buf)
	name := hex.EncodeToString(buf) + ext
	dir := filepath.Join("uploads", "products", strconv.FormatInt(productID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Error("admin image: mkdir", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	dst, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		slog.Error("admin image: create", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		slog.Error("admin image: write", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	webPath := "/uploads/products/" + strconv.FormatInt(productID, 10) + "/" + name
	if err := h.admin.AddImage(r.Context(), productID, webPath, r.FormValue("alt")); err != nil {
		slog.Error("admin image: db", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (h *Handlers) AdminImageDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	path, err := h.admin.DeleteImage(r.Context(), id)
	if err != nil && !errors.Is(err, services.ErrNotFound) {
		slog.Error("admin image delete", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Only unlink files we manage under uploads/ (seed images live in static/).
	if rel, ok := strings.CutPrefix(path, "/uploads/"); ok && !strings.Contains(rel, "..") {
		if err := os.Remove(filepath.Join("uploads", filepath.FromSlash(rel))); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("admin image delete: unlink", "err", err)
		}
	}
	http.Redirect(w, r, r.FormValue("back"), http.StatusSeeOther)
}

/* ---------- categories ---------- */

func (h *Handlers) AdminCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.admin.Categories(r.Context())
	if err != nil {
		slog.Error("admin categories", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.renderer.Render(w, "admin/categories.html", struct{ Categories []services.AdminCategoryRow }{cats})
}

func (h *Handlers) AdminCategoryCreate(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	if name != "" {
		if err := h.admin.CreateCategory(r.Context(), name); err != nil {
			slog.Error("admin category create", "err", err)
		}
	}
	http.Redirect(w, r, "/admin/categories", http.StatusSeeOther)
}

/* ---------- settings ---------- */

type adminSettingsData struct {
	Shipping services.ShippingConfig
	WhatsApp string
	Methods  []services.PaymentMethod
	Saved    bool
}

func (h *Handlers) AdminSettingsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shipping, err := h.settings.Shipping(ctx)
	if err != nil {
		slog.Error("admin settings: shipping", "err", err)
	}
	wa, err := h.settings.WhatsAppNumber(ctx)
	if err != nil {
		slog.Error("admin settings: whatsapp", "err", err)
	}
	methods, err := h.settings.PaymentMethods(ctx)
	if err != nil {
		slog.Error("admin settings: methods", "err", err)
	}
	h.renderer.Render(w, "admin/settings.html", adminSettingsData{
		Shipping: shipping, WhatsApp: wa, Methods: methods,
		Saved: r.URL.Query().Get("saved") == "1",
	})
}

func (h *Handlers) AdminSettingsSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flat, _ := strconv.ParseFloat(r.FormValue("shipping_flat"), 64)
	freeAbove, _ := strconv.ParseFloat(r.FormValue("free_above"), 64)
	if err := h.admin.SaveSetting(ctx, "shipping_fee",
		services.ShippingConfig{Flat: flat, FreeAbove: freeAbove}); err != nil {
		slog.Error("admin settings: save shipping", "err", err)
	}
	if err := h.admin.SaveSetting(ctx, "whatsapp_number",
		strings.TrimSpace(r.FormValue("whatsapp"))); err != nil {
		slog.Error("admin settings: save whatsapp", "err", err)
	}

	methods, err := h.settings.PaymentMethods(ctx)
	if err == nil {
		for i := range methods {
			methods[i].Enabled = r.FormValue("method_"+methods[i].Key) == "on"
		}
		if err := h.admin.SaveSetting(ctx, "payment_methods", methods); err != nil {
			slog.Error("admin settings: save methods", "err", err)
		}
	}
	http.Redirect(w, r, "/admin/settings?saved=1", http.StatusSeeOther)
}
