package scraper

import (
	"context"
	"log/slog"
)

// MercadonaScraper — Mercadona scraping is currently not possible without a session.
//
// Investigation (2025-05):
//   - The old API at tiendas.mercadona.es is NXDOMAIN globally (domain no longer exists).
//   - The new site www.mercadona.es/api/categories/?postal_code=28001 returns HTTP 302
//     and requires the user to have selected a store/postal code in a browser session first
//     (sets cookies via JS). There is no publicly accessible JSON API without that session.
//   - Mercadona actively blocks programmatic access. No public API is documented.
//
// To make this work: use a headless browser (Playwright/Rod) to:
//  1. Load www.mercadona.es, enter postal code 28001 to select a store.
//  2. Follow the redirect to the store-specific URL (e.g. /inicio/es/).
//  3. Hit /api/categories/ with the session cookies to get category IDs.
//  4. Hit /api/categories/{id}/?postal_code=28001 for each category.

type MercadonaScraper struct {
	postalCode string
	userAgent  string
}

func NewMercadonaScraper(postalCode, userAgent string) *MercadonaScraper {
	return &MercadonaScraper{postalCode: postalCode, userAgent: userAgent}
}

func (s *MercadonaScraper) Name() string { return "mercadona" }

func (s *MercadonaScraper) Scrape(_ context.Context) ([]RawProduct, error) {
	slog.Warn("mercadona: scraper disabled — API requires browser session with store selection; no public API available")
	return nil, nil
}
