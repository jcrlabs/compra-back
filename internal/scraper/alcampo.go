package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// AlcampoScraper uses Alcampo's search/category API.
// Alcampo (Auchan España) exposes a JSON API for their online shop.

type AlcampoScraper struct {
	client    *http.Client
	userAgent string
}

func NewAlcampoScraper(userAgent string) *AlcampoScraper {
	return &AlcampoScraper{
		userAgent: userAgent,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *AlcampoScraper) Name() string { return "alcampo" }

var alcampoCategories = []struct {
	ID   string
	Name string
}{
	{"lacteos-huevos", "Lácteos"},
	{"frutas-verduras", "Frutas y verduras"},
	{"carne-charcuteria", "Carnicería"},
	{"pescado-marisco", "Pescadería"},
	{"pan-cereales-dulces", "Pan y bollería"},
	{"congelados", "Congelados"},
	{"bebidas", "Bebidas"},
	{"aceite-conservas-salsas", "Aceite y conservas"},
	{"drogueria-limpieza", "Droguería"},
	{"higiene-belleza-salud", "Higiene y salud"},
}

type alcampoResponse struct {
	Data struct {
		Products []struct {
			Code        string  `json:"code"`
			Name        string  `json:"name"`
			Brand       string  `json:"manufacturerName"`
			Price       float64 `json:"price"`
			ImageURL    string  `json:"image"`
			URL         string  `json:"url"`
			PackageSize string  `json:"salesUnit"`
		} `json:"products"`
	} `json:"data"`
}

func (s *AlcampoScraper) setHeaders(req *http.Request) {
	ua := s.userAgent
	if ua == "" || ua == "Mozilla/5.0 (compatible; jcrlabs-bot/1.0)" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9")
	req.Header.Set("Referer", "https://www.alcampo.es/")
}

func (s *AlcampoScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	var all []RawProduct
	for _, cat := range alcampoCategories {
		prods, err := s.scrapeCategory(ctx, cat.ID, cat.Name)
		if err != nil {
			slog.Warn("alcampo: skip category", slog.String("id", cat.ID), slog.String("error", err.Error()))
			continue
		}
		all = append(all, prods...)
		select {
		case <-ctx.Done():
			return all, ctx.Err()
		case <-time.After(800 * time.Millisecond):
		}
	}
	return all, nil
}

func (s *AlcampoScraper) scrapeCategory(ctx context.Context, catID, catName string) ([]RawProduct, error) {
	url := fmt.Sprintf("https://www.alcampo.es/api/v2/products?category=%s&page=0&size=48", catID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alcampo category %s: status %d", catID, resp.StatusCode)
	}

	var data alcampoResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("alcampo category %s: decode: %w", catID, err)
	}
	slog.Debug("alcampo: fetched category", slog.String("id", catID), slog.Int("products", len(data.Data.Products)))

	var products []RawProduct
	for _, p := range data.Data.Products {
		unit, qty := ParseUnit(p.PackageSize)
		products = append(products, RawProduct{
			ExternalID:   p.Code,
			Name:         p.Name,
			Brand:        p.Brand,
			Price:        p.Price,
			Unit:         unit,
			UnitQuantity: qty,
			Category:     catName,
			ImageURL:     p.ImageURL,
			ProductURL:   "https://www.alcampo.es" + p.URL,
		})
	}
	return products, nil
}
