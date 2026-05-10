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

// CarrefourScraper uses go-rod to navigate category pages and extract products
// from the rendered DOM after the React SPA has loaded.

type CarrefourScraper struct {
	userAgent string
}

func NewCarrefourScraper(userAgent string) *CarrefourScraper {
	return &CarrefourScraper{userAgent: userAgent}
}

func (s *CarrefourScraper) Name() string { return "carrefour" }

var carrefourCategories = []struct {
	Name string
	URL  string
}{
	{"Lácteos, huevos y margarina", "https://www.carrefour.es/supermercado/lacteos-huevos-y-margarina/cat20138/"},
	{"Charcutería y quesos", "https://www.carrefour.es/supermercado/charcuteria-y-quesos/cat20120/"},
	{"Frutas y verduras", "https://www.carrefour.es/supermercado/frutas-y-verduras/cat20129/"},
	{"Carne y aves", "https://www.carrefour.es/supermercado/carne-y-aves/cat20119/"},
	{"Pescado y marisco", "https://www.carrefour.es/supermercado/pescado-y-marisco/cat20143/"},
	{"Pan, bollería y pastelería", "https://www.carrefour.es/supermercado/pan-bolleria-y-pasteleria/cat20140/"},
	{"Aceite, condimentos y conservas", "https://www.carrefour.es/supermercado/aceite-condimentos-y-conservas/cat20101/"},
	{"Pasta, arroz y legumbres", "https://www.carrefour.es/supermercado/pasta-arroz-y-legumbres/cat20142/"},
	{"Bebidas", "https://www.carrefour.es/supermercado/bebidas/cat20114/"},
	{"Congelados", "https://www.carrefour.es/supermercado/congelados/cat20125/"},
	{"Cereales y galletas", "https://www.carrefour.es/supermercado/cereales-y-galletas/cat20117/"},
	{"Higiene y belleza", "https://www.carrefour.es/supermercado/higiene-y-belleza/cat20131/"},
	{"Limpieza del hogar", "https://www.carrefour.es/supermercado/limpieza-del-hogar/cat20135/"},
}

func (s *CarrefourScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	browser := getBrowser()
	var products []RawProduct

	for _, cat := range carrefourCategories {
		select {
		case <-ctx.Done():
			return products, nil
		default:
		}

		catProducts, err := scrapeCarrefourCategory(ctx, browser, cat.Name, cat.URL)
		if err != nil {
			slog.Warn("carrefour: category failed", slog.String("cat", cat.Name), slog.String("error", err.Error()))
			continue
		}
		products = append(products, catProducts...)
		slog.Info("carrefour: category scraped", slog.String("cat", cat.Name), slog.Int("count", len(catProducts)))
	}

	slog.Info("carrefour: scrape complete", slog.Int("products", len(products)))
	return products, nil
}

func scrapeCarrefourCategory(ctx context.Context, browser *rod.Browser, catName, url string) ([]RawProduct, error) {
	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, fmt.Errorf("open page: %w", err)
	}
	defer func() { _ = page.Close() }()

	page = page.Context(ctx)

	// Wait for product cards to render.
	if err := page.WaitLoad(); err != nil {
		slog.Warn("carrefour: WaitLoad failed", slog.String("url", url), slog.String("error", err.Error()))
	}

	// Extra wait for JS rendering.
	time.Sleep(3 * time.Second)

	// Try multiple selectors Carrefour has used.
	selectors := []string{
		".product-card",
		"[data-testid='product-card']",
		".shelf-product-card",
		"article.product",
		".product__item",
	}

	var els rod.Elements
	for _, sel := range selectors {
		els, err = page.Elements(sel)
		if err == nil && len(els) > 0 {
			break
		}
	}

	if len(els) == 0 {
		return nil, fmt.Errorf("no product elements found on %s", url)
	}

	var products []RawProduct
	for _, el := range els {
		p, err := extractCarrefourProduct(el, catName, url)
		if err != nil || p.Name == "" {
			continue
		}
		products = append(products, p)
	}
	return products, nil
}

func extractCarrefourProduct(el *rod.Element, catName, pageURL string) (RawProduct, error) {
	var p RawProduct
	p.Category = catName

	// Name.
	nameSelectors := []string{
		".product-card__title",
		"[data-testid='product-title']",
		".product-name",
		"h3",
		"h2",
	}
	for _, sel := range nameSelectors {
		if nameEl, err := el.Element(sel); err == nil {
			if t, err := nameEl.Text(); err == nil && t != "" {
				p.Name = strings.TrimSpace(t)
				break
			}
		}
	}

	// Price.
	priceSelectors := []string{
		".product-card__price",
		"[data-testid='product-price']",
		".buybox__price",
		".price",
		".product-price",
	}
	for _, sel := range priceSelectors {
		if priceEl, err := el.Element(sel); err == nil {
			if t, err := priceEl.Text(); err == nil && t != "" {
				p.Price = parseCarrefourPrice(t)
				break
			}
		}
	}

	// Image.
	if imgEl, err := el.Element("img"); err == nil {
		if src, err := imgEl.Attribute("src"); err == nil && src != nil {
			p.ImageURL = *src
		}
	}

	// Product URL.
	if aEl, err := el.Element("a"); err == nil {
		if href, err := aEl.Attribute("href"); err == nil && href != nil {
			if strings.HasPrefix(*href, "http") {
				p.ProductURL = *href
			} else {
				p.ProductURL = "https://www.carrefour.es" + *href
			}
		}
	}

	// Unit — try to extract from price-per-unit text.
	unitSelectors := []string{
		".product-card__price-per-unit",
		"[data-testid='price-per-unit']",
		".price-per-unit",
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

	// ExternalID from URL.
	parts := strings.Split(strings.TrimRight(pageURL, "/"), "/")
	if len(parts) > 0 {
		p.ExternalID = parts[len(parts)-1]
	}

	_ = catName
	return p, nil
}

func parseCarrefourPrice(s string) float64 {
	s = strings.ReplaceAll(s, "€", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")
	s = strings.TrimSpace(s)
	// Take first number only.
	fields := strings.Fields(s)
	if len(fields) > 0 {
		s = fields[0]
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
