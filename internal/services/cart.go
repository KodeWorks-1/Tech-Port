package services

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrOutOfStock = errors.New("out of stock")

type Cart struct {
	pool *pgxpool.Pool
}

func NewCart(pool *pgxpool.Pool) *Cart {
	return &Cart{pool: pool}
}

type CartItem struct {
	ID        int64
	VariantID int64
	Title     string
	Slug      string
	Image     string
	Options   map[string]string
	Price     float64
	Stock     int
	Qty       int
}

func (i CartItem) LineTotal() float64 { return i.Price * float64(i.Qty) }

type CartView struct {
	Items    []CartItem
	Subtotal float64
	Count    int
}

func (c *Cart) cartID(ctx context.Context, sessionID string) (int64, error) {
	var id int64
	err := c.pool.QueryRow(ctx, `
		INSERT INTO carts (session_id) VALUES ($1)
		ON CONFLICT (session_id) DO UPDATE SET updated_at = now()
		RETURNING id`, sessionID,
	).Scan(&id)
	return id, err
}

// Add puts qty of a variant in the session's cart, clamped to available stock.
func (c *Cart) Add(ctx context.Context, sessionID string, variantID int64, qty int) error {
	if qty < 1 {
		qty = 1
	}
	cartID, err := c.cartID(ctx, sessionID)
	if err != nil {
		return err
	}
	tag, err := c.pool.Exec(ctx, `
		INSERT INTO cart_items (cart_id, variant_id, qty, price_at_add)
		SELECT $1, v.id, LEAST($3, v.stock), v.price
		FROM product_variants v WHERE v.id=$2 AND v.stock > 0
		ON CONFLICT (cart_id, variant_id) DO UPDATE
		SET qty = LEAST(cart_items.qty + EXCLUDED.qty,
		                (SELECT stock FROM product_variants WHERE id=$2))`,
		cartID, variantID, qty)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOutOfStock
	}
	return nil
}

// SetQty updates an item's quantity (clamped to stock); qty <= 0 removes it.
// The item must belong to the session's cart.
func (c *Cart) SetQty(ctx context.Context, sessionID string, itemID int64, qty int) error {
	if qty <= 0 {
		_, err := c.pool.Exec(ctx, `
			DELETE FROM cart_items ci USING carts c
			WHERE ci.id=$2 AND ci.cart_id=c.id AND c.session_id=$1`, sessionID, itemID)
		return err
	}
	_, err := c.pool.Exec(ctx, `
		UPDATE cart_items ci SET qty = LEAST($3, v.stock)
		FROM carts c, product_variants v
		WHERE ci.id=$2 AND ci.cart_id=c.id AND c.session_id=$1 AND v.id=ci.variant_id`,
		sessionID, itemID, qty)
	return err
}

func (c *Cart) View(ctx context.Context, sessionID string) (CartView, error) {
	var view CartView
	rows, err := c.pool.Query(ctx, `
		SELECT ci.id, v.id, p.title, p.slug,
		       COALESCE((SELECT i.path FROM product_images i
		                 WHERE i.product_id = p.id ORDER BY i.sort LIMIT 1), ''),
		       v.options, v.price, v.stock, ci.qty
		FROM cart_items ci
		JOIN carts c ON c.id = ci.cart_id
		JOIN product_variants v ON v.id = ci.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE c.session_id=$1
		ORDER BY ci.id`, sessionID)
	if err != nil {
		return view, err
	}
	defer rows.Close()

	for rows.Next() {
		var it CartItem
		if err := rows.Scan(&it.ID, &it.VariantID, &it.Title, &it.Slug, &it.Image,
			&it.Options, &it.Price, &it.Stock, &it.Qty); err != nil {
			return view, err
		}
		view.Items = append(view.Items, it)
		view.Subtotal += it.LineTotal()
		view.Count += it.Qty
	}
	return view, rows.Err()
}

func (c *Cart) Count(ctx context.Context, sessionID string) (int, error) {
	var n int
	err := c.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(ci.qty), 0)
		FROM cart_items ci JOIN carts c ON c.id = ci.cart_id
		WHERE c.session_id=$1`, sessionID,
	).Scan(&n)
	return n, err
}
