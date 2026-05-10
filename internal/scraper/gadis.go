package scraper

import (
	"context"
	"log/slog"
)

// GadisScraper — Gadis online shop scraping is currently not possible without JS rendering.
//
// Investigation (2025-05):
//   - www.gadis.es/tienda-online/* is a pure React SPA (single-page application).
//   - All URLs (including category paths) return the same ~9 KB HTML shell with no product data.
//   - The HTML has no CSS classes matching .producto, .product, .item-product, or any
//     product-related selectors — the shell contains only the React mount point and script tags.
//   - Products are loaded via XHR after the React bundle executes in the browser.
//   - The JS bundle URL pattern is /static/js/main.<hash>.js but is minified.
//   - Attempts to call /api/products?category=... return the same SPA shell HTML (HTTP 200).
//
// To make this work: use a headless browser (Playwright/Rod) to:
//  1. Load https://www.gadis.es/tienda-online/lacteos-quesos-huevos/
//  2. Wait for the product grid to appear (e.g. wait for a product card selector).
//  3. Scrape the rendered DOM or intercept the underlying XHR API call.

type GadisScraper struct {
	userAgent string
}

func NewGadisScraper(userAgent string) *GadisScraper {
	return &GadisScraper{userAgent: userAgent}
}

func (s *GadisScraper) Name() string { return "gadis" }

func (s *GadisScraper) Scrape(_ context.Context) ([]RawProduct, error) {
	slog.Warn("gadis: scraper disabled — site is a React SPA; no server-rendered HTML or public API available; requires headless browser")
	return nil, nil
}
