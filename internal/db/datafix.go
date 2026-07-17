package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DataFixes applies one-time content repairs to an already-seeded database
// (SeedIfEmpty only runs on fresh DBs, so catalog fixes need this path).
// Each fix version runs once, tracked by a settings key.
func DataFixes(ctx context.Context, pool *pgxpool.Pool) error {
	const key = "datafix_v1"
	var done bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM settings WHERE key=$1)`, key).Scan(&done); err != nil {
		return fmt.Errorf("datafix check: %w", err)
	}
	if done {
		return nil
	}

	// v1: resync product descriptions from the embedded catalog (the original
	// import had lost all line breaks) and replace the placeholder WhatsApp
	// number so header/footer/wa.me links look real in demos.
	var products []catalogProduct
	if err := json.Unmarshal(catalogJSON, &products); err != nil {
		return fmt.Errorf("datafix parse catalog: %w", err)
	}
	for _, p := range products {
		if _, err := pool.Exec(ctx,
			`UPDATE products SET description=$2 WHERE slug=$1`, p.Slug, p.Description); err != nil {
			return fmt.Errorf("datafix description %s: %w", p.Slug, err)
		}
	}
	if _, err := pool.Exec(ctx,
		`UPDATE settings SET value=$1::jsonb WHERE key='whatsapp_number' AND value::text LIKE '%XXXX%'`,
		`"+923001234567"`); err != nil {
		return fmt.Errorf("datafix whatsapp: %w", err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO settings (key, value) VALUES ($1, '"done"'::jsonb) ON CONFLICT (key) DO NOTHING`, key); err != nil {
		return fmt.Errorf("datafix mark: %w", err)
	}
	slog.Info("applied data fixes", "version", key)
	return nil
}
