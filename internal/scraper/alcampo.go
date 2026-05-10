package scraper

import (
	"context"
	"log/slog"
)

// AlcampoScraper — Alcampo (Auchan España) scraping is currently not possible.
//
// Investigation (2025-05):
//   - www.alcampo.es is unreachable from non-ES IPs (connection timeout / HTTP 000).
//   - The endpoint /api/v2/products?category=... returns HTTP 404 — it does not exist.
//   - Alcampo's online shop (alcampo.es/compra-online) uses a hybris-based frontend
//     that renders products via JS and requires region/store selection.
//   - No public documented API is available.
//
// To make this work:
//  1. Run from a Spanish IP (or proxy).
//  2. Use a headless browser to select a store and capture the XHR requests.
//  3. The likely real API pattern is /search?query=*&category=<hybris-cat-id>&currentPage=0&pageSize=48
//     but the category IDs are internal Hybris IDs, not slugs.

type AlcampoScraper struct {
	userAgent string
}

func NewAlcampoScraper(userAgent string) *AlcampoScraper {
	return &AlcampoScraper{userAgent: userAgent}
}

func (s *AlcampoScraper) Name() string { return "alcampo" }

func (s *AlcampoScraper) Scrape(_ context.Context) ([]RawProduct, error) {
	slog.Warn("alcampo: scraper disabled — site unreachable from non-ES IPs and API endpoint does not exist; requires store selection session")
	return nil, nil
}
