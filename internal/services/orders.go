package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmptyCart    = errors.New("cart is empty")
	ErrStockChanged = errors.New("an item just went out of stock")
)

type Orders struct {
	pool     *pgxpool.Pool
	settings *Settings
}

func NewOrders(pool *pgxpool.Pool, settings *Settings) *Orders {
	return &Orders{pool: pool, settings: settings}
}

type PlaceOrderInput struct {
	CustomerName  string
	Phone         string
	Address       string
	City          string
	PaymentMethod string
	Notes         string
}

type Order struct {
	ID            int64
	Code          string
	SessionID     string
	CustomerName  string
	Phone         string
	Address       string
	City          string
	PaymentMethod string
	Status        string
	Subtotal      float64
	ShippingFee   float64
	Total         float64
	Notes         string
	CreatedAt     time.Time
}

type OrderItem struct {
	Title   string
	Options map[string]string
	Price   float64
	Qty     int
}

func (i OrderItem) LineTotal() float64 { return i.Price * float64(i.Qty) }

type OrderEvent struct {
	Status    string
	Note      string
	CreatedAt time.Time
}

type OrderDetail struct {
	Order
	Items  []OrderItem
	Events []OrderEvent
}

// Place converts the session's cart into an order: locks stock, decrements
// it, snapshots items, logs the first event, and empties the cart — all in
// one transaction. Returns the order code.
func (o *Orders) Place(ctx context.Context, sessionID string, in PlaceOrderInput) (string, error) {
	shipping, err := o.settings.Shipping(ctx)
	if err != nil {
		return "", fmt.Errorf("load shipping config: %w", err)
	}

	tx, err := o.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT ci.id, ci.qty, v.id, v.price, v.stock, v.options, p.title
		FROM cart_items ci
		JOIN carts c ON c.id = ci.cart_id
		JOIN product_variants v ON v.id = ci.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE c.session_id = $1
		ORDER BY ci.id
		FOR UPDATE OF v`, sessionID)
	if err != nil {
		return "", err
	}

	type line struct {
		itemID    int64
		qty       int
		variantID int64
		price     float64
		stock     int
		options   map[string]string
		title     string
	}
	var lines []line
	var subtotal float64
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.itemID, &l.qty, &l.variantID, &l.price, &l.stock, &l.options, &l.title); err != nil {
			rows.Close()
			return "", err
		}
		lines = append(lines, l)
		subtotal += l.price * float64(l.qty)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", ErrEmptyCart
	}

	fee := shipping.Flat
	if subtotal >= shipping.FreeAbove {
		fee = 0
	}
	total := subtotal + fee

	var orderID int64
	var code string
	for attempt := 0; ; attempt++ {
		code = genOrderCode()
		err = tx.QueryRow(ctx, `
			INSERT INTO orders (code, session_id, customer_name, phone, address, city,
			                    payment_method, status, subtotal, shipping_fee, total, notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7,'pending',$8,$9,$10,$11)
			RETURNING id`,
			code, sessionID, in.CustomerName, in.Phone, in.Address, in.City,
			in.PaymentMethod, subtotal, fee, total, in.Notes,
		).Scan(&orderID)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && attempt < 5 {
			continue // code collision — regenerate
		}
		if err != nil {
			return "", err
		}
		break
	}

	for _, l := range lines {
		tag, err := tx.Exec(ctx,
			`UPDATE product_variants SET stock = stock - $2 WHERE id = $1 AND stock >= $2`,
			l.variantID, l.qty)
		if err != nil {
			return "", err
		}
		if tag.RowsAffected() == 0 {
			return "", ErrStockChanged
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_items (order_id, variant_id, title, options, price, qty)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			orderID, l.variantID, l.title, l.options, l.price, l.qty); err != nil {
			return "", err
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO order_events (order_id, status, note, actor)
		VALUES ($1, 'pending', 'Order placed', 'customer')`, orderID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM cart_items ci USING carts c
		WHERE ci.cart_id = c.id AND c.session_id = $1`, sessionID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return code, nil
}

func (o *Orders) ByCode(ctx context.Context, code string) (OrderDetail, error) {
	var d OrderDetail
	err := o.pool.QueryRow(ctx, `
		SELECT id, code, session_id, customer_name, phone, address, city,
		       payment_method, status, subtotal, shipping_fee, total, notes, created_at
		FROM orders WHERE code = $1`, code,
	).Scan(&d.ID, &d.Code, &d.SessionID, &d.CustomerName, &d.Phone, &d.Address, &d.City,
		&d.PaymentMethod, &d.Status, &d.Subtotal, &d.ShippingFee, &d.Total, &d.Notes, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}

	rows, err := o.pool.Query(ctx,
		`SELECT title, options, price, qty FROM order_items WHERE order_id=$1 ORDER BY id`, d.ID)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.Title, &it.Options, &it.Price, &it.Qty); err != nil {
			return d, err
		}
		d.Items = append(d.Items, it)
	}
	if err := rows.Err(); err != nil {
		return d, err
	}

	evRows, err := o.pool.Query(ctx,
		`SELECT status, note, created_at FROM order_events WHERE order_id=$1 ORDER BY created_at, id`, d.ID)
	if err != nil {
		return d, err
	}
	defer evRows.Close()
	for evRows.Next() {
		var ev OrderEvent
		if err := evRows.Scan(&ev.Status, &ev.Note, &ev.CreatedAt); err != nil {
			return d, err
		}
		d.Events = append(d.Events, ev)
	}
	return d, evRows.Err()
}

// genOrderCode returns codes like "TP-7K2M9Q" using an alphabet without
// easily-confused characters.
func genOrderCode() string {
	const alphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	buf := make([]byte, 6)
	rand.Read(buf)
	out := make([]byte, 6)
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return "TP-" + string(out)
}
