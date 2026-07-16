package services

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Settings struct {
	pool *pgxpool.Pool
}

func NewSettings(pool *pgxpool.Pool) *Settings {
	return &Settings{pool: pool}
}

func (s *Settings) get(ctx context.Context, key string, dest any) error {
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&raw); err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

type ShippingConfig struct {
	Flat      float64 `json:"flat"`
	FreeAbove float64 `json:"free_above"`
}

func (s *Settings) Shipping(ctx context.Context) (ShippingConfig, error) {
	var cfg ShippingConfig
	err := s.get(ctx, "shipping_fee", &cfg)
	return cfg, err
}

func (s *Settings) WhatsAppNumber(ctx context.Context) (string, error) {
	var n string
	err := s.get(ctx, "whatsapp_number", &n)
	return n, err
}

type PaymentMethod struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}

func (s *Settings) PaymentMethods(ctx context.Context) ([]PaymentMethod, error) {
	var methods []PaymentMethod
	err := s.get(ctx, "payment_methods", &methods)
	return methods, err
}
