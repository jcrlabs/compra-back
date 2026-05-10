package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// flexFloat accepts both JSON number and string for price fields.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = flexFloat(v)
	return nil
}

// MercadonaScraper fetches products from the tienda.mercadona.es JSON REST API.
// No browser required — the API is publicly accessible without authentication.

type MercadonaScraper struct {
	postalCode string
	client     *http.Client
}

func NewMercadonaScraper(postalCode, _ string) *MercadonaScraper {
	return &MercadonaScraper{
		postalCode: postalCode,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *MercadonaScraper) Name() string { return "mercadona" }

// API response types.

type mercadonaCategoriesResp struct {
	Count   int `json:"count"`
	Results []struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Categories []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"categories"`
	} `json:"results"`
}

type mercadonaSubcatResp struct {
	Categories []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Products []struct {
			ID          string `json:"id"`
			Slug        string `json:"slug"`
			DisplayName string `json:"display_name"`
			Thumbnail   string `json:"thumbnail"`
			PriceInstructions struct {
				UnitPrice    flexFloat `json:"unit_price"`
				SizeFormat   string    `json:"size_format"`
				UnitSize     flexFloat `json:"unit_size"`
				PricePerUnit flexFloat `json:"price_per_unit"`
			} `json:"price_instructions"`
			Brand string `json:"brand"`
		} `json:"products"`
	} `json:"categories"`
}

func (s *MercadonaScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	slog.Info("mercadona: starting scrape", slog.String("postal_code", s.postalCode))

	// Step 1: fetch top-level categories.
	catURL := fmt.Sprintf("https://tienda.mercadona.es/api/categories/?postal_code=%s", s.postalCode)
	var catResp mercadonaCategoriesResp
	if err := s.getJSON(ctx, catURL, &catResp); err != nil {
		return nil, fmt.Errorf("mercadona: categories: %w", err)
	}
	slog.Info("mercadona: categories fetched", slog.Int("top_level", catResp.Count))

	var products []RawProduct

	// Step 2: for each top-level category, fetch each subcategory.
	for _, top := range catResp.Results {
		if ctx.Err() != nil {
			break
		}
		for _, sub := range top.Categories {
			if ctx.Err() != nil {
				break
			}

			subcatURL := fmt.Sprintf("https://tienda.mercadona.es/api/categories/%d/?postal_code=%s", sub.ID, s.postalCode)
			var subcatResp mercadonaSubcatResp
			if err := s.getJSON(ctx, subcatURL, &subcatResp); err != nil {
				slog.Warn("mercadona: subcat failed", slog.Int("id", sub.ID), slog.String("error", err.Error()))
				continue
			}

			for _, group := range subcatResp.Categories {
				for _, p := range group.Products {
					pi := p.PriceInstructions
					unit, qty := ParseUnit(pi.SizeFormat)
					if qty == 0 {
						qty = float64(pi.UnitSize)
					}
					if qty == 0 {
						qty = 1
					}

					imgURL := p.Thumbnail

					products = append(products, RawProduct{
						ExternalID:   p.ID,
						Name:         p.DisplayName,
						Brand:        p.Brand,
						Price:        float64(pi.UnitPrice),
						PricePerUnit: float64(pi.PricePerUnit),
						Unit:         unit,
						UnitQuantity: qty,
						Category:     top.Name + " > " + sub.Name,
						ImageURL:     imgURL,
						ProductURL:   fmt.Sprintf("https://tienda.mercadona.es/producto/%s/%s/", p.Slug, p.ID),
					})
				}
			}

			// Brief pause to avoid hammering the API.
			select {
			case <-ctx.Done():
				goto done
			case <-time.After(150 * time.Millisecond):
			}
		}
	}

done:
	slog.Info("mercadona: scrape complete", slog.Int("products", len(products)))
	return products, nil
}

func (s *MercadonaScraper) getJSON(ctx context.Context, url string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}
