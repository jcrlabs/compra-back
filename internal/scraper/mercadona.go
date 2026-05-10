package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// MercadonaScraper uses go-rod to:
//  1. Navigate to www.mercadona.es and select postal code 28001.
//  2. Intercept the /api/categories/ XHR to get category IDs.
//  3. Fetch /api/categories/{id}/?postal_code=28001 for each category.

type MercadonaScraper struct {
	postalCode string
	userAgent  string
}

func NewMercadonaScraper(postalCode, userAgent string) *MercadonaScraper {
	return &MercadonaScraper{postalCode: postalCode, userAgent: userAgent}
}

func (s *MercadonaScraper) Name() string { return "mercadona" }

type mercadonaCategoriesResp struct {
	Results []struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Categories []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"categories"`
	} `json:"results"`
}

type mercadonaCategoryResp struct {
	Categories []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Products []struct {
			ID          string `json:"id"`
			Slug        string `json:"slug"`
			DisplayName string `json:"display_name"`
			Photos      []struct {
				Regular string `json:"regular"`
			} `json:"photos"`
			PriceInstructions struct {
				UnitPrice    float64 `json:"unit_price"`
				BulkPrice    float64 `json:"bulk_price"`
				SizeFormat   string  `json:"size_format"`
				NetWeight    string  `json:"net_weight"`
				UnitSize     float64 `json:"unit_size"`
				PricePerUnit float64 `json:"price_per_unit"`
			} `json:"price_instructions"`
			Brand    string `json:"brand"`
			Category string `json:"category"`
		} `json:"products"`
	} `json:"categories"`
}

func (s *MercadonaScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	browser := getBrowser()

	// Step 1: navigate to site and select postal code to get session cookies.
	page, err := browser.Page(proto.TargetCreateTarget{URL: "https://www.mercadona.es/"})
	if err != nil {
		return nil, fmt.Errorf("mercadona: open home page: %w", err)
	}
	defer func() { _ = page.Close() }()

	page = page.Context(ctx)

	// Wait for the postal code input to appear and fill it.
	if err := page.WaitLoad(); err != nil {
		slog.Warn("mercadona: WaitLoad failed, continuing", slog.String("error", err.Error()))
	}

	// Try to find and fill the postal code selector modal.
	_ = rod.Try(func() {
		el := page.MustElement(`input[placeholder*="postal"], input[name*="postal"], input[id*="postal"], input[placeholder*="código"]`)
		el.MustInput(s.postalCode)
		el.MustType('\r')
	})

	// Wait a bit for the session to establish.
	time.Sleep(3 * time.Second)

	// Step 2: get categories via API using session cookies from the browser.
	// Use rod's fetch interception approach: navigate to the API URL.
	apiPage, err := browser.Page(proto.TargetCreateTarget{
		URL: fmt.Sprintf("https://www.mercadona.es/api/categories/?postal_code=%s", s.postalCode),
	})
	if err != nil {
		slog.Warn("mercadona: failed to open categories API page", slog.String("error", err.Error()))
		return nil, nil
	}
	defer func() { _ = apiPage.Close() }()

	apiPage = apiPage.Context(ctx)
	if err := apiPage.WaitLoad(); err != nil {
		slog.Warn("mercadona: categories API WaitLoad failed", slog.String("error", err.Error()))
	}

	bodyText, err := apiPage.MustElement("body").Text()
	if err != nil {
		slog.Warn("mercadona: failed to get categories response body", slog.String("error", err.Error()))
		return nil, nil
	}

	var catResp mercadonaCategoriesResp
	if err := json.Unmarshal([]byte(bodyText), &catResp); err != nil {
		slog.Warn("mercadona: failed to parse categories JSON", slog.String("error", err.Error()))
		return nil, nil
	}

	var products []RawProduct

	// Step 3: iterate each top-level category.
	for _, top := range catResp.Results {
		select {
		case <-ctx.Done():
			return products, nil
		default:
		}

		catURL := fmt.Sprintf("https://www.mercadona.es/api/categories/%d/?postal_code=%s", top.ID, s.postalCode)
		catPage, err := browser.Page(proto.TargetCreateTarget{URL: catURL})
		if err != nil {
			slog.Warn("mercadona: failed to open category page", slog.String("id", fmt.Sprint(top.ID)), slog.String("error", err.Error()))
			continue
		}

		catPage = catPage.Context(ctx)
		if err := catPage.WaitLoad(); err != nil {
			_ = catPage.Close()
			continue
		}

		body, err := catPage.MustElement("body").Text()
		_ = catPage.Close()
		if err != nil {
			continue
		}

		var detail mercadonaCategoryResp
		if err := json.Unmarshal([]byte(body), &detail); err != nil {
			slog.Warn("mercadona: failed to parse category detail", slog.String("name", top.Name), slog.String("error", err.Error()))
			continue
		}

		for _, sub := range detail.Categories {
			for _, p := range sub.Products {
				unit, qty := ParseUnit(p.PriceInstructions.SizeFormat)
				if qty == 0 {
					qty = p.PriceInstructions.UnitSize
				}

				imgURL := ""
				if len(p.Photos) > 0 {
					imgURL = p.Photos[0].Regular
				}

				products = append(products, RawProduct{
					ExternalID:   p.ID,
					Name:         p.DisplayName,
					Brand:        p.Brand,
					Price:        p.PriceInstructions.UnitPrice,
					PricePerUnit: p.PriceInstructions.PricePerUnit,
					Unit:         unit,
					UnitQuantity: qty,
					Category:     top.Name + " > " + sub.Name,
					ImageURL:     imgURL,
					ProductURL:   fmt.Sprintf("https://www.mercadona.es/producto/%s/%s/", p.Slug, p.ID),
				})
			}
		}
	}

	slog.Info("mercadona: scrape complete", slog.Int("products", len(products)))
	return products, nil
}
