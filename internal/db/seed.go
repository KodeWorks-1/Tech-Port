package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

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

type seedProduct struct {
	Slug      string
	Title     string
	Brand     string
	Category  string
	Desc      string
	Specs     map[string]string
	Price     float64
	CompareAt float64 // 0 = none
	Featured  bool
	Stock     int
	Colors    []string // extra color variants beyond default
}

// SeedIfEmpty inserts demo catalog data on a fresh database so the store is
// browsable before the customer's real catalog arrives, and ensures a
// default admin account exists.
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

	categories := []struct{ Slug, Name string }{
		{"keyboards", "Keyboards"},
		{"mice", "Mice"},
		{"audio", "Audio"},
		{"chargers-cables", "Chargers & Cables"},
		{"storage", "Storage"},
		{"accessories", "Accessories"},
	}
	catIDs := map[string]int64{}
	for i, c := range categories {
		var id int64
		if err := pool.QueryRow(ctx,
			`INSERT INTO categories (slug, name, sort) VALUES ($1,$2,$3) RETURNING id`,
			c.Slug, c.Name, i,
		).Scan(&id); err != nil {
			return fmt.Errorf("seed category %s: %w", c.Slug, err)
		}
		catIDs[c.Slug] = id
	}

	products := []seedProduct{
		{
			Slug: "coolbell-smart-board-bt-kb02", Title: "CoolBell Smart Board BT-KB02 — Bluetooth Keyboard with Touchpad",
			Brand: "CoolBell", Category: "keyboards", Featured: true, Price: 3000, CompareAt: 3499, Stock: 25,
			Desc: "Wireless Bluetooth keyboard with a built-in touchpad — control your tablet, laptop or smart TV from the couch. Slim, rechargeable and works with Android, iOS and Windows.",
			Specs: map[string]string{"Connectivity": "Bluetooth 5.0", "Battery": "Rechargeable, USB-C", "Compatibility": "Android / iOS / Windows", "Layout": "QWERTY with multi-touch trackpad", "Color": "Black"},
		},
		{
			Slug: "rgb-mechanical-keyboard-k550", Title: "K550 RGB Mechanical Gaming Keyboard — Blue Switches",
			Brand: "TechPort", Category: "keyboards", Price: 5499, Stock: 12,
			Desc: "Full-size mechanical keyboard with clicky blue switches, per-key RGB and a detachable USB-C cable.",
			Specs: map[string]string{"Switches": "Blue (clicky)", "Backlight": "Per-key RGB", "Connection": "Wired USB-C (detachable)", "Rollover": "N-key"},
		},
		{
			Slug: "wireless-silent-mouse-m330", Title: "M330 Wireless Silent Mouse",
			Brand: "TechPort", Category: "mice", Price: 1299, Stock: 40, Colors: []string{"Black", "White"},
			Desc: "Quiet-click wireless mouse with 18-month battery life and a comfortable contoured grip.",
			Specs: map[string]string{"Connection": "2.4GHz USB receiver", "DPI": "1000 / 1600", "Battery": "1x AA (18 months)", "Clicks": "90% quieter"},
		},
		{
			Slug: "gaming-mouse-rgb-7200dpi", Title: "Viper X7 RGB Gaming Mouse — 7200 DPI",
			Brand: "TechPort", Category: "mice", Featured: true, Price: 1899, CompareAt: 2299, Stock: 18,
			Desc: "Lightweight gaming mouse with adjustable DPI up to 7200, 7 programmable buttons and breathing RGB.",
			Specs: map[string]string{"DPI": "800–7200 (7 steps)", "Buttons": "7 programmable", "Polling": "1000Hz", "Cable": "Braided 1.8m"},
		},
		{
			Slug: "tws-earbuds-pro-anc", Title: "AirPro TWS Earbuds with Active Noise Cancellation",
			Brand: "TechPort", Category: "audio", Featured: true, Price: 4999, CompareAt: 5999, Stock: 30,
			Desc: "True-wireless earbuds with hybrid ANC, transparency mode and 30-hour total battery with the charging case.",
			Specs: map[string]string{"ANC": "Hybrid, -35dB", "Battery": "6h + 24h case", "Bluetooth": "5.3", "Charging": "USB-C, wireless"},
		},
		{
			Slug: "bluetooth-speaker-mini-boom", Title: "MiniBoom Portable Bluetooth Speaker",
			Brand: "TechPort", Category: "audio", Price: 2499, Stock: 22,
			Desc: "Pocket-size speaker with surprisingly big sound, IPX5 splash resistance and 12-hour playtime.",
			Specs: map[string]string{"Output": "8W", "Battery": "12 hours", "Waterproof": "IPX5", "Pairing": "TWS stereo pair"},
		},
		{
			Slug: "gan-charger-65w-dual", Title: "65W GaN Fast Charger — USB-C + USB-A",
			Brand: "TechPort", Category: "chargers-cables", Featured: true, Price: 3299, Stock: 35,
			Desc: "Charge a laptop and phone together from one compact GaN brick. PD 3.0 and QC 4.0 supported.",
			Specs: map[string]string{"Output": "65W total (PD 3.0 / QC 4.0)", "Ports": "1x USB-C, 1x USB-A", "Input": "100–240V", "Size": "50% smaller than stock chargers"},
		},
		{
			Slug: "braided-type-c-cable-100w-2m", Title: "Braided USB-C Cable 100W — 2 Metre",
			Brand: "TechPort", Category: "chargers-cables", Price: 599, Stock: 100,
			Desc: "Nylon-braided 100W PD cable with e-marker chip — fast-charges laptops, tablets and phones.",
			Specs: map[string]string{"Power": "100W PD (e-marker)", "Length": "2m", "Data": "480Mbps", "Jacket": "Braided nylon"},
		},
		{
			Slug: "usb3-flash-drive-64gb", Title: "USB 3.0 Flash Drive 64GB — Metal Body",
			Brand: "TechPort", Category: "storage", Price: 1499, Stock: 60,
			Desc: "Rugged all-metal 64GB flash drive with USB 3.0 speeds and a keyring loop.",
			Specs: map[string]string{"Capacity": "64GB", "Interface": "USB 3.0", "Read": "up to 100MB/s", "Body": "Zinc alloy"},
		},
		{
			Slug: "laptop-stand-aluminium", Title: "Aluminium Laptop Stand — Adjustable Height",
			Brand: "TechPort", Category: "accessories", Featured: true, Price: 2799, CompareAt: 3200, Stock: 15,
			Desc: "CNC aluminium stand that lifts your laptop to eye level, with silicone pads and folding legs for travel.",
			Specs: map[string]string{"Material": "Aluminium alloy", "Fits": "10–17.3 inch laptops", "Adjustment": "6 height levels", "Weight": "480g, foldable"},
		},
		{
			Slug: "phone-holder-flexible-arm", Title: "Flexible Arm Phone Holder — Bed & Desk Mount",
			Brand: "TechPort", Category: "accessories", Price: 899, Stock: 45,
			Desc: "90cm gooseneck phone mount that clamps to a bed frame or desk. 360° rotating cradle fits all phones.",
			Specs: map[string]string{"Arm": "90cm gooseneck", "Clamp": "up to 8cm surfaces", "Rotation": "360°", "Fits": "4–7 inch phones"},
		},
		{
			Slug: "webcam-full-hd-1080p", Title: "StreamCam Full HD 1080p Webcam with Mic",
			Brand: "TechPort", Category: "accessories", Price: 3999, Stock: 10,
			Desc: "Plug-and-play 1080p/30fps webcam with a built-in noise-reducing mic and a privacy cover.",
			Specs: map[string]string{"Resolution": "1080p @ 30fps", "Mic": "Built-in, noise reducing", "Mount": "Clip + tripod thread", "Connection": "USB-A, no drivers"},
		},
	}

	for _, p := range products {
		specs, _ := json.Marshal(p.Specs)
		var compareAt *float64
		if p.CompareAt > 0 {
			compareAt = &p.CompareAt
		}
		var productID int64
		if err := pool.QueryRow(ctx,
			`INSERT INTO products (slug, title, brand, category_id, description, specs, base_price, compare_at_price, featured)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
			p.Slug, p.Title, p.Brand, catIDs[p.Category], p.Desc, specs, p.Price, compareAt, p.Featured,
		).Scan(&productID); err != nil {
			return fmt.Errorf("seed product %s: %w", p.Slug, err)
		}

		variants := [][2]any{{map[string]string{}, true}}
		if len(p.Colors) > 0 {
			variants = nil
			for i, color := range p.Colors {
				variants = append(variants, [2]any{map[string]string{"Color": color}, i == 0})
			}
		}
		for i, v := range variants {
			opts, _ := json.Marshal(v[0])
			sku := fmt.Sprintf("TP-%s-%d", p.Slug[:min(8, len(p.Slug))], i+1)
			if _, err := pool.Exec(ctx,
				`INSERT INTO product_variants (product_id, sku, options, price, stock, is_default)
				 VALUES ($1,$2,$3,$4,$5,$6)`,
				productID, sku, opts, p.Price, p.Stock, v[1],
			); err != nil {
				return fmt.Errorf("seed variant for %s: %w", p.Slug, err)
			}
		}

		if _, err := pool.Exec(ctx,
			`INSERT INTO product_images (product_id, path, alt, sort) VALUES ($1,$2,$3,0)`,
			productID, "/static/img/products/"+p.Slug+".svg", p.Title,
		); err != nil {
			return fmt.Errorf("seed image for %s: %w", p.Slug, err)
		}
	}

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
