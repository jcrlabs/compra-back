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

// EroskiScraper uses go-rod to navigate supermercado.eroski.es,
// complete the store-selection flow, and then scrape category pages.

type EroskiScraper struct {
	userAgent string
}

func NewEroskiScraper(userAgent string) *EroskiScraper {
	return &EroskiScraper{userAgent: userAgent}
}

func (s *EroskiScraper) Name() string { return "eroski" }

var eroskiCategories = []struct {
	Name string
	Path string
}{
	{"Lácteos y huevos", "/es/alimentacion/lacteos-huevos-mantequilla/"},
	{"Charcutería y quesos", "/es/alimentacion/charcuteria-y-quesos/"},
	{"Frutas y verduras", "/es/alimentacion/frutas-y-verduras/"},
	{"Carnes y aves", "/es/alimentacion/carne-y-aves/"},
	{"Pescado y marisco", "/es/alimentacion/pescado-y-marisco/"},
	{"Pan y bollería", "/es/alimentacion/pan-bolleria-pasteleria/"},
	{"Aceite y conservas", "/es/alimentacion/aceite-vinagre-condimentos-y-conservas/"},
	{"Pasta y arroz", "/es/alimentacion/pasta-arroz-y-legumbres/"},
	{"Bebidas", "/es/alimentacion/bebidas/"},
	{"Congelados", "/es/alimentacion/congelados/"},
}

const eroskiBase = "https://supermercado.eroski.es"

func (s *EroskiScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	browser := getBrowser()

	// Step 1: open homepage and complete store selection if prompted.
	home, err := browser.Page(proto.TargetCreateTarget{URL: eroskiBase + "/"})
	if err != nil {
		return nil, fmt.Errorf("eroski: open home: %w", err)
	}
	home = home.Context(ctx)
	if err := home.WaitLoad(); err != nil {
		slog.Warn("eroski: home WaitLoad failed", slog.String("error", err.Error()))
	}
	time.Sleep(2 * time.Second)

	// Try to dismiss store-selection modal by clicking first available store or CP input.
	_ = rod.Try(func() {
		// Try postal code input.
		el := home.MustElement(`input[name*="postal"], input[placeholder*="código postal"], input[placeholder*="CP"]`)
		el.MustInput("48001") // Bilbao — Eroski HQ region.
		el.MustType('\r')
		time.Sleep(2 * time.Second)
	})

	_ = rod.Try(func() {
		// Alternatively click the first store button.
		btn := home.MustElement(`button.store-selector, button[data-testid*="store"], .store-list button, .tienda button`)
		btn.MustClick()
		time.Sleep(2 * time.Second)
	})

	_ = home.Close()

	var products []RawProduct

	for _, cat := range eroskiCategories {
		select {
		case <-ctx.Done():
			return products, nil
		default:
		}

		catURL := eroskiBase + cat.Path
		catProducts, err := scrapeEroskiCategory(ctx, browser, cat.Name, catURL)
		if err != nil {
			slog.Warn("eroski: category failed", slog.String("cat", cat.Name), slog.String("error", err.Error()))
			continue
		}
		products = append(products, catProducts...)
		slog.Info("eroski: category scraped", slog.String("cat", cat.Name), slog.Int("count", len(catProducts)))
	}

	if len(products) == 0 {
		slog.Warn("eroski: no products found — store selection may have failed")
	} else {
		slog.Info("eroski: scrape complete", slog.Int("products", len(products)))
	}
	return products, nil
}

func scrapeEroskiCategory(ctx context.Context, browser *rod.Browser, catName, url string) ([]RawProduct, error) {
	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, fmt.Errorf("open page: %w", err)
	}
	defer func() { _ = page.Close() }()

	page = page.Context(ctx)
	if err := page.WaitLoad(); err != nil {
		slog.Warn("eroski: WaitLoad failed", slog.String("url", url))
	}
	time.Sleep(3 * time.Second)

	selectors := []string{
		".product-item",
		".product-card",
		"[data-testid='product']",
		"article.product",
		"li.product",
		".product",
	}

	var els rod.Elements
	for _, sel := range selectors {
		els, _ = page.Elements(sel)
		if len(els) > 0 {
			break
		}
	}

	if len(els) == 0 {
		return nil, fmt.Errorf("no products on %s", url)
	}

	var products []RawProduct
	for _, el := range els {
		p := extractEroskiProduct(el, catName)
		if p.Name == "" {
			continue
		}
		products = append(products, p)
	}
	return products, nil
}

func extractEroskiProduct(el *rod.Element, catName string) RawProduct {
	var p RawProduct
	p.Category = catName

	nameSelectors := []string{".product-item__description-name", ".product-name", "h3", "h2", ".name"}
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
				p.Price = parseEroskiPrice(t)
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
				p.ProductURL = eroskiBase + *href
			}
		}
	}

	p.Unit = "unidad"
	p.UnitQuantity = 1
	return p
}

func parseEroskiPrice(s string) float64 {
	s = strings.ReplaceAll(s, "€", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")
	s = strings.TrimSpace(s)
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
