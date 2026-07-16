package models

import "time"

type Category struct {
	ID   int64
	Slug string
	Name string
	Sort int
}

// ProductCard is the lightweight shape rendered in grids and carousels.
type ProductCard struct {
	ID             int64
	Slug           string
	Title          string
	Brand          string
	CategoryName   string
	Price          float64
	CompareAtPrice *float64
	Image          string
	VariantID      int64 // default variant, for one-click add-to-cart
	InStock        bool
}

type Product struct {
	ID             int64
	Slug           string
	Title          string
	Brand          string
	CategoryID     int64
	Description    string
	Specs          map[string]string
	BasePrice      float64
	CompareAtPrice *float64
	Active         bool
	Featured       bool
	CreatedAt      time.Time
}

type Variant struct {
	ID        int64
	ProductID int64
	SKU       string
	Options   map[string]string
	Price     float64
	Stock     int
	IsDefault bool
}

type Image struct {
	ID        int64
	ProductID int64
	Path      string
	Alt       string
	Sort      int
}
