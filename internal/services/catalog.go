package services

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KodeWorks-1/techport/internal/models"
)

var ErrNotFound = errors.New("not found")

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

func (c *Catalog) CategoryBySlug(ctx context.Context, slug string) (models.Category, error) {
	var cat models.Category
	err := c.pool.QueryRow(ctx,
		`SELECT id, slug, name, sort FROM categories WHERE slug=$1`, slug,
	).Scan(&cat.ID, &cat.Slug, &cat.Name, &cat.Sort)
	if errors.Is(err, pgx.ErrNoRows) {
		return cat, ErrNotFound
	}
	return cat, err
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

func (c *Catalog) Related(ctx context.Context, categoryID, excludeProductID int64, limit int) ([]models.ProductCard, error) {
	return c.cards(ctx,
		productCardQuery+` AND p.category_id=$1 AND p.id<>$2 ORDER BY p.featured DESC, p.created_at DESC LIMIT $3`,
		categoryID, excludeProductID, limit)
}

// ProductsByCategory returns one page of product cards plus the total count.
func (c *Catalog) ProductsByCategory(ctx context.Context, categoryID int64, sort string, page, perPage int) ([]models.ProductCard, int, error) {
	order := `p.created_at DESC`
	switch sort {
	case "price_asc":
		order = `p.base_price ASC`
	case "price_desc":
		order = `p.base_price DESC`
	}

	var total int
	if err := c.pool.QueryRow(ctx,
		`SELECT count(*) FROM products p WHERE p.active AND p.category_id=$1`, categoryID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	cards, err := c.cards(ctx,
		productCardQuery+` AND p.category_id=$1 ORDER BY `+order+` LIMIT $2 OFFSET $3`,
		categoryID, perPage, offset)
	return cards, total, err
}

// ProductDetail is everything the product page needs.
type ProductDetail struct {
	models.Product
	CategoryName string
	CategorySlug string
	Variants     []models.Variant
	Images       []models.Image
}

func (c *Catalog) ProductBySlug(ctx context.Context, slug string) (ProductDetail, error) {
	var d ProductDetail
	err := c.pool.QueryRow(ctx, `
		SELECT p.id, p.slug, p.title, p.brand, p.category_id, p.description, p.specs,
		       p.base_price, p.compare_at_price, p.active, p.featured, p.created_at,
		       c.name, c.slug
		FROM products p
		JOIN categories c ON c.id = p.category_id
		WHERE p.slug=$1 AND p.active`, slug,
	).Scan(&d.ID, &d.Slug, &d.Title, &d.Brand, &d.CategoryID, &d.Description, &d.Specs,
		&d.BasePrice, &d.CompareAtPrice, &d.Active, &d.Featured, &d.CreatedAt,
		&d.CategoryName, &d.CategorySlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}

	rows, err := c.pool.Query(ctx, `
		SELECT id, product_id, sku, options, price, stock, is_default
		FROM product_variants WHERE product_id=$1 ORDER BY is_default DESC, id`, d.ID)
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

	imgRows, err := c.pool.Query(ctx, `
		SELECT id, product_id, path, alt, sort
		FROM product_images WHERE product_id=$1 ORDER BY sort, id`, d.ID)
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
