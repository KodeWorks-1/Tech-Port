package db

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// catalog.json is TechPort's real Daraz catalog (scraped 2026-07-16);
// images live in static/img/products/.
//
//go:embed catalog.json
var catalogJSON []byte

var seedCategories = []struct{ Slug, Name string }{
	{"keyboards", "Keyboards"},
	{"mice", "Mice"},
	{"cooling-pads", "Cooling Pads & Stands"},
	{"backpacks", "Laptop Backpacks"},
	{"accessories", "Accessories"},
}

type catalogProduct struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Brand       string   `json:"brand"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`
	CompareAt   *float64 `json:"compare_at"`
	Featured    bool     `json:"featured"`
	Stock       int      `json:"stock"`
	Images      []string `json:"images"`
}

// seedAdmin creates the default admin login on first run. The password MUST
// be changed before going live.
func seedAdmin(ctx context.Context, pool *pgxpool.Pool) error {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM admin_users`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("changeme123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO admin_users (email, password_hash) VALUES ($1, $2)`,
		"admin@techport.pk", string(hash)); err != nil {
		return err
	}
	slog.Warn("seeded default admin", "email", "admin@techport.pk", "password", "changeme123")
	return nil
}

// SeedIfEmpty loads the embedded TechPort catalog on a fresh database and
// ensures a default admin account exists.
func SeedIfEmpty(ctx context.Context, pool *pgxpool.Pool) error {
	if err := seedAdmin(ctx, pool); err != nil {
		return err
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM products`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	var products []catalogProduct
	if err := json.Unmarshal(catalogJSON, &products); err != nil {
		return fmt.Errorf("parse embedded catalog: %w", err)
	}

	catIDs := map[string]int64{}
	for i, c := range seedCategories {
		var id int64
		if err := pool.QueryRow(ctx, `
			INSERT INTO categories (slug, name, sort) VALUES ($1,$2,$3)
			ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
			RETURNING id`, c.Slug, c.Name, i,
		).Scan(&id); err != nil {
			return fmt.Errorf("seed category %s: %w", c.Slug, err)
		}
		catIDs[c.Slug] = id
	}

	for _, p := range products {
		catID, ok := catIDs[p.Category]
		if !ok {
			return fmt.Errorf("product %s: unknown category %q", p.Slug, p.Category)
		}
		var productID int64
		if err := pool.QueryRow(ctx, `
			INSERT INTO products (slug, title, brand, category_id, description, specs, base_price, compare_at_price, featured)
			VALUES ($1,$2,$3,$4,$5,'{}',$6,$7,$8)
			ON CONFLICT (slug) DO NOTHING
			RETURNING id`,
			p.Slug, p.Title, p.Brand, catID, p.Description, p.Price, p.CompareAt, p.Featured,
		).Scan(&productID); err != nil {
			return fmt.Errorf("seed product %s: %w", p.Slug, err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO product_variants (product_id, sku, options, price, stock, is_default)
			VALUES ($1, $2, '{}', $3, $4, TRUE)`,
			productID, "TP-"+p.Slug[:min(16, len(p.Slug))], p.Price, p.Stock); err != nil {
			return fmt.Errorf("seed variant for %s: %w", p.Slug, err)
		}
		for i, img := range p.Images {
			if _, err := pool.Exec(ctx, `
				INSERT INTO product_images (product_id, path, alt, sort) VALUES ($1,$2,$3,$4)`,
				productID, img, p.Title, i); err != nil {
				return fmt.Errorf("seed image for %s: %w", p.Slug, err)
			}
		}
	}
	slog.Info("seeded catalog", "products", len(products))

	settings := map[string]string{
		"shipping_fee":    `{"flat": 250, "free_above": 5000}`,
		"whatsapp_number": `"+92XXXXXXXXXX"`,
		"payment_methods": `[
			{"key":"cod","label":"Cash on Delivery","enabled":true},
			{"key":"card","label":"Credit / Debit Card","enabled":false},
			{"key":"easypaisa","label":"Easypaisa","enabled":false},
			{"key":"jazzcash","label":"JazzCash","enabled":false},
			{"key":"bank","label":"Bank Transfer","enabled":false}
		]`,
	}
	for k, v := range settings {
		if _, err := pool.Exec(ctx,
			`INSERT INTO settings (key, value) VALUES ($1,$2) ON CONFLICT (key) DO NOTHING`, k, v,
		); err != nil {
			return fmt.Errorf("seed setting %s: %w", k, err)
		}
	}
	return nil
}
