package services

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var ErrPhoneTaken = errors.New("phone already registered")

type User struct {
	ID      int64
	Name    string
	Phone   string
	Email   string
	Address string
	City    string
}

type Users struct {
	pool *pgxpool.Pool
}

func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{pool: pool}
}

// Register creates an account keyed by (normalized) phone number.
func (u *Users) Register(ctx context.Context, name, phone, password, email string) (int64, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	var id int64
	err = u.pool.QueryRow(ctx, `
		INSERT INTO users (name, phone, email, password_hash)
		VALUES ($1, $2, $3, $4) RETURNING id`,
		name, phone, email, string(hash),
	).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return 0, ErrPhoneTaken
	}
	return id, err
}

func (u *Users) Login(ctx context.Context, phone, password string) (int64, error) {
	var id int64
	var hash string
	err := u.pool.QueryRow(ctx,
		`SELECT id, password_hash FROM users WHERE phone=$1`, phone,
	).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrBadCredentials
	}
	if err != nil {
		return 0, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return 0, ErrBadCredentials
	}
	return id, nil
}

// AttachSession logs the browser session in as the user.
func (u *Users) AttachSession(ctx context.Context, sessionID string, userID int64) error {
	_, err := u.pool.Exec(ctx, `
		INSERT INTO user_sessions (session_id, user_id) VALUES ($1, $2)
		ON CONFLICT (session_id) DO UPDATE SET user_id = EXCLUDED.user_id`,
		sessionID, userID)
	return err
}

func (u *Users) DetachSession(ctx context.Context, sessionID string) error {
	_, err := u.pool.Exec(ctx, `DELETE FROM user_sessions WHERE session_id=$1`, sessionID)
	return err
}

// BySession returns the logged-in user for a session, or ErrNotFound.
func (u *Users) BySession(ctx context.Context, sessionID string) (User, error) {
	var usr User
	err := u.pool.QueryRow(ctx, `
		SELECT u.id, u.name, u.phone, u.email, u.address, u.city
		FROM user_sessions s JOIN users u ON u.id = s.user_id
		WHERE s.session_id=$1`, sessionID,
	).Scan(&usr.ID, &usr.Name, &usr.Phone, &usr.Email, &usr.Address, &usr.City)
	if errors.Is(err, pgx.ErrNoRows) {
		return usr, ErrNotFound
	}
	return usr, err
}

func (u *Users) UpdateProfile(ctx context.Context, id int64, name, email, address, city string) error {
	_, err := u.pool.Exec(ctx, `
		UPDATE users SET name=$2, email=$3, address=$4, city=$5 WHERE id=$1`,
		id, name, email, address, city)
	return err
}

// RememberAddress fills in a blank saved address after a checkout.
func (u *Users) RememberAddress(ctx context.Context, id int64, address, city string) error {
	_, err := u.pool.Exec(ctx, `
		UPDATE users SET address=$2, city=$3 WHERE id=$1 AND address=''`,
		id, address, city)
	return err
}
