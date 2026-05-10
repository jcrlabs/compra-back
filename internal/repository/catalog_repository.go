package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jcrlabs/compra-back/internal/models"
)

type CatalogRepository struct {
	db *pgxpool.Pool
}

func NewCatalogRepository(db *pgxpool.Pool) *CatalogRepository {
	return &CatalogRepository{db: db}
}

// Categories

func (r *CatalogRepository) ListCategories() ([]models.Category, error) {
	rows, err := r.db.Query(context.Background(),
		`SELECT id, name, slug, parent_id, COALESCE(icon,''), created_at FROM categories ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []models.Category
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID, &c.Icon, &c.CreatedAt); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, nil
}

func (r *CatalogRepository) GetCategory(id uuid.UUID) (*models.Category, error) {
	var c models.Category
	err := r.db.QueryRow(context.Background(),
		`SELECT id, name, slug, parent_id, COALESCE(icon,''), created_at FROM categories WHERE id=$1`, id,
	).Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID, &c.Icon, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("category not found: %w", err)
	}
	return &c, nil
}

// Products

type ProductFilter struct {
	Search      string
	CategoryID  *uuid.UUID
	Supermarket string
	Page        int
	Limit       int
}

type ProductPage struct {
	Products []models.Product `json:"products"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	Limit    int              `json:"limit"`
}

func (r *CatalogRepository) ListProducts(f ProductFilter) (*ProductPage, error) {
	if f.Limit == 0 {
		f.Limit = 40
	}
	if f.Page < 1 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.Limit

	where := []string{"1=1"}
	args := []any{}
	i := 1

	if f.Search != "" {
		where = append(where, fmt.Sprintf("to_tsvector('spanish', p.name) @@ plainto_tsquery('spanish', $%d)", i))
		args = append(args, f.Search)
		i++
	}
	if f.CategoryID != nil {
		where = append(where, fmt.Sprintf("p.category_id=$%d", i))
		args = append(args, *f.CategoryID)
		i++
	}
	if f.Supermarket != "" {
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM current_prices cp WHERE cp.product_id=p.id AND cp.supermarket=$%d)", i))
		args = append(args, f.Supermarket)
		i++
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	err := r.db.QueryRow(context.Background(),
		fmt.Sprintf(`SELECT COUNT(*) FROM products p WHERE %s`, whereClause),
		args...,
	).Scan(&total)
	if err != nil {
		return nil, err
	}

	args = append(args, f.Limit, offset)
	rows, err := r.db.Query(context.Background(),
		fmt.Sprintf(`
			SELECT p.id, p.name, COALESCE(p.brand,''), p.category_id,
			       p.unit, p.unit_quantity, COALESCE(p.image_url,''), p.created_at, p.updated_at,
			       mp.price, mp.supermarket
			FROM products p
			LEFT JOIN LATERAL (
			    SELECT price, supermarket
			    FROM current_prices cp
			    WHERE cp.product_id = p.id
			    ORDER BY price ASC
			    LIMIT 1
			) mp ON true
			WHERE %s
			ORDER BY p.name
			LIMIT $%d OFFSET $%d
		`, whereClause, i, i+1),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		var minPrice *float64
		var cheapestSuper *string
		if err := rows.Scan(&p.ID, &p.Name, &p.Brand, &p.CategoryID,
			&p.Unit, &p.UnitQuantity, &p.ImageURL, &p.CreatedAt, &p.UpdatedAt,
			&minPrice, &cheapestSuper); err != nil {
			return nil, err
		}
		p.MinPrice = minPrice
		if cheapestSuper != nil {
			p.CheapestSuper = *cheapestSuper
		}
		products = append(products, p)
	}

	return &ProductPage{
		Products: products,
		Total:    total,
		Page:     f.Page,
		Limit:    f.Limit,
	}, nil
}

func (r *CatalogRepository) GetProduct(id uuid.UUID) (*models.Product, error) {
	var p models.Product
	err := r.db.QueryRow(context.Background(),
		`SELECT id, name, COALESCE(brand,''), category_id, unit, unit_quantity, COALESCE(image_url,''), created_at, updated_at
		 FROM products WHERE id=$1`, id,
	).Scan(&p.ID, &p.Name, &p.Brand, &p.CategoryID, &p.Unit, &p.UnitQuantity, &p.ImageURL, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}
	return &p, nil
}

func (r *CatalogRepository) UpsertProduct(p *models.Product) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	_, err := r.db.Exec(context.Background(),
		`INSERT INTO products (id, name, brand, category_id, unit, unit_quantity, image_url)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (id) DO UPDATE SET
		   name=EXCLUDED.name, brand=EXCLUDED.brand, category_id=EXCLUDED.category_id,
		   unit=EXCLUDED.unit, unit_quantity=EXCLUDED.unit_quantity,
		   image_url=EXCLUDED.image_url, updated_at=now()`,
		p.ID, p.Name, p.Brand, p.CategoryID, p.Unit, p.UnitQuantity, p.ImageURL,
	)
	return err
}

// FindProductByNameBrandUnit — fuzzy matching helper for scrapers
func (r *CatalogRepository) FindProductByNameBrandUnit(name, brand, unit string, qty float64) (*models.Product, error) {
	var p models.Product
	err := r.db.QueryRow(context.Background(),
		`SELECT id, name, COALESCE(brand,''), category_id, unit, unit_quantity, COALESCE(image_url,''), created_at, updated_at
		 FROM products
		 WHERE lower(name)=lower($1) AND lower(COALESCE(brand,''))=lower($2)
		   AND unit=$3 AND unit_quantity=$4
		 LIMIT 1`,
		name, brand, unit, qty,
	).Scan(&p.ID, &p.Name, &p.Brand, &p.CategoryID, &p.Unit, &p.UnitQuantity, &p.ImageURL, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
