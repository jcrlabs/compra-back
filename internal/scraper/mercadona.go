package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// MercadonaScraper uses Mercadona's unofficial internal API.
// Endpoint: https://tiendas.mercadona.es/api/categories/
// Each category returns products with prices. No JS rendering required.

type MercadonaScraper struct {
	client     *http.Client
	postalCode string
	userAgent  string
}

func NewMercadonaScraper(postalCode, userAgent string) *MercadonaScraper {
	return &MercadonaScraper{
		postalCode: postalCode,
		userAgent:  userAgent,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *MercadonaScraper) Name() string { return "mercadona" }

func (s *MercadonaScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	// Step 1: get all category IDs
	categories, err := s.fetchCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("mercadona: fetch categories: %w", err)
	}

	var products []RawProduct
	for _, cat := range categories {
		catProducts, err := s.fetchCategory(ctx, cat.ID, cat.Name)
		if err != nil {
			// Log and continue — partial data is better than nothing
			continue
		}
		products = append(products, catProducts...)
		// Polite delay between requests
		select {
		case <-ctx.Done():
			return products, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return products, nil
}

type mercadonaCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type mercadonaCategoryResponse struct {
	Results []struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Categories []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"categories"`
	} `json:"results"`
}

func (s *MercadonaScraper) fetchCategories(ctx context.Context) ([]mercadonaCategory, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://tiendas.mercadona.es/api/categories/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data mercadonaCategoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var cats []mercadonaCategory
	for _, root := range data.Results {
		for _, sub := range root.Categories {
			cats = append(cats, mercadonaCategory{ID: sub.ID, Name: sub.Name})
		}
	}
	return cats, nil
}

type mercadonaProductResponse struct {
	Results []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Brand       string `json:"brand"`
		Thumbnail   string `json:"thumbnail"`
		PriceInstructions struct {
			UnitPrice       string `json:"unit_price"`
			ReferencePrice  string `json:"reference_price"`
			ReferenceFormat string `json:"reference_format"`
		} `json:"price_instructions"`
		PackagingType string `json:"packaging_type"`
	} `json:"results"`
}

func (s *MercadonaScraper) fetchCategory(ctx context.Context, catID int, catName string) ([]RawProduct, error) {
	url := fmt.Sprintf("https://tiendas.mercadona.es/api/categories/%d/?postal_code=%s", catID, s.postalCode)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data mercadonaProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var products []RawProduct
	for _, p := range data.Results {
		var price float64
		fmt.Sscanf(p.PriceInstructions.UnitPrice, "%f", &price)

		unit, qty := ParseUnit(p.PackagingType)
		products = append(products, RawProduct{
			ExternalID:   p.ID,
			Name:         p.DisplayName,
			Brand:        p.Brand,
			Price:        price,
			Unit:         unit,
			UnitQuantity: qty,
			Category:     catName,
			ImageURL:     p.Thumbnail,
			ProductURL:   fmt.Sprintf("https://tiendas.mercadona.es/product/%s", p.ID),
		})
	}
	return products, nil
}
