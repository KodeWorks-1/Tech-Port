package services

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KodeWorks-1/techport/internal/models"
)

type Catalog struct {
	pool *pgxpool.Pool
}

func NewCatalog(pool *pgxpool.Pool) *Catalog {
	return &Catalog{pool: pool}
}

func (c *Catalog) Categories(ctx context.Context) ([]models.Category, error) {
	rows, err := c.pool.Query(ctx,
		`SELECT id, slug, name, sort FROM categories ORDER BY sort, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Category
	for rows.Next() {
		var cat models.Category
		if err := rows.Scan(&cat.ID, &cat.Slug, &cat.Name, &cat.Sort); err != nil {
			return nil, err
		}
		out = append(out, cat)
	}
	return out, rows.Err()
}

const productCardQuery = `
	SELECT p.id, p.slug, p.title, p.brand, c.name,
	       p.base_price, p.compare_at_price,
	       COALESCE((SELECT i.path FROM product_images i
	                 WHERE i.product_id = p.id ORDER BY i.sort LIMIT 1), '')
	FROM products p
	JOIN categories c ON c.id = p.category_id
	WHERE p.active`

func (c *Catalog) Featured(ctx context.Context, limit int) ([]models.ProductCard, error) {
	return c.cards(ctx, productCardQuery+` AND p.featured ORDER BY p.created_at DESC LIMIT $1`, limit)
}

func (c *Catalog) Latest(ctx context.Context, limit int) ([]models.ProductCard, error) {
	return c.cards(ctx, productCardQuery+` ORDER BY p.created_at DESC LIMIT $1`, limit)
}

func (c *Catalog) cards(ctx context.Context, query string, args ...any) ([]models.ProductCard, error) {
	rows, err := c.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ProductCard
	for rows.Next() {
		var p models.ProductCard
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Brand, &p.CategoryName,
			&p.Price, &p.CompareAtPrice, &p.Image); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
