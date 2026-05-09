package scraper

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

type GadisScraper struct {
	userAgent string
}

func NewGadisScraper(userAgent string) *GadisScraper {
	return &GadisScraper{userAgent: userAgent}
}

func (s *GadisScraper) Name() string { return "gadis" }

var gadisCategoryURLs = []struct {
	URL  string
	Name string
}{
	{"https://www.gadis.es/tienda-online/lacteos-quesos-huevos/", "Lácteos"},
	{"https://www.gadis.es/tienda-online/frutas-y-verduras/", "Frutas y verduras"},
	{"https://www.gadis.es/tienda-online/carne-y-aves/", "Carnicería"},
	{"https://www.gadis.es/tienda-online/pescados-y-mariscos/", "Pescadería"},
	{"https://www.gadis.es/tienda-online/pan-y-bolleria/", "Pan y bollería"},
	{"https://www.gadis.es/tienda-online/congelados/", "Congelados"},
	{"https://www.gadis.es/tienda-online/bebidas/", "Bebidas"},
	{"https://www.gadis.es/tienda-online/aceite-arroz-legumbres/", "Aceite y conservas"},
	{"https://www.gadis.es/tienda-online/drogueria/", "Droguería"},
	{"https://www.gadis.es/tienda-online/higiene-y-belleza/", "Higiene y salud"},
}

func (s *GadisScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	var products []RawProduct

	c := colly.NewCollector(
		colly.UserAgent(s.userAgent),
		colly.MaxDepth(1),
	)
	c.SetRequestTimeout(30 * time.Second)
	_ = c.Limit(&colly.LimitRule{
		DomainGlob:  "*gadis.es*",
		Delay:       700 * time.Millisecond,
		RandomDelay: 300 * time.Millisecond,
	})

	var currentCat string

	c.OnHTML(".producto, .product, .item-product", func(e *colly.HTMLElement) {
		name := strings.TrimSpace(e.ChildText(".nombre, .product-name, h3, h4"))
		priceStr := strings.TrimSpace(e.ChildText(".precio, .price, .product-price"))
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
			productURL = "https://www.gadis.es" + productURL
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

	for _, cat := range gadisCategoryURLs {
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
