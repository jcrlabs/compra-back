package scraper

import (
	"context"
	"log/slog"
)

// FroizScraper is disabled — froiz.com online shop appears to be discontinued (404).

type FroizScraper struct{}

func NewFroizScraper(_ string) *FroizScraper { return &FroizScraper{} }

func (s *FroizScraper) Name() string { return "froiz" }

func (s *FroizScraper) Scrape(_ context.Context) ([]RawProduct, error) {
	slog.Warn("froiz: scraper disabled — online shop discontinued")
	return nil, nil
}
