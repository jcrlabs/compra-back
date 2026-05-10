package models

import (
	"time"

	"github.com/google/uuid"
)

type Supermarket string

const (
	Mercadona Supermarket = "mercadona"
	Froiz     Supermarket = "froiz"
	Gadis     Supermarket = "gadis"
	Carrefour Supermarket = "carrefour"
	Alcampo   Supermarket = "alcampo"
	Eroski    Supermarket = "eroski"
)

var AllSupermarkets = []Supermarket{Mercadona, Froiz, Gadis, Carrefour, Alcampo, Eroski}

func (s Supermarket) Valid() bool {
	for _, v := range AllSupermarkets {
		if v == s {
			return true
		}
	}
	return false
}

// User

type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Category

type Category struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Slug      string     `json:"slug"`
	ParentID  *uuid.UUID `json:"parent_id,omitempty"`
	Icon      string     `json:"icon,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Product

type Product struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	Brand        string     `json:"brand,omitempty"`
	CategoryID   *uuid.UUID `json:"category_id,omitempty"`
	Category     *Category  `json:"category,omitempty"`
	Unit         string     `json:"unit"`
	UnitQuantity float64    `json:"unit_quantity"`
	ImageURL     string     `json:"image_url,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Price

type Price struct {
	ID           uuid.UUID   `json:"id"`
	ProductID    uuid.UUID   `json:"product_id"`
	Supermarket  Supermarket `json:"supermarket"`
	Price        float64     `json:"price"`
	PricePerUnit *float64    `json:"price_per_unit,omitempty"`
	ScrapedAt    time.Time   `json:"scraped_at"`
	ExternalID   string      `json:"external_id,omitempty"`
	ExternalURL  string      `json:"external_url,omitempty"`
}

// ProductWithPrices — used for comparison views
type ProductWithPrices struct {
	Product
	Prices []Price `json:"prices"`
}

// BestPrice — cheapest option per product
type BestPrice struct {
	ProductID   uuid.UUID   `json:"product_id"`
	Supermarket Supermarket `json:"supermarket"`
	Price       float64     `json:"price"`
	ExternalURL string      `json:"external_url,omitempty"`
}

// ShoppingList

type ShoppingList struct {
	ID         uuid.UUID  `json:"id"`
	OwnerID    uuid.UUID  `json:"owner_id"`
	Name       string     `json:"name"`
	ShareToken uuid.UUID  `json:"share_token"`
	Members    []User     `json:"members,omitempty"`
	Items      []ListItem `json:"items,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type ListItem struct {
	ID        uuid.UUID  `json:"id"`
	ListID    uuid.UUID  `json:"list_id"`
	ProductID *uuid.UUID `json:"product_id,omitempty"`
	Product   *Product   `json:"product,omitempty"`
	Name      string     `json:"name"`
	Quantity  float64    `json:"quantity"`
	Unit      string     `json:"unit,omitempty"`
	Checked   bool       `json:"checked"`
	Notes     string     `json:"notes,omitempty"`
	AddedBy   *uuid.UUID `json:"added_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// OptimizedList — shopping list with best-price breakdown per supermarket
type OptimizedList struct {
	TotalBestPrice float64                    `json:"total_best_price"`
	TotalSingle    float64                    `json:"total_single_super"`
	SavedAmount    float64                    `json:"saved_amount"`
	Groups         map[Supermarket]SuperGroup `json:"groups"`
	Uncovered      []ListItem                 `json:"uncovered"` // items with no price found
}

type SuperGroup struct {
	Supermarket Supermarket     `json:"supermarket"`
	Items       []OptimizedItem `json:"items"`
	Subtotal    float64         `json:"subtotal"`
}

type OptimizedItem struct {
	Item        ListItem    `json:"item"`
	Price       float64     `json:"price"`
	Supermarket Supermarket `json:"supermarket"`
	ExternalURL string      `json:"external_url,omitempty"`
}

// Offer — product with a price drop vs 30-day average
type Offer struct {
	ProductID    uuid.UUID   `json:"product_id"`
	Name         string      `json:"name"`
	Brand        string      `json:"brand,omitempty"`
	ImageURL     string      `json:"image_url,omitempty"`
	Unit         string      `json:"unit"`
	UnitQuantity float64     `json:"unit_quantity"`
	Supermarket  Supermarket `json:"supermarket"`
	CurrentPrice float64     `json:"current_price"`
	AvgPrice     float64     `json:"avg_price"`
	DiscountPct  float64     `json:"discount_pct"`
}

// UserSupermarketPrefs

type UserSupermarketPrefs struct {
	UserID       uuid.UUID            `json:"user_id"`
	Supermarkets map[Supermarket]bool `json:"supermarkets"`
}
