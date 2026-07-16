package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KodeWorks-1/techport/internal/models"
)

// Admin bundles the store-management operations behind /admin.
type Admin struct {
	pool *pgxpool.Pool
}

func NewAdmin(pool *pgxpool.Pool) *Admin {
	return &Admin{pool: pool}
}

/* ---------- dashboard ---------- */

type Stats struct {
	OrdersToday   int
	RevenueToday  float64
	OrdersWeek    int
	RevenueWeek   float64
	PendingOrders int
}

func (a *Admin) Stats(ctx context.Context) (Stats, error) {
	var s Stats
	err := a.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE created_at::date = CURRENT_DATE),
		       COALESCE(SUM(total) FILTER (WHERE created_at::date = CURRENT_DATE
		                                   AND status NOT IN ('cancelled','returned')), 0),
		       count(*) FILTER (WHERE created_at >= now() - interval '7 days'),
		       COALESCE(SUM(total) FILTER (WHERE created_at >= now() - interval '7 days'
		                                   AND status NOT IN ('cancelled','returned')), 0),
		       count(*) FILTER (WHERE status = 'pending')
		FROM orders`,
	).Scan(&s.OrdersToday, &s.RevenueToday, &s.OrdersWeek, &s.RevenueWeek, &s.PendingOrders)
	return s, err
}

type LowStockRow struct {
	ProductID int64
	Title     string
	Options   map[string]string
	Stock     int
}

func (a *Admin) LowStock(ctx context.Context, threshold, limit int) ([]LowStockRow, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT p.id, p.title, v.options, v.stock
		FROM product_variants v JOIN products p ON p.id = v.product_id
		WHERE p.active AND v.stock <= $1
		ORDER BY v.stock, p.title LIMIT $2`, threshold, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LowStockRow
	for rows.Next() {
		var r LowStockRow
		if err := rows.Scan(&r.ProductID, &r.Title, &r.Options, &r.Stock); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

/* ---------- orders ---------- */

type AdminOrderRow struct {
	ID           int64
	Code         string
	CustomerName string
	Phone        string
	City         string
	Status       string
	Total        float64
	ItemCount    int
	CreatedAt    time.Time
}

func (a *Admin) Orders(ctx context.Context, status string, limit int) ([]AdminOrderRow, error) {
	q := `
		SELECT o.id, o.code, o.customer_name, o.phone, o.city, o.status, o.total,
		       (SELECT COALESCE(SUM(qty),0) FROM order_items WHERE order_id=o.id),
		       o.created_at
		FROM orders o`
	args := []any{}
	if status != "" {
		q += ` WHERE o.status = $1`
		args = append(args, status)
	}
	q += fmt.Sprintf(` ORDER BY o.created_at DESC LIMIT %d`, limit)

	rows, err := a.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminOrderRow
	for rows.Next() {
		var r AdminOrderRow
		if err := rows.Scan(&r.ID, &r.Code, &r.CustomerName, &r.Phone, &r.City,
			&r.Status, &r.Total, &r.ItemCount, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

var ValidOrderStatuses = []string{"pending", "confirmed", "shipped", "delivered", "cancelled", "returned"}

// SetOrderStatus moves an order to a new status, records the event, and
// restocks items when the order enters a terminated state.
func (a *Admin) SetOrderStatus(ctx context.Context, orderID int64, status, note, actor string) error {
	valid := false
	for _, s := range ValidOrderStatuses {
		if s == status {
			valid = true
		}
	}
	if !valid {
		return fmt.Errorf("invalid status %q", status)
	}

	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var prev string
	err = tx.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&prev)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if prev == status {
		return nil
	}

	terminated := func(s string) bool { return s == "cancelled" || s == "returned" }
	if terminated(status) && !terminated(prev) {
		if _, err := tx.Exec(ctx, `
			UPDATE product_variants v SET stock = v.stock + oi.qty
			FROM order_items oi
			WHERE oi.order_id = $1 AND oi.variant_id = v.id`, orderID); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE orders SET status=$2, updated_at=now() WHERE id=$1`, orderID, status); err != nil {
		return err
	}
	if note == "" {
		note = "Status changed to " + status
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO order_events (order_id, status, note, actor)
		VALUES ($1,$2,$3,$4)`, orderID, status, note, actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

/* ---------- products ---------- */

type AdminProductRow struct {
	ID           int64
	Slug         string
	Title        string
	CategoryName string
	BasePrice    float64
	Stock        int
	Active       bool
	Featured     bool
}

func (a *Admin) Products(ctx context.Context) ([]AdminProductRow, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT p.id, p.slug, p.title, c.name, p.base_price,
		       COALESCE((SELECT SUM(stock) FROM product_variants WHERE product_id=p.id), 0),
		       p.active, p.featured
		FROM products p JOIN categories c ON c.id = p.category_id
		ORDER BY p.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminProductRow
	for rows.Next() {
		var r AdminProductRow
		if err := rows.Scan(&r.ID, &r.Slug, &r.Title, &r.CategoryName, &r.BasePrice,
			&r.Stock, &r.Active, &r.Featured); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ProductByID loads a product for editing (including inactive ones).
func (a *Admin) ProductByID(ctx context.Context, id int64) (ProductDetail, error) {
	var d ProductDetail
	err := a.pool.QueryRow(ctx, `
		SELECT p.id, p.slug, p.title, p.brand, p.category_id, p.description, p.specs,
		       p.base_price, p.compare_at_price, p.active, p.featured, p.created_at,
		       c.name, c.slug
		FROM products p JOIN categories c ON c.id = p.category_id
		WHERE p.id=$1`, id,
	).Scan(&d.ID, &d.Slug, &d.Title, &d.Brand, &d.CategoryID, &d.Description, &d.Specs,
		&d.BasePrice, &d.CompareAtPrice, &d.Active, &d.Featured, &d.CreatedAt,
		&d.CategoryName, &d.CategorySlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}

	rows, err := a.pool.Query(ctx, `
		SELECT id, product_id, sku, options, price, stock, is_default
		FROM product_variants WHERE product_id=$1 ORDER BY is_default DESC, id`, id)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var v models.Variant
		if err := rows.Scan(&v.ID, &v.ProductID, &v.SKU, &v.Options, &v.Price, &v.Stock, &v.IsDefault); err != nil {
			return d, err
		}
		d.Variants = append(d.Variants, v)
	}
	if err := rows.Err(); err != nil {
		return d, err
	}

	imgRows, err := a.pool.Query(ctx, `
		SELECT id, product_id, path, alt, sort FROM product_images
		WHERE product_id=$1 ORDER BY sort, id`, id)
	if err != nil {
		return d, err
	}
	defer imgRows.Close()
	for imgRows.Next() {
		var img models.Image
		if err := imgRows.Scan(&img.ID, &img.ProductID, &img.Path, &img.Alt, &img.Sort); err != nil {
			return d, err
		}
		d.Images = append(d.Images, img)
	}
	return d, imgRows.Err()
}

type ProductInput struct {
	Title          string
	Brand          string
	CategoryID     int64
	Description    string
	Specs          map[string]string
	BasePrice      float64
	CompareAtPrice *float64
	Active         bool
	Featured       bool
}

var slugStrip = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.Trim(slugStrip.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if s == "" {
		s = "product"
	}
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// CreateProduct inserts a product plus its default variant and returns the id.
func (a *Admin) CreateProduct(ctx context.Context, in ProductInput, stock int) (int64, error) {
	specs, _ := json.Marshal(in.Specs)
	base := slugify(in.Title)
	slug := base
	var id int64
	for attempt := 2; ; attempt++ {
		var exists bool
		if err := a.pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM products WHERE slug=$1)`, slug).Scan(&exists); err != nil {
			return 0, err
		}
		if !exists {
			break
		}
		slug = fmt.Sprintf("%s-%d", base, attempt)
	}

	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		INSERT INTO products (slug, title, brand, category_id, description, specs,
		                      base_price, compare_at_price, active, featured)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		slug, in.Title, in.Brand, in.CategoryID, in.Description, specs,
		in.BasePrice, in.CompareAtPrice, in.Active, in.Featured,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO product_variants (product_id, sku, options, price, stock, is_default)
		VALUES ($1, $2, '{}', $3, $4, TRUE)`,
		id, fmt.Sprintf("TP-%d-1", id), in.BasePrice, stock); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

func (a *Admin) UpdateProduct(ctx context.Context, id int64, in ProductInput) error {
	specs, _ := json.Marshal(in.Specs)
	tag, err := a.pool.Exec(ctx, `
		UPDATE products SET title=$2, brand=$3, category_id=$4, description=$5, specs=$6,
		       base_price=$7, compare_at_price=$8, active=$9, featured=$10, updated_at=now()
		WHERE id=$1`,
		id, in.Title, in.Brand, in.CategoryID, in.Description, specs,
		in.BasePrice, in.CompareAtPrice, in.Active, in.Featured)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

/* ---------- variants ---------- */

func (a *Admin) AddVariant(ctx context.Context, productID int64, options map[string]string, price float64, stock int) error {
	opts, _ := json.Marshal(options)
	_, err := a.pool.Exec(ctx, `
		INSERT INTO product_variants (product_id, sku, options, price, stock, is_default)
		VALUES ($1, '', $2, $3, $4,
		        NOT EXISTS (SELECT 1 FROM product_variants WHERE product_id=$1))`,
		productID, opts, price, stock)
	return err
}

func (a *Admin) UpdateVariant(ctx context.Context, id int64, options map[string]string, price float64, stock int) error {
	opts, _ := json.Marshal(options)
	_, err := a.pool.Exec(ctx,
		`UPDATE product_variants SET options=$2, price=$3, stock=$4 WHERE id=$1`,
		id, opts, price, stock)
	return err
}

// DeleteVariant refuses to remove a product's last variant.
func (a *Admin) DeleteVariant(ctx context.Context, id int64) error {
	_, err := a.pool.Exec(ctx, `
		DELETE FROM product_variants v
		WHERE v.id = $1
		  AND (SELECT count(*) FROM product_variants WHERE product_id = v.product_id) > 1`, id)
	return err
}

/* ---------- images ---------- */

func (a *Admin) AddImage(ctx context.Context, productID int64, path, alt string) error {
	_, err := a.pool.Exec(ctx, `
		INSERT INTO product_images (product_id, path, alt, sort)
		VALUES ($1, $2, $3,
		        COALESCE((SELECT MAX(sort)+1 FROM product_images WHERE product_id=$1), 0))`,
		productID, path, alt)
	return err
}

func (a *Admin) DeleteImage(ctx context.Context, id int64) (string, error) {
	var path string
	err := a.pool.QueryRow(ctx,
		`DELETE FROM product_images WHERE id=$1 RETURNING path`, id).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return path, err
}

/* ---------- categories ---------- */

type AdminCategoryRow struct {
	models.Category
	ProductCount int
}

func (a *Admin) Categories(ctx context.Context) ([]AdminCategoryRow, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT c.id, c.slug, c.name, c.sort,
		       (SELECT count(*) FROM products WHERE category_id=c.id)
		FROM categories c ORDER BY c.sort, c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminCategoryRow
	for rows.Next() {
		var r AdminCategoryRow
		if err := rows.Scan(&r.ID, &r.Slug, &r.Name, &r.Sort, &r.ProductCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (a *Admin) CreateCategory(ctx context.Context, name string) error {
	_, err := a.pool.Exec(ctx, `
		INSERT INTO categories (slug, name, sort)
		VALUES ($1, $2, (SELECT COALESCE(MAX(sort)+1, 0) FROM categories))
		ON CONFLICT (slug) DO NOTHING`, slugify(name), name)
	return err
}

/* ---------- settings ---------- */

func (a *Admin) SaveSetting(ctx context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = a.pool.Exec(ctx, `
		INSERT INTO settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, raw)
	return err
}
