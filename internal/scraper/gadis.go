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

// GadisScraper uses go-rod to load Gadis's React SPA category pages,
// wait for the product grid to render, then extract product data from the DOM.

type GadisScraper struct {
	userAgent string
}

func NewGadisScraper(userAgent string) *GadisScraper {
	return &GadisScraper{userAgent: userAgent}
}

func (s *GadisScraper) Name() string { return "gadis" }

var gadisCategories = []struct {
	Name string
	URL  string
}{
	{"Lácteos, quesos y huevos", "https://www.gadis.es/tienda-online/lacteos-quesos-huevos/"},
	{"Charcutería", "https://www.gadis.es/tienda-online/charcuteria/"},
	{"Frutas y verduras", "https://www.gadis.es/tienda-online/frutas-y-verduras/"},
	{"Carnes y aves", "https://www.gadis.es/tienda-online/carnes-y-aves/"},
	{"Pescado y marisco", "https://www.gadis.es/tienda-online/pescado-y-marisco/"},
	{"Pan y bollería", "https://www.gadis.es/tienda-online/panaderia-bolleria/"},
	{"Aceite y conservas", "https://www.gadis.es/tienda-online/aceite-conservas-y-condimentos/"},
	{"Pasta y arroz", "https://www.gadis.es/tienda-online/pasta-arroz-y-legumbres/"},
	{"Bebidas", "https://www.gadis.es/tienda-online/bebidas/"},
	{"Congelados", "https://www.gadis.es/tienda-online/congelados/"},
	{"Cereales y galletas", "https://www.gadis.es/tienda-online/cereales-y-galletas/"},
	{"Droguería e higiene", "https://www.gadis.es/tienda-online/drogueria-e-higiene/"},
}

// Product card selectors to try in order — Gadis's React SPA may use any of these.
var gadisProductSelectors = []string{
	".product-card",
	".product-item",
	"[data-testid='product']",
	"article.product",
	"li.product",
	".product",
	"[class*='ProductCard']",
	"[class*='product-card']",
}

func (s *GadisScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	browser := getBrowser()
	var products []RawProduct

	for _, cat := range gadisCategories {
		select {
		case <-ctx.Done():
			return products, nil
		default:
		}

		catProducts, err := scrapeGadisCategory(ctx, browser, cat.Name, cat.URL)
		if err != nil {
			slog.Warn("gadis: category failed", slog.String("cat", cat.Name), slog.String("error", err.Error()))
			continue
		}
		products = append(products, catProducts...)
		slog.Info("gadis: category scraped", slog.String("cat", cat.Name), slog.Int("count", len(catProducts)))
	}

	if len(products) == 0 {
		slog.Warn("gadis: no products extracted — React SPA selectors may have changed")
	} else {
		slog.Info("gadis: scrape complete", slog.Int("products", len(products)))
	}
	return products, nil
}

func scrapeGadisCategory(ctx context.Context, browser *rod.Browser, catName, url string) ([]RawProduct, error) {
	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, fmt.Errorf("open page: %w", err)
	}
	defer func() { _ = page.Close() }()

	page = page.Context(ctx)

	if err := page.WaitLoad(); err != nil {
		slog.Warn("gadis: WaitLoad failed", slog.String("url", url))
	}

	// Extra wait for React hydration.
	time.Sleep(4 * time.Second)

	// Try each product selector.
	var els rod.Elements
	for _, sel := range gadisProductSelectors {
		els, _ = page.Elements(sel)
		if len(els) > 0 {
			break
		}
	}

	if len(els) == 0 {
		return nil, fmt.Errorf("no product elements on %s", url)
	}

	var products []RawProduct
	for _, el := range els {
		p := extractGadisProduct(el, catName)
		if p.Name == "" {
			continue
		}
		products = append(products, p)
	}
	return products, nil
}

func extractGadisProduct(el *rod.Element, catName string) RawProduct {
	var p RawProduct
	p.Category = catName

	nameSelectors := []string{
		".product-card__name",
		".product-name",
		"[class*='ProductName']",
		"[class*='product-name']",
		"h3", "h2",
		".name",
	}
	for _, sel := range nameSelectors {
		if nameEl, err := el.Element(sel); err == nil {
			if t, err := nameEl.Text(); err == nil && t != "" {
				p.Name = strings.TrimSpace(t)
				break
			}
		}
	}

	priceSelectors := []string{
		".product-card__price",
		".product-price",
		"[class*='Price']",
		"[class*='price']",
		".price",
	}
	for _, sel := range priceSelectors {
		if priceEl, err := el.Element(sel); err == nil {
			if t, err := priceEl.Text(); err == nil && t != "" {
				p.Price = parseGadisPrice(t)
				break
			}
		}
	}

	unitSelectors := []string{
		".product-card__unit",
		".product-unit",
		"[class*='unit']",
		".unit",
	}
	for _, sel := range unitSelectors {
		if unitEl, err := el.Element(sel); err == nil {
			if t, err := unitEl.Text(); err == nil && t != "" {
				p.Unit, p.UnitQuantity = ParseUnit(t)
				break
			}
		}
	}
	if p.Unit == "" {
		p.Unit = "unidad"
		p.UnitQuantity = 1
	}

	if imgEl, err := el.Element("img"); err == nil {
		if src, err := imgEl.Attribute("src"); err == nil && src != nil {
			p.ImageURL = *src
		} else if src, err := imgEl.Attribute("data-src"); err == nil && src != nil {
			p.ImageURL = *src
		}
	}

	if aEl, err := el.Element("a"); err == nil {
		if href, err := aEl.Attribute("href"); err == nil && href != nil {
			if strings.HasPrefix(*href, "http") {
				p.ProductURL = *href
			} else {
				p.ProductURL = "https://www.gadis.es" + *href
			}
		}
	}

	return p
}

func parseGadisPrice(s string) float64 {
	s = strings.ReplaceAll(s, "€", "")
	s = strings.ReplaceAll(s, "\u00a0", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")
	s = strings.TrimSpace(s)
	// Take first numeric token.
	for _, field := range strings.Fields(s) {
		if v, err := strconv.ParseFloat(field, 64); err == nil {
			return v
		}
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
