package scraper

import (
	"context"
	"log/slog"
)

// EroskiScraper — Eroski online supermarket scraping is currently not possible without a session.
//
// Investigation (2025-05):
//   - supermercado.eroski.es returns HTTP 302 for all category URLs.
//   - The redirect chain requires session cookies set by JS (store selection, CSRF tokens).
//   - Even after following redirects (curl -L), the final response is a JS-rendered SPA shell.
//   - No public JSON API is exposed without authentication.
//   - supermercado.eroski.es/es/alimentacion/lacteos-huevos-mantequilla/ → 302 → login/store flow.
//
// To make this work: use a headless browser (Playwright/Rod) to:
//  1. Load https://supermercado.eroski.es/ and complete the store/postal-code selection.
//  2. Session cookies (JSESSIONID, eroski_store, etc.) are then set.
//  3. Navigate to category pages and scrape rendered product cards, or intercept
//     the underlying XHR product listing API (likely /api/search or SAP Hybris OCC API).

type EroskiScraper struct {
	userAgent string
}

func NewEroskiScraper(userAgent string) *EroskiScraper {
	return &EroskiScraper{userAgent: userAgent}
}

func (s *EroskiScraper) Name() string { return "eroski" }

func (s *EroskiScraper) Scrape(_ context.Context) ([]RawProduct, error) {
	slog.Warn("eroski: scraper disabled — site requires browser session with store selection (302 redirect chain); no public API available")
	return nil, nil
}
