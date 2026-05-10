package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jcrlabs/compra-back/internal/models"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var u models.User
	err := r.db.QueryRow(context.Background(),
		`SELECT id, email, password, name, created_at, updated_at FROM users WHERE email=$1`,
		email,
	).Scan(&u.ID, &u.Email, &u.Password, &u.Name, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) FindByID(id uuid.UUID) (*models.User, error) {
	var u models.User
	err := r.db.QueryRow(context.Background(),
		`SELECT id, email, password, name, created_at, updated_at FROM users WHERE id=$1`,
		id,
	).Scan(&u.ID, &u.Email, &u.Password, &u.Name, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) Create(u *models.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	_, err := r.db.Exec(context.Background(),
		`INSERT INTO users (id, email, password, name) VALUES ($1,$2,$3,$4)`,
		u.ID, u.Email, u.Password, u.Name,
	)
	return err
}

func (r *UserRepository) Update(u *models.User) error {
	_, err := r.db.Exec(context.Background(),
		`UPDATE users SET name=$1, updated_at=now() WHERE id=$2`,
		u.Name, u.ID,
	)
	return err
}

func (r *UserRepository) GetSupermarketPrefs(userID uuid.UUID) (map[models.Supermarket]bool, error) {
	rows, err := r.db.Query(context.Background(),
		`SELECT supermarket, enabled FROM user_supermarket_prefs WHERE user_id=$1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prefs := make(map[models.Supermarket]bool)
	for _, s := range models.AllSupermarkets {
		prefs[s] = true
	}
	for rows.Next() {
		var super models.Supermarket
		var enabled bool
		if err := rows.Scan(&super, &enabled); err != nil {
			continue
		}
		prefs[super] = enabled
	}
	return prefs, nil
}

func (r *UserRepository) SetSupermarketPrefs(userID uuid.UUID, prefs map[models.Supermarket]bool) error {
	ctx := context.Background()
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `DELETE FROM user_supermarket_prefs WHERE user_id=$1`, userID)
	if err != nil {
		return err
	}

	for super, enabled := range prefs {
		_, err = tx.Exec(ctx,
			`INSERT INTO user_supermarket_prefs (user_id, supermarket, enabled) VALUES ($1,$2,$3)`,
			userID, string(super), enabled,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
