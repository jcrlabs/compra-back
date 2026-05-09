package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jcrlabs/compra-back/internal/models"
)

type PricingRepository struct {
	db *pgxpool.Pool
}

func NewPricingRepository(db *pgxpool.Pool) *PricingRepository {
	return &PricingRepository{db: db}
}

func (r *PricingRepository) InsertPrice(p *models.Price) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	_, err := r.db.Exec(context.Background(),
		`INSERT INTO prices (id, product_id, supermarket, price, price_per_unit, external_id, external_url)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		p.ID, p.ProductID, string(p.Supermarket), p.Price, p.PricePerUnit, p.ExternalID, p.ExternalURL,
	)
	return err
}

func (r *PricingRepository) GetCurrentPrices(productID uuid.UUID) ([]models.Price, error) {
	rows, err := r.db.Query(context.Background(),
		`SELECT id, product_id, supermarket, price, price_per_unit, scraped_at, COALESCE(external_url,'')
		 FROM current_prices WHERE product_id=$1`, productID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrices(rows)
}

func (r *PricingRepository) GetCurrentPricesFiltered(productID uuid.UUID, supers []models.Supermarket) ([]models.Price, error) {
	if len(supers) == 0 {
		return r.GetCurrentPrices(productID)
	}
	placeholders := make([]string, len(supers))
	args := []any{productID}
	for i, s := range supers {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, string(s))
	}
	rows, err := r.db.Query(context.Background(),
		fmt.Sprintf(`SELECT id, product_id, supermarket, price, price_per_unit, scraped_at, COALESCE(external_url,'')
		 FROM current_prices WHERE product_id=$1 AND supermarket IN (%s)`,
			strings.Join(placeholders, ",")),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrices(rows)
}

func (r *PricingRepository) GetPriceHistory(productID uuid.UUID, super models.Supermarket, limit int) ([]models.Price, error) {
	if limit == 0 {
		limit = 30
	}
	rows, err := r.db.Query(context.Background(),
		`SELECT id, product_id, supermarket, price, price_per_unit, scraped_at, COALESCE(external_url,'')
		 FROM prices WHERE product_id=$1 AND supermarket=$2
		 ORDER BY scraped_at DESC LIMIT $3`,
		productID, string(super), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrices(rows)
}

func (r *PricingRepository) GetBestPrices(productIDs []uuid.UUID, supers []models.Supermarket) ([]models.BestPrice, error) {
	if len(productIDs) == 0 {
		return nil, nil
	}

	idPlaceholders := make([]string, len(productIDs))
	args := []any{}
	for i, id := range productIDs {
		idPlaceholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, id)
	}

	superFilter := ""
	if len(supers) > 0 {
		sPlaceholders := make([]string, len(supers))
		for i, s := range supers {
			sPlaceholders[i] = fmt.Sprintf("$%d", len(args)+1)
			args = append(args, string(s))
			i++
		}
		superFilter = fmt.Sprintf(" AND supermarket IN (%s)", strings.Join(sPlaceholders, ","))
	}

	rows, err := r.db.Query(context.Background(),
		fmt.Sprintf(`
			SELECT DISTINCT ON (product_id)
			    product_id, supermarket, price, COALESCE(external_url,'')
			FROM current_prices
			WHERE product_id IN (%s)%s
			ORDER BY product_id, price ASC
		`, strings.Join(idPlaceholders, ","), superFilter),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.BestPrice
	for rows.Next() {
		var bp models.BestPrice
		var superStr string
		if err := rows.Scan(&bp.ProductID, &superStr, &bp.Price, &bp.ExternalURL); err != nil {
			return nil, err
		}
		bp.Supermarket = models.Supermarket(superStr)
		result = append(result, bp)
	}
	return result, nil
}

func scanPrices(rows interface {
	Next() bool
	Scan(...any) error
}) ([]models.Price, error) {
	var prices []models.Price
	for rows.Next() {
		var p models.Price
		var superStr string
		if err := rows.Scan(&p.ID, &p.ProductID, &superStr, &p.Price, &p.PricePerUnit, &p.ScrapedAt, &p.ExternalURL); err != nil {
			return nil, err
		}
		p.Supermarket = models.Supermarket(superStr)
		prices = append(prices, p)
	}
	return prices, nil
}
