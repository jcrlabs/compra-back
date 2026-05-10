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

// FroizScraper uses go-rod to navigate froiz.com's online shop.
// The shop may have been discontinued; if no products are found the scraper
// returns nil, nil with a warning log.

type FroizScraper struct {
	userAgent string
}

func NewFroizScraper(userAgent string) *FroizScraper {
	return &FroizScraper{userAgent: userAgent}
}

func (s *FroizScraper) Name() string { return "froiz" }

// froizShopURLs lists candidate entry points for Froiz's online shop.
var froizShopURLs = []string{
	"https://www.froiz.com/tienda/",
	"https://www.froiz.com/supermercado/",
	"https://www.froiz.com/compra-online/",
	"https://www.froiz.com/shop/",
	"https://www.froiz.com/",
}

func (s *FroizScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	browser := getBrowser()

	// Try to find a working shop entry point.
	var shopURL string
	for _, candidate := range froizShopURLs {
		page, err := browser.Page(proto.TargetCreateTarget{URL: candidate})
		if err != nil {
			continue
		}
		page = page.Context(ctx)
		if err := page.WaitLoad(); err != nil {
			_ = page.Close()
			continue
		}
		time.Sleep(2 * time.Second)

		// Look for product or category links.
		links, _ := page.Elements("a[href]")
		for _, link := range links {
			href, _ := link.Attribute("href")
			if href == nil {
				continue
			}
			h := strings.ToLower(*href)
			if strings.Contains(h, "product") || strings.Contains(h, "categoria") ||
				strings.Contains(h, "category") || strings.Contains(h, "tienda") {
				shopURL = candidate
				break
			}
		}
		_ = page.Close()
		if shopURL != "" {
			break
		}
	}

	if shopURL == "" {
		slog.Warn("froiz: no active online shop found — site may be discontinued or moved to a third-party platform")
		return nil, nil
	}

	products, err := scrapeFroizShop(ctx, browser, shopURL)
	if err != nil {
		slog.Warn("froiz: scrape failed", slog.String("error", err.Error()))
		return nil, nil
	}

	if len(products) == 0 {
		slog.Warn("froiz: shop found but no products extracted", slog.String("url", shopURL))
	} else {
		slog.Info("froiz: scrape complete", slog.Int("products", len(products)))
	}
	return products, nil
}

func scrapeFroizShop(ctx context.Context, browser *rod.Browser, shopURL string) ([]RawProduct, error) {
	page, err := browser.Page(proto.TargetCreateTarget{URL: shopURL})
	if err != nil {
		return nil, fmt.Errorf("open shop page: %w", err)
	}
	defer func() { _ = page.Close() }()

	page = page.Context(ctx)
	if err := page.WaitLoad(); err != nil {
		slog.Warn("froiz: WaitLoad failed", slog.String("url", shopURL))
	}
	time.Sleep(3 * time.Second)

	selectors := []string{
		".product",
		".product-item",
		".product-card",
		"article.product",
		"li.product",
		"[class*='product']",
	}

	var els rod.Elements
	for _, sel := range selectors {
		els, _ = page.Elements(sel)
		if len(els) > 0 {
			break
		}
	}

	if len(els) == 0 {
		return nil, fmt.Errorf("no product elements found on %s", shopURL)
	}

	var products []RawProduct
	for _, el := range els {
		p := extractFroizProduct(el)
		if p.Name == "" {
			continue
		}
		products = append(products, p)
	}
	return products, nil
}

func extractFroizProduct(el *rod.Element) RawProduct {
	var p RawProduct

	nameSelectors := []string{".product-name", ".product-title", "h3", "h2", ".name", ".title"}
	for _, sel := range nameSelectors {
		if nameEl, err := el.Element(sel); err == nil {
			if t, err := nameEl.Text(); err == nil && t != "" {
				p.Name = strings.TrimSpace(t)
				break
			}
		}
	}

	priceSelectors := []string{".price", ".product-price", ".amount", "[class*='price']"}
	for _, sel := range priceSelectors {
		if priceEl, err := el.Element(sel); err == nil {
			if t, err := priceEl.Text(); err == nil && t != "" {
				p.Price = parseFroizPrice(t)
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
				p.ProductURL = "https://www.froiz.com" + *href
			}
		}
	}

	p.Unit = "unidad"
	p.UnitQuantity = 1
	return p
}

func parseFroizPrice(s string) float64 {
	s = strings.ReplaceAll(s, "€", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")
	s = strings.TrimSpace(s)
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
