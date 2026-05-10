package scraper

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// EroskiScraper fetches products from supermercado.eroski.es using HTTP + HTML parsing.
// The site uses Apache Tapestry (server-rendered HTML) with GA4 ecommerce data embedded.
// Store 157 (Bilbondo, Bilbao) is used as a reliable reference store.

type EroskiScraper struct {
	client *http.Client
}

func NewEroskiScraper(_ string) *EroskiScraper {
	return &EroskiScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *EroskiScraper) Name() string { return "eroski" }

const eroskiBase = "https://supermercado.eroski.es"

// eroskiCookies sets a known working store (Bilbondo, shop=157).
const eroskiCookies = "supermarket.locale=es; supermarket.direct_access=true; supermarket.ali.site=eroski; supermarket.ali.shop=157"

// Search terms to cover main product categories.
var eroskiSearchTerms = []struct {
	category string
	query    string
}{
	{"Lácteos", "leche"},
	{"Lácteos", "yogur"},
	{"Lácteos", "queso"},
	{"Lácteos", "mantequilla"},
	{"Huevos", "huevos"},
	{"Charcutería", "jamon"},
	{"Charcutería", "embutido"},
	{"Carne", "pollo"},
	{"Carne", "ternera"},
	{"Carne", "cerdo"},
	{"Pescado", "salmon"},
	{"Pescado", "merluza"},
	{"Pescado", "atun"},
	{"Pan", "pan"},
	{"Bollería", "galletas"},
	{"Bollería", "cereales"},
	{"Aceite", "aceite"},
	{"Conservas", "conservas"},
	{"Pasta", "pasta"},
	{"Pasta", "arroz"},
	{"Legumbres", "legumbres"},
	{"Bebidas", "agua"},
	{"Bebidas", "refresco"},
	{"Bebidas", "cerveza"},
	{"Bebidas", "vino"},
	{"Bebidas", "zumo"},
	{"Congelados", "congelado"},
	{"Congelados", "pizza"},
	{"Higiene", "champu"},
	{"Higiene", "gel"},
	{"Limpieza", "detergente"},
	{"Limpieza", "suavizante"},
}

var (
	eroskiProductRe = regexp.MustCompile(`/productdetail/(\d+)-([^/"]+)/`)
	eroskiPriceRe   = regexp.MustCompile(`item_name&quot;:&quot;([^&]+)&quot;[^}]*?price&quot;:(\d+(?:\.\d+)?)`)
	eroskiImageRe   = regexp.MustCompile(`data-big-images="\[https://supermercado\.eroski\.es//images/(\d+)_[^"]+\.jpg`)
)

func (s *EroskiScraper) Scrape(ctx context.Context) ([]RawProduct, error) {
	slog.Info("eroski: starting scrape")

	seen := make(map[string]bool) // deduplicate by product ID
	var products []RawProduct

	for _, term := range eroskiSearchTerms {
		if ctx.Err() != nil {
			break
		}

		termProducts, err := s.scrapeSearchTerm(ctx, term.category, term.query, seen)
		if err != nil {
			slog.Warn("eroski: search failed", slog.String("q", term.query), slog.String("error", err.Error()))
			continue
		}
		products = append(products, termProducts...)
		slog.Info("eroski: term scraped", slog.String("q", term.query), slog.Int("new", len(termProducts)))

		select {
		case <-ctx.Done():
			goto done
		case <-time.After(300 * time.Millisecond):
		}
	}

done:
	slog.Info("eroski: scrape complete", slog.Int("products", len(products)))
	return products, nil
}

func (s *EroskiScraper) scrapeSearchTerm(ctx context.Context, category, query string, seen map[string]bool) ([]RawProduct, error) {
	var products []RawProduct

	for page := 0; page < 10; page++ {
		if ctx.Err() != nil {
			break
		}

		url := fmt.Sprintf("%s/es/search/results/?q=%s&numberResultsPerPage=40&currentPage=%d", eroskiBase, query, page)
		html, err := s.fetchHTML(ctx, url)
		if err != nil {
			return products, err
		}

		pageProducts := extractEroskiProducts(html, category, seen)
		if len(pageProducts) == 0 {
			break // no more results
		}
		products = append(products, pageProducts...)

		// If fewer than 40 results, this was the last page.
		if len(pageProducts) < 40 {
			break
		}

		select {
		case <-ctx.Done():
			return products, nil
		case <-time.After(200 * time.Millisecond):
		}
	}

	return products, nil
}

func extractEroskiProducts(html, category string, seen map[string]bool) []RawProduct {
	// Extract name+price pairs from GA4 encoded data.
	matches := eroskiPriceRe.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return nil
	}

	// Extract product IDs and URLs in order.
	urlMatches := eroskiProductRe.FindAllStringSubmatch(html, -1)

	// Extract image IDs in order.
	imgMatches := eroskiImageRe.FindAllStringSubmatch(html, -1)

	var products []RawProduct
	for i, m := range matches {
		name := strings.TrimSpace(m[1])
		priceStr := m[2]
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil || name == "" || price == 0 {
			continue
		}

		pid := ""
		productURL := ""
		if i < len(urlMatches) {
			pid = urlMatches[i][1]
			slug := urlMatches[i][2]
			productURL = fmt.Sprintf("%s/es/productdetail/%s-%s/", eroskiBase, pid, slug)
		}

		if pid != "" && seen[pid] {
			continue
		}
		if pid != "" {
			seen[pid] = true
		}

		imgURL := ""
		if i < len(imgMatches) {
			imgURL = fmt.Sprintf("https://supermercado.eroski.es//images/%s_x.jpg", imgMatches[i][1])
		}

		products = append(products, RawProduct{
			ExternalID:   pid,
			Name:         name,
			Price:        price,
			Category:     category,
			ImageURL:     imgURL,
			ProductURL:   productURL,
			Unit:         "unidad",
			UnitQuantity: 1,
		})
	}
	return products
}

func (s *EroskiScraper) fetchHTML(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Cookie", eroskiCookies)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
