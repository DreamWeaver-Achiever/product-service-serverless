package models

import (
	"time"
)

// Product represents a product in the database and cache
type Product struct {
	ID         string    `json:"id"` // UUID as string
	Name       string    `json:"name"`
	Image      *string   `json:"image"` // Pointer for nullable field
	Price      float64   `json:"price"`
	Qty        int       `json:"qty"`
	OutOfStock bool      `json:"out_of_stock"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ProductCSV represents a product as read from a CSV file
type ProductCSV struct {
	ID    string  `csv:"id"` // Optional: if CSV has ID, else generate
	Name  string  `csv:"name"`
	Image string  `csv:"image"`
	Price float64 `csv:"price"`
	Qty   int     `csv:"qty"`
	// out_of_stock is derived from qty, not directly from CSV
}
