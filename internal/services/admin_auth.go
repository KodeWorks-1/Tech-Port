package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var ErrBadCredentials = errors.New("invalid email or password")

type AdminAuth struct {
	pool *pgxpool.Pool
}

func NewAdminAuth(pool *pgxpool.Pool) *AdminAuth {
	return &AdminAuth{pool: pool}
}

// Login verifies credentials and returns a new session token (30 days).
func (a *AdminAuth) Login(ctx context.Context, email, password string) (string, error) {
	var id int64
	var hash string
	err := a.pool.QueryRow(ctx,
		`SELECT id, password_hash FROM admin_users WHERE lower(email)=lower($1)`, email,
	).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrBadCredentials
	}
	if err != nil {
		return "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", ErrBadCredentials
	}

	buf := make([]byte, 24)
	rand.Read(buf)
	token := hex.EncodeToString(buf)
	_, err = a.pool.Exec(ctx, `
		INSERT INTO admin_sessions (token, admin_id, expires_at)
		VALUES ($1, $2, now() + interval '30 days')`, token, id)
	if err != nil {
		return "", err
	}
	return token, nil
}

// AdminByToken returns the admin's email for a valid, unexpired session.
func (a *AdminAuth) AdminByToken(ctx context.Context, token string) (string, error) {
	var email string
	err := a.pool.QueryRow(ctx, `
		SELECT u.email FROM admin_sessions s
		JOIN admin_users u ON u.id = s.admin_id
		WHERE s.token=$1 AND s.expires_at > now()`, token,
	).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return email, err
}

func (a *AdminAuth) Logout(ctx context.Context, token string) error {
	_, err := a.pool.Exec(ctx, `DELETE FROM admin_sessions WHERE token=$1`, token)
	return err
}
