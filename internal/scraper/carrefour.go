package scraper

import (
	"context"
	"log/slog"
)

// CarrefourScraper is disabled — carrefour.es is protected by Cloudflare (HTTP 403).

type CarrefourScraper struct{}

func NewCarrefourScraper(_ string) *CarrefourScraper { return &CarrefourScraper{} }

func (s *CarrefourScraper) Name() string { return "carrefour" }

func (s *CarrefourScraper) Scrape(_ context.Context) ([]RawProduct, error) {
	slog.Warn("carrefour: scraper disabled — Cloudflare protection blocks scraping")
	return nil, nil
}
