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

// FroizScraper fetches products from the Froiz online store REST API.
// API base: https://servicios.froiz.com/api

const (
	froizCategoriesURL = "https://servicios.froiz.com/api/categories?channel.id=1"
	froizFamilyURL     = "https://servicios.froiz.com/api/products/families/%d"
	froizImageBase     = "https://imagedelivery.net/laxGYDNZyT04iZVpzPzryw/"
	froizProductBase   = "https://supermercado.froiz.com/"
)

type FroizScraper struct {
	client *http.Client
}

func NewFroizScraper(_ string) *FroizScraper {
	return &FroizScraper{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *FroizScraper) Name() string { return "froiz" }

type froizCategory struct {
	Name     string         `json:"name"`
	ID       int            `json:"id"`
	Sections []froizSection `json:"sections"`
}
type froizSection struct {
	Name     string        `json:"name"`
	Families []froizFamily `json:"families"`
}
type froizFamily struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
	Slug string `json:"slug"`
}
type froizProduct struct {
	ID                   int     `json:"id"`
	Name                 string  `json:"name"`
	Enabled              bool    `json:"enabled"`
	BrandName            string  `json:"brand_name"`
	FamilyName           string  `json:"family_name"`
	FamilySlug           string  `json:"family_slug"`
	OrderPrice           float64 `json:"order_price"`
	MeasurementUnit      string  `json:"measurement_unit"`
	MeasurementUnitRatio string  `json:"measurement_unit_ratio"`
	Slug                 string  `json:"slug"`
	ImageID              string  `json:"image_id"`
}

func (s *FroizScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	slog.Info("froiz: starting scrape")

	categories, err := s.fetchCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("froiz: fetch categories: %w", err)
	}

	seen := make(map[int]bool)
	var products []RawProduct

	for _, cat := range categories {
		for _, section := range cat.Sections {
			for _, family := range section.Families {
				if ctx.Err() != nil {
					goto done
				}
				if seen[family.ID] {
					continue
				}
				seen[family.ID] = true

				fps, err := s.fetchFamily(ctx, family.ID, family.Name)
				if err != nil {
					slog.Warn("froiz: family failed", slog.Int("id", family.ID), slog.String("error", err.Error()))
					continue
				}
				products = append(products, fps...)
				slog.Info("froiz: family scraped", slog.String("family", family.Name), slog.Int("count", len(fps)))

				select {
				case <-ctx.Done():
					goto done
				case <-time.After(150 * time.Millisecond):
				}
			}
		}
	}

done:
	slog.Info("froiz: scrape complete", slog.Int("products", len(products)))
	return products, nil
}

func (s *FroizScraper) fetchCategories(ctx context.Context) ([]froizCategory, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, froizCategoriesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var cats []froizCategory
	if err := json.NewDecoder(resp.Body).Decode(&cats); err != nil {
		return nil, err
	}
	return cats, nil
}

func (s *FroizScraper) fetchFamily(ctx context.Context, familyID int, catName string) ([]RawProduct, error) {
	url := fmt.Sprintf(froizFamilyURL, familyID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var fps []froizProduct
	if err := json.Unmarshal(body, &fps); err != nil {
		return nil, err
	}

	var products []RawProduct
	for _, fp := range fps {
		if !fp.Enabled || fp.Name == "" || fp.OrderPrice == 0 {
			continue
		}
		p := RawProduct{
			ExternalID: strconv.Itoa(fp.ID),
			Name:       fp.Name,
			Brand:      fp.BrandName,
			Price:      fp.OrderPrice,
			Category:   catName,
		}
		if fp.FamilyName != "" {
			p.Category = fp.FamilyName
		}
		p.Unit, p.UnitQuantity = parseFroizUnit(fp.MeasurementUnit, fp.MeasurementUnitRatio)
		if fp.ImageID != "" {
			p.ImageURL = froizImageBase + fp.ImageID + "/public"
		}
		if fp.FamilySlug != "" && fp.Slug != "" {
			p.ProductURL = froizProductBase + fp.FamilySlug + "/" + fp.Slug
		}
		products = append(products, p)
	}
	return products, nil
}

func parseFroizUnit(unit, ratioStr string) (string, float64) {
	ratio, _ := strconv.ParseFloat(strings.TrimSpace(ratioStr), 64)
	if ratio <= 0 {
		ratio = 1
	}
	u := strings.ToLower(strings.TrimSpace(unit))
	switch {
	case strings.Contains(u, "litro"):
		return "l", ratio
	case strings.Contains(u, "kilogramo"):
		return "kg", ratio
	case strings.Contains(u, "gramo"):
		return "kg", ratio / 1000
	case strings.Contains(u, "mililitro"):
		return "l", ratio / 1000
	default:
		return "unidad", ratio
	}
}
