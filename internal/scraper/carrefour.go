package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	"lacteos-huevos":   "Lácteos y huevos",
	"frutas-verduras":  "Frutas y verduras",
	"carniceria":       "Carnicería",
	"pescaderia":       "Pescadería",
	"pan-bolleria":     "Pan y bollería",
	"congelados":       "Congelados",
	"bebidas":          "Bebidas",
	"aceite-conservas": "Aceite y conservas",
	"drogueria":        "Droguería",
	"higiene-salud":    "Higiene y salud",
	"mascotas":         "Mascotas",
	"cafe-desayuno":    "Café y desayuno",
}

func (s *CarrefourScraper) setHeaders(req *http.Request) {
	ua := s.userAgent
	if ua == "" || ua == "Mozilla/5.0 (compatible; jcrlabs-bot/1.0)" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9")
	req.Header.Set("Referer", "https://www.carrefour.es/")
}

func (s *CarrefourScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	var all []RawProduct
	for slug, name := range carrefourCategories {
		products, err := s.scrapeCategory(ctx, slug, name)
		if err != nil {
			slog.Warn("carrefour: skip category", slog.String("slug", slug), slog.String("error", err.Error()))
			continue
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
	s.setHeaders(req)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("carrefour category %s: status %d", slug, resp.StatusCode)
	}

	var data carrefourSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("carrefour category %s: decode: %w", slug, err)
	}
	slog.Debug("carrefour: fetched category", slog.String("slug", slug), slog.Int("products", len(data.Products)))

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
