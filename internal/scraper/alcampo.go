package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// AlcampoScraper uses go-rod to navigate Alcampo's hybris-based SPA.
// Requires a Spanish IP to reach www.alcampo.es.

type AlcampoScraper struct {
	userAgent string
}

func NewAlcampoScraper(userAgent string) *AlcampoScraper {
	return &AlcampoScraper{userAgent: userAgent}
}

func (s *AlcampoScraper) Name() string { return "alcampo" }

var alcampoCategories = []struct {
	Name string
	URL  string
}{
	{"Lácteos y huevos", "https://www.alcampo.es/compra-online/alimentacion/lacteos-y-huevos/"},
	{"Charcutería y quesos", "https://www.alcampo.es/compra-online/alimentacion/charcuteria-y-quesos/"},
	{"Frutas y verduras", "https://www.alcampo.es/compra-online/alimentacion/frutas-y-verduras/"},
	{"Carnes y aves", "https://www.alcampo.es/compra-online/alimentacion/carnes-y-aves/"},
	{"Pescado y marisco", "https://www.alcampo.es/compra-online/alimentacion/pescado-y-marisco/"},
	{"Pan y bollería", "https://www.alcampo.es/compra-online/alimentacion/panaderia/"},
	{"Aceite y condimentos", "https://www.alcampo.es/compra-online/alimentacion/aceite-y-condimentos/"},
	{"Pasta y arroz", "https://www.alcampo.es/compra-online/alimentacion/pasta-arroz-y-legumbres/"},
	{"Bebidas", "https://www.alcampo.es/compra-online/alimentacion/bebidas/"},
	{"Congelados", "https://www.alcampo.es/compra-online/alimentacion/congelados/"},
}

func (s *AlcampoScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	browser := getBrowser()
	var products []RawProduct

	for _, cat := range alcampoCategories {
		select {
		case <-ctx.Done():
			return products, nil
		default:
		}

		catProducts, err := scrapeAlcampoCategory(ctx, browser, cat.Name, cat.URL)
		if err != nil {
			slog.Warn("alcampo: category failed", slog.String("cat", cat.Name), slog.String("error", err.Error()))
			continue
		}
		products = append(products, catProducts...)
		slog.Info("alcampo: category scraped", slog.String("cat", cat.Name), slog.Int("count", len(catProducts)))
	}

	if len(products) == 0 {
		slog.Warn("alcampo: no products found — site may be unreachable from non-ES IPs")
	} else {
		slog.Info("alcampo: scrape complete", slog.Int("products", len(products)))
	}
	return products, nil
}

func scrapeAlcampoCategory(ctx context.Context, browser *rod.Browser, catName, url string) ([]RawProduct, error) {
	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, fmt.Errorf("open page: %w", err)
	}
	defer func() { _ = page.Close() }()

	page = page.Context(ctx)

	if err := page.WaitLoad(); err != nil {
		slog.Warn("alcampo: WaitLoad failed", slog.String("url", url))
	}
	time.Sleep(3 * time.Second)

	selectors := []string{
		".product-item",
		".product-card",
		"[data-component-id='product']",
		"li.product",
		"article.product",
	}

	var els rod.Elements
	for _, sel := range selectors {
		els, err = page.Elements(sel)
		if err == nil && len(els) > 0 {
			break
		}
	}

	if len(els) == 0 {
		return nil, fmt.Errorf("no product elements on %s", url)
	}

	var products []RawProduct
	for _, el := range els {
		p := extractAlcampoProduct(el, catName)
		if p.Name == "" {
			continue
		}
		products = append(products, p)
	}
	return products, nil
}

func extractAlcampoProduct(el *rod.Element, catName string) RawProduct {
	var p RawProduct
	p.Category = catName

	nameSelectors := []string{".product-item__name", ".product-name", "h3", "h2", ".name"}
	for _, sel := range nameSelectors {
		if nameEl, err := el.Element(sel); err == nil {
			if t, err := nameEl.Text(); err == nil && t != "" {
				p.Name = strings.TrimSpace(t)
				break
			}
		}
	}

	priceSelectors := []string{".product-item__price", ".price", ".product-price", "[class*='price']"}
	for _, sel := range priceSelectors {
		if priceEl, err := el.Element(sel); err == nil {
			if t, err := priceEl.Text(); err == nil && t != "" {
				p.Price = parseAlcampoPrice(t)
				break
			}
		}
	}

	if imgEl, err := el.Element("img"); err == nil {
		if src, err := imgEl.Attribute("src"); err == nil && src != nil {
			p.ImageURL = *src
		}
	}

	if aEl, err := el.Element("a"); err == nil {
		if href, err := aEl.Attribute("href"); err == nil && href != nil {
			if strings.HasPrefix(*href, "http") {
				p.ProductURL = *href
			} else {
				p.ProductURL = "https://www.alcampo.es" + *href
			}
		}
	}

	p.Unit = "unidad"
	p.UnitQuantity = 1

	return p
}

func parseAlcampoPrice(s string) float64 {
	s = strings.ReplaceAll(s, "€", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	if len(fields) > 0 {
		s = fields[0]
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
