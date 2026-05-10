package scraper

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jcrlabs/compra-back/internal/models"
	"github.com/jcrlabs/compra-back/internal/repository"
	"github.com/robfig/cron/v3"
)

// Ingester persists scraped products and prices.
type Ingester struct {
	catalog *repository.CatalogRepository
	pricing *repository.PricingRepository
}

func NewIngester(catalog *repository.CatalogRepository, pricing *repository.PricingRepository) *Ingester {
	return &Ingester{catalog: catalog, pricing: pricing}
}

func (ing *Ingester) Ingest(super models.Supermarket, products []RawProduct) {
	saved := 0
	for _, raw := range products {
		productID, err := ing.resolveProduct(raw)
		if err != nil {
			slog.Warn("scraper: skip product", slog.String("name", raw.Name), slog.String("error", err.Error()))
			continue
		}

		price := &models.Price{
			ID:          uuid.New(),
			ProductID:   productID,
			Supermarket: super,
			Price:       raw.Price,
			ExternalID:  raw.ExternalID,
			ExternalURL: raw.ProductURL,
		}
		if raw.PricePerUnit > 0 {
			price.PricePerUnit = &raw.PricePerUnit
		}

		if err := ing.pricing.InsertPrice(price); err != nil {
			slog.Warn("scraper: insert price", slog.String("error", err.Error()))
			continue
		}
		saved++
	}
	slog.Info("scraper: ingested", slog.String("super", string(super)), slog.Int("saved", saved), slog.Int("total", len(products)))
}

func (ing *Ingester) resolveProduct(raw RawProduct) (uuid.UUID, error) {
	// Try exact match first
	existing, err := ing.catalog.FindProductByNameBrandUnit(raw.Name, raw.Brand, raw.Unit, raw.UnitQuantity)
	if err == nil {
		return existing.ID, nil
	}

	// Try normalised match
	existing, err = ing.catalog.FindProductByNameBrandUnit(
		Normalise(raw.Name), Normalise(raw.Brand), raw.Unit, raw.UnitQuantity,
	)
	if err == nil {
		return existing.ID, nil
	}

	// Create new product
	p := &models.Product{
		ID:           uuid.New(),
		Name:         raw.Name,
		Brand:        raw.Brand,
		Unit:         raw.Unit,
		UnitQuantity: raw.UnitQuantity,
		ImageURL:     raw.ImageURL,
	}
	if err := ing.catalog.UpsertProduct(p); err != nil {
		return uuid.Nil, err
	}
	return p.ID, nil
}

// Scheduler wraps cron and runs all scrapers on a schedule.
type Scheduler struct {
	cron     *cron.Cron
	scrapers []Scraper
	ingester *Ingester
}

func NewScheduler(ingester *Ingester, scrapers ...Scraper) *Scheduler {
	return &Scheduler{
		cron:     cron.New(),
		scrapers: scrapers,
		ingester: ingester,
	}
}

func (s *Scheduler) Start(cronExpr string) {
	_, err := s.cron.AddFunc(cronExpr, func() {
		s.RunAll()
	})
	if err != nil {
		slog.Error("scheduler: invalid cron expr", slog.String("expr", cronExpr), slog.String("error", err.Error()))
		return
	}
	s.cron.Start()
	slog.Info("scraper scheduler started", slog.String("cron", cronExpr))
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) RunAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	for _, sc := range s.scrapers {
		slog.Info("scraper: starting", slog.String("name", sc.Name()))
		products, err := sc.Scrape(ctx)
		LogScraperResult(sc.Name(), len(products), err)
		if err == nil && len(products) > 0 {
			s.ingester.Ingest(models.Supermarket(sc.Name()), products)
		}
		select {
		case <-ctx.Done():
			slog.Warn("scraper: context cancelled, stopping")
			return
		case <-time.After(2 * time.Minute): // pause between scrapers
		}
	}
}

// RunOne triggers a single scraper by name (used by admin API).
func (s *Scheduler) RunOne(ctx context.Context, name string) error {
	for _, sc := range s.scrapers {
		if sc.Name() == name {
			products, err := sc.Scrape(ctx)
			LogScraperResult(sc.Name(), len(products), err)
			if err == nil && len(products) > 0 {
				s.ingester.Ingest(models.Supermarket(name), products)
			}
			return err
		}
	}
	return nil
}
