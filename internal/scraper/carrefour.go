package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CarrefourScraper uses Carrefour's internal search API.
// The API is JSON-based and returns product listings.

type CarrefourScraper struct {
	client    *http.Client
	userAgent string
}

func NewCarrefourScraper(userAgent string) *CarrefourScraper {
	return &CarrefourScraper{
		userAgent: userAgent,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *CarrefourScraper) Name() string { return "carrefour" }

// carrefourCategories maps category slug → display name for the main grocery sections.
var carrefourCategories = map[string]string{
	"lacteos-huevos":    "Lácteos y huevos",
	"frutas-verduras":   "Frutas y verduras",
	"carniceria":        "Carnicería",
	"pescaderia":        "Pescadería",
	"pan-bolleria":      "Pan y bollería",
	"congelados":        "Congelados",
	"bebidas":           "Bebidas",
	"aceite-conservas":  "Aceite y conservas",
	"drogueria":         "Droguería",
	"higiene-salud":     "Higiene y salud",
	"mascotas":          "Mascotas",
	"cafe-desayuno":     "Café y desayuno",
}

func (s *CarrefourScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	var all []RawProduct
	for slug, name := range carrefourCategories {
		products, err := s.scrapeCategory(ctx, slug, name)
		if err != nil {
			continue // partial — keep going
		}
		all = append(all, products...)
		select {
		case <-ctx.Done():
			return all, ctx.Err()
		case <-time.After(800 * time.Millisecond):
		}
	}
	return all, nil
}

type carrefourSearchResponse struct {
	Products []struct {
		ID          string  `json:"id"`
		DisplayName string  `json:"display_name"`
		Brand       string  `json:"brand"`
		Price       float64 `json:"price"`
		ImageURL    string  `json:"image_url"`
		Format      string  `json:"format"`
		Link        string  `json:"link"`
	} `json:"products"`
}

func (s *CarrefourScraper) scrapeCategory(ctx context.Context, slug, catName string) ([]RawProduct, error) {
	url := fmt.Sprintf("https://www.carrefour.es/api/search?category=%s&page_size=48&offset=0", slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data carrefourSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var products []RawProduct
	for _, p := range data.Products {
		unit, qty := ParseUnit(p.Format)
		products = append(products, RawProduct{
			ExternalID:   p.ID,
			Name:         p.DisplayName,
			Brand:        p.Brand,
			Price:        p.Price,
			Unit:         unit,
			UnitQuantity: qty,
			Category:     catName,
			ImageURL:     p.ImageURL,
			ProductURL:   p.Link,
		})
	}
	return products, nil
}
