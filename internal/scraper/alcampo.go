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

// AlcampoScraper uses the compraonline.alcampo.es REST API (webproductpagews v6).
// No browser required — categories are fetched via paginated JSON API.
//
// API: GET https://www.compraonline.alcampo.es/api/webproductpagews/v6/product-pages
//   ?category={uuid}&tag=web&tag=category-item&maxPageSize=100&maxProductsToDecorate=100&imageFormats=medium

type AlcampoScraper struct {
	userAgent string
	client    *http.Client
}

func NewAlcampoScraper(userAgent string) *AlcampoScraper {
	return &AlcampoScraper{
		userAgent: userAgent,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *AlcampoScraper) Name() string { return "alcampo" }

// alcampoCategories: top-level food categories from compraonline.alcampo.es initial state (2026-05).
var alcampoFoodCategories = []struct {
	Name string
	UUID string
}{
	{"Frescos", "fe16f111-2da2-4c24-9bad-56b10b28b87a"},
	{"Leche, Huevos y Lácteos", "e33fcb5b-8a16-4202-b88b-13d7dfc547ce"},
	{"Alimentación", "dc4ac5ed-f53e-4f6d-b2fd-8de2aa8e723c"},
	{"Bebidas", "d01a3417-5a84-4135-8d0c-a368cfd581a8"},
	{"Congelados", "ab82727b-0a66-4fac-bff6-d9864ecd9dbc"},
	{"Desayuno y Merienda", "d611a12e-a365-4ec5-9f0f-d2baf03e1f69"},
	{"Comida Preparada", "78da9a99-800a-4cbe-91c7-d77378efc4b8"},
}

const alcampoAPIBase = "https://www.compraonline.alcampo.es"

type alcampoAPIResponse struct {
	ProductGroups []struct {
		DecoratedProducts []alcampoProduct `json:"decoratedProducts"`
	} `json:"productGroups"`
	Metadata struct {
		NextPageToken string `json:"nextPageToken"`
	} `json:"metadata"`
}

type alcampoProduct struct {
	ProductID  string `json:"productId"`
	RetailerID string `json:"retailerProductId"`
	Name       string `json:"name"`
	PackSize   string `json:"packSizeDescription"`
	Available  bool   `json:"available"`
	Price      struct {
		Amount string `json:"amount"`
	} `json:"price"`
	Image struct {
		Src string `json:"src"`
	} `json:"image"`
	CategoryPath []string `json:"categoryPath"`
}

func (s *AlcampoScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	var products []RawProduct

	for _, cat := range alcampoFoodCategories {
		select {
		case <-ctx.Done():
			return products, nil
		default:
		}

		catProducts, err := s.scrapeCategory(ctx, cat.UUID, cat.Name)
		if err != nil {
			slog.Warn("alcampo: category failed", slog.String("cat", cat.Name), slog.String("err", err.Error()))
			continue
		}
		products = append(products, catProducts...)
		slog.Info("alcampo: category scraped", slog.String("cat", cat.Name), slog.Int("count", len(catProducts)))
	}

	if len(products) == 0 {
		slog.Warn("alcampo: no products extracted")
	} else {
		slog.Info("alcampo: scrape complete", slog.Int("products", len(products)))
	}
	return products, nil
}

func (s *AlcampoScraper) scrapeCategory(ctx context.Context, uuid, catName string) ([]RawProduct, error) {
	var products []RawProduct
	pageToken := ""
	maxPages := 20

	for page := 0; page < maxPages; page++ {
		select {
		case <-ctx.Done():
			return products, nil
		default:
		}

		url := fmt.Sprintf(
			"%s/api/webproductpagews/v6/product-pages?category=%s&tag=web&tag=category-item&maxPageSize=100&maxProductsToDecorate=100&imageFormats=medium",
			alcampoAPIBase, uuid,
		)
		if pageToken != "" {
			url += "&pageToken=" + pageToken
		}

		resp, err := s.doRequest(ctx, url)
		if err != nil {
			return products, fmt.Errorf("page %d: %w", page, err)
		}

		for _, g := range resp.ProductGroups {
			for _, p := range g.DecoratedProducts {
				if !p.Available || p.Name == "" {
					continue
				}
				raw := s.toRawProduct(p, catName)
				if raw.Price > 0 {
					products = append(products, raw)
				}
			}
		}

		pageToken = resp.Metadata.NextPageToken
		if pageToken == "" {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	return products, nil
}

func (s *AlcampoScraper) doRequest(ctx context.Context, url string) (*alcampoAPIResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Referer", alcampoAPIBase+"/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result alcampoAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}
	return &result, nil
}

func (s *AlcampoScraper) toRawProduct(p alcampoProduct, catName string) RawProduct {
	price, _ := strconv.ParseFloat(p.Price.Amount, 64)

	unit, qty := ParseUnit(p.PackSize)
	if unit == "" {
		unit = "unidad"
		qty = 1
	}

	// Use the deepest available category from the product's path
	category := catName
	if len(p.CategoryPath) > 0 {
		category = p.CategoryPath[0]
	}

	productURL := alcampoAPIBase + "/p/" + p.ProductID

	return RawProduct{
		Name:         strings.TrimSpace(p.Name),
		Price:        price,
		Unit:         unit,
		UnitQuantity: qty,
		ImageURL:     p.Image.Src,
		ProductURL:   productURL,
		Category:     category,
	}
}
