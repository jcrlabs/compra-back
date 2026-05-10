package scraper

import (
	"context"
	"log/slog"
)

// GadisScraper is disabled — gadis.es/tienda-online/* has been discontinued.
//
// Investigation (2026-05):
//   - www.gadis.es is a React SPA (main.e000a98bc69b8efe.js, 3.2 MB).
//   - The React Router has NO routes for /tienda-online/* at all.
//   - Both "/" and "/tienda-online/" serve the same identical HTML shell (md5 match).
//   - The React app has only corporate routes: /folleto, /centros, /noticias, /oportunigadis…
//   - The PHP API at gadisa.es/api has no product endpoints (returns 501 for all product paths).
//   - No alternative Gadis e-commerce domain found (shop.gadis.es, tienda.gadis.es, etc. — all NXDOMAIN).
//   - Conclusion: Gadis has discontinued its online store.

type GadisScraper struct{}

func NewGadisScraper(_ string) *GadisScraper { return &GadisScraper{} }

func (s *GadisScraper) Name() string { return "gadis" }

func (s *GadisScraper) Scrape(_ context.Context) ([]RawProduct, error) {
	slog.Warn("gadis: scraper disabled — tienda-online discontinued, no product API available")
	return nil, nil
}
