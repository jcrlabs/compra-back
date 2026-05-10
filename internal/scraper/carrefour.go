package scraper

import (
	"context"
	"log/slog"
)

// CarrefourScraper — Carrefour España scraping is currently not possible without JS rendering.
//
// Investigation (2025-05):
//   - The endpoint /api/search?category=...&page_size=48 does not exist (403/503).
//   - www.carrefour.es/supermercado/ returns ~1 KB of HTML with no product data — the
//     catalogue is rendered entirely by a React SPA loaded via JS bundles.
//   - Real category URLs follow the pattern:
//     /supermercado/lacteos-huevos-y-margarina/cat20138/
//     but all product data is fetched client-side via XHR after JS execution.
//   - The internal API (inferred from network tab) uses endpoints like:
//     /api/cms/v3/products?lang=es&query=*&rows=48&category=cat20138
//     but these require session tokens injected by the SPA bootstrap.
//
// To make this work: use a headless browser (Playwright/Rod) to load a category page
// and intercept the XHR response from the /api/cms/v3/products endpoint, or use
// Carrefour's affiliate/open API if/when they publish one.

type CarrefourScraper struct {
	userAgent string
}

func NewCarrefourScraper(userAgent string) *CarrefourScraper {
	return &CarrefourScraper{userAgent: userAgent}
}

func (s *CarrefourScraper) Name() string { return "carrefour" }

func (s *CarrefourScraper) Scrape(_ context.Context) ([]RawProduct, error) {
	slog.Warn("carrefour: scraper disabled — catalogue is JS-rendered SPA with no public API; requires headless browser")
	return nil, nil
}
