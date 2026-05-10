package scraper

import (
	"context"
	"log/slog"
)

// FroizScraper — Froiz scraping is currently not possible.
//
// Investigation (2025-05):
//   - The old URL pattern /supermercado/*/ returns HTTP 404 — those paths no longer exist.
//   - froiz.com has no robots.txt or sitemap.xml (both return 404).
//   - The homepage returns no links matching grocery category paths.
//   - Alternate URL patterns tried: /categoria/*, /tienda/*, /products?cat=*, /shop/*, /catalogo/*
//     — all return 404 or are unreachable.
//   - froiz.com appears to be an informational/institutional site only.
//     Their online shopping may have moved to a third-party platform or been discontinued.
//
// To make this work:
//  1. Navigate froiz.com manually in a browser to find current category URLs.
//  2. Check if they use a third-party platform (e.g. Instacart, ulabox, etc.).
//  3. Update froizCategoryURLs with working paths and re-enable colly scraping.

type FroizScraper struct {
	userAgent string
}

func NewFroizScraper(userAgent string) *FroizScraper {
	return &FroizScraper{userAgent: userAgent}
}

func (s *FroizScraper) Name() string { return "froiz" }

func (s *FroizScraper) Scrape(_ context.Context) ([]RawProduct, error) {
	slog.Warn("froiz: scraper disabled — old /supermercado/* category URLs return 404; online shop may have been discontinued or moved to a third-party platform")
	return nil, nil
}
