package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
			slog.Warn("mercadona: skip category", slog.Int("id", cat.ID), slog.String("name", cat.Name), slog.String("error", err.Error()))
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

func (s *MercadonaScraper) setHeaders(req *http.Request) {
	ua := s.userAgent
	if ua == "" || ua == "Mozilla/5.0 (compatible; jcrlabs-bot/1.0)" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9")
	req.Header.Set("Referer", "https://tiendas.mercadona.es/")
	req.Header.Set("Origin", "https://tiendas.mercadona.es")
}

func (s *MercadonaScraper) fetchCategories(ctx context.Context) ([]mercadonaCategory, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://tiendas.mercadona.es/api/categories/", nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from categories endpoint", resp.StatusCode)
	}

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
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mercadona category %d: status %d", catID, resp.StatusCode)
	}

	var data mercadonaProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("mercadona category %d: decode: %w", catID, err)
	}
	slog.Debug("mercadona: fetched category", slog.Int("id", catID), slog.String("name", catName), slog.Int("products", len(data.Results)))

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
