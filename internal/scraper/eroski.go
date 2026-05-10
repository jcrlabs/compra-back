package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

type EroskiScraper struct {
	userAgent string
}

func NewEroskiScraper(userAgent string) *EroskiScraper {
	return &EroskiScraper{userAgent: userAgent}
}

func (s *EroskiScraper) Name() string { return "eroski" }

var eroskiCategoryURLs = []struct {
	URL  string
	Name string
}{
	{"https://supermercado.eroski.es/es/alimentacion/lacteos-huevos-mantequilla/", "Lácteos"},
	{"https://supermercado.eroski.es/es/alimentacion/frutas-verduras/", "Frutas y verduras"},
	{"https://supermercado.eroski.es/es/alimentacion/carne-charcuteria/", "Carnicería"},
	{"https://supermercado.eroski.es/es/alimentacion/pescado-mariscos/", "Pescadería"},
	{"https://supermercado.eroski.es/es/alimentacion/panaderia-pasteleria/", "Pan y bollería"},
	{"https://supermercado.eroski.es/es/alimentacion/congelados/", "Congelados"},
	{"https://supermercado.eroski.es/es/alimentacion/bebidas/", "Bebidas"},
	{"https://supermercado.eroski.es/es/alimentacion/aceite-conservas-legumbres/", "Aceite y conservas"},
	{"https://supermercado.eroski.es/es/drogueria-hogar/", "Droguería"},
	{"https://supermercado.eroski.es/es/cuidado-personal/", "Higiene y salud"},
}

func (s *EroskiScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	var products []RawProduct

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
		DomainGlob:  "*eroski.es*",
		Delay:       700 * time.Millisecond,
		RandomDelay: 300 * time.Millisecond,
	})
	c.OnResponse(func(r *colly.Response) {
		if r.StatusCode != 200 {
			slog.Warn("eroski: unexpected status", slog.String("url", r.Request.URL.String()), slog.Int("status", r.StatusCode))
		}
	})
	c.OnError(func(r *colly.Response, err error) {
		slog.Warn("eroski: request error", slog.String("url", r.Request.URL.String()), slog.String("error", err.Error()))
	})

	var currentCat string

	// Eroski product cards
	c.OnHTML("li.product, .grid-product, article.product", func(e *colly.HTMLElement) {
		name := strings.TrimSpace(e.ChildText(".product-name, h3, .name"))
		priceStr := strings.TrimSpace(e.ChildText(".price, .precio, .product-price"))
		imgURL := e.ChildAttr("img", "src")
		productURL := e.ChildAttr("a", "href")

		if name == "" {
			return
		}

		priceStr = strings.ReplaceAll(priceStr, "€", "")
		priceStr = strings.ReplaceAll(priceStr, ",", ".")
		priceStr = strings.TrimSpace(priceStr)

		var price float64
		fmt.Sscanf(priceStr, "%f", &price)
		if price == 0 {
			return
		}

		unit, qty := ParseUnit(name)
		if !strings.HasPrefix(productURL, "http") {
			productURL = "https://supermercado.eroski.es" + productURL
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

	for _, cat := range eroskiCategoryURLs {
		select {
		case <-ctx.Done():
			return products, ctx.Err()
		default:
		}
		currentCat = cat.Name
		_ = c.Visit(cat.URL)
	}
	return products, nil
}
