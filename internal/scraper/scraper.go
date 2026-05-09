package scraper

import (
	"context"
	"log/slog"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// RawProduct is the raw output from any scraper before normalisation.
type RawProduct struct {
	ExternalID   string
	Name         string
	Brand        string
	Price        float64
	PricePerUnit float64 // optional, 0 means unknown
	Unit         string  // "kg", "l", "unidad", "ud", etc.
	UnitQuantity float64 // 1.0, 0.5, 6 (pack), etc.
	Category     string
	ImageURL     string
	ProductURL   string
}

// Scraper is the interface all supermarket scrapers must implement.
type Scraper interface {
	Name() string
	Scrape(ctx context.Context) ([]RawProduct, error)
}

// Normalise lowercases and removes accents for matching.
func Normalise(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	return strings.ToLower(strings.TrimSpace(result))
}

// ParseUnit converts raw unit strings to canonical form.
func ParseUnit(raw string) (unit string, qty float64) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(raw, "kg"):
		unit = "kg"
		qty = parseQty(raw, "kg")
	case strings.Contains(raw, "gr") || strings.Contains(raw, "g"):
		unit = "kg"
		qty = parseQty(raw, "gr") / 1000
		if qty == 0 {
			qty = parseQty(raw, "g") / 1000
		}
	case strings.Contains(raw, "ml"):
		unit = "l"
		qty = parseQty(raw, "ml") / 1000
	case strings.Contains(raw, "cl"):
		unit = "l"
		qty = parseQty(raw, "cl") / 100
	case strings.Contains(raw, "l"):
		unit = "l"
		qty = parseQty(raw, "l")
	case strings.Contains(raw, "ud") || strings.Contains(raw, "unidad") || strings.Contains(raw, "u ") || raw == "u":
		unit = "unidad"
		qty = parseQty(raw, "ud")
		if qty == 0 {
			qty = 1
		}
	default:
		unit = "unidad"
		qty = 1
	}
	if qty == 0 {
		qty = 1
	}
	return
}

func parseQty(s, suffix string) float64 {
	idx := strings.Index(s, suffix)
	if idx == 0 {
		return 0
	}
	numStr := strings.TrimSpace(s[:idx])
	numStr = strings.ReplaceAll(numStr, ",", ".")
	var v float64
	_, _ = parseFloat(numStr, &v)
	return v
}

func parseFloat(s string, dest *float64) (int, error) {
	var v float64
	dotSeen := false
	dec := 1.0
	for _, c := range s {
		if c == '.' {
			dotSeen = true
			continue
		}
		if c < '0' || c > '9' {
			break
		}
		if dotSeen {
			dec *= 10
			v += float64(c-'0') / dec
		} else {
			v = v*10 + float64(c-'0')
		}
	}
	*dest = v
	return 0, nil
}

// LogScraperResult logs summary after a scrape run.
func LogScraperResult(name string, count int, err error) {
	if err != nil {
		slog.Error("scraper failed", slog.String("scraper", name), slog.String("error", err.Error()))
		return
	}
	slog.Info("scraper done", slog.String("scraper", name), slog.Int("products", count))
}
