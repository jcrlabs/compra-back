package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

// FroizScraper scrapes froiz.com using colly + goquery.
// Froiz serves mostly static HTML — no JS rendering needed.

type FroizScraper struct {
	userAgent string
}

func NewFroizScraper(userAgent string) *FroizScraper {
	return &FroizScraper{userAgent: userAgent}
}

func (s *FroizScraper) Name() string { return "froiz" }

var froizCategoryURLs = []struct {
	URL  string
	Name string
}{
	{"https://www.froiz.com/supermercado/lacteos/", "Lácteos"},
	{"https://www.froiz.com/supermercado/frutas-verduras/", "Frutas y verduras"},
	{"https://www.froiz.com/supermercado/carne-aves/", "Carnicería"},
	{"https://www.froiz.com/supermercado/pescado-marisco/", "Pescadería"},
	{"https://www.froiz.com/supermercado/panaderia-bolleria/", "Pan y bollería"},
	{"https://www.froiz.com/supermercado/congelados/", "Congelados"},
	{"https://www.froiz.com/supermercado/bebidas/", "Bebidas"},
	{"https://www.froiz.com/supermercado/aceites-conservas/", "Aceite y conservas"},
	{"https://www.froiz.com/supermercado/drogueria/", "Droguería"},
	{"https://www.froiz.com/supermercado/higiene/", "Higiene y salud"},
}

func (s *FroizScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	var products []RawProduct
	var scraperErr error

	ua := s.userAgent
	if ua == "" || ua == "Mozilla/5.0 (compatible; jcrlabs-bot/1.0)" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	}
	c := colly.NewCollector(
		colly.UserAgent(ua),
		colly.MaxDepth(1),
	)
	c.SetRequestTimeout(30 * time.Second)
	_ = c.Limit(&colly.LimitRule{
		DomainGlob:  "*froiz.com*",
		Delay:       600 * time.Millisecond,
		RandomDelay: 300 * time.Millisecond,
	})
	c.OnResponse(func(r *colly.Response) {
		if r.StatusCode != 200 {
			slog.Warn("froiz: unexpected status", slog.String("url", r.Request.URL.String()), slog.Int("status", r.StatusCode))
		}
	})

	var currentCat string

	c.OnHTML(".product-item, .product-card, article.product", func(e *colly.HTMLElement) {
		name := strings.TrimSpace(e.ChildText(".product-name, .product-title, h3, h2"))
		priceStr := strings.TrimSpace(e.ChildText(".price, .product-price, .precio"))
		imgURL := e.ChildAttr("img", "src")
		productURL := e.ChildAttr("a", "href")

		if name == "" {
			return
		}

		var price float64
		priceStr = strings.ReplaceAll(priceStr, "€", "")
		priceStr = strings.ReplaceAll(priceStr, ",", ".")
		priceStr = strings.TrimSpace(priceStr)
		fmt.Sscanf(priceStr, "%f", &price)

		if price == 0 {
			return
		}

		// Try to extract unit from name (e.g. "Leche 1L")
		unit, qty := extractUnitFromName(name)

		if !strings.HasPrefix(productURL, "http") {
			productURL = "https://www.froiz.com" + productURL
		}

		products = append(products, RawProduct{
			Name:         name,
			Price:        price,
			Unit:         unit,
			UnitQuantity: qty,
			Category:     currentCat,
			ImageURL:     imgURL,
			ProductURL:   productURL,
		})
	})

	c.OnError(func(r *colly.Response, err error) {
		slog.Warn("froiz: request error", slog.String("url", r.Request.URL.String()), slog.String("error", err.Error()))
		scraperErr = err
	})

	for _, cat := range froizCategoryURLs {
		select {
		case <-ctx.Done():
			return products, ctx.Err()
		default:
		}
		currentCat = cat.Name
		_ = c.Visit(cat.URL)
	}

	_ = scraperErr // non-fatal — return what we have
	return products, nil
}

// extractUnitFromName tries to find unit info in the product name.
func extractUnitFromName(name string) (unit string, qty float64) {
	lower := strings.ToLower(name)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<p>" + lower + "</p>"))
	if err != nil {
		return ParseUnit(lower)
	}
	text := doc.Find("p").Text()
	return ParseUnit(text)
}
