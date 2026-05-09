package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jcrlabs/compra-back/internal/models"
)

type ListRepository struct {
	db *pgxpool.Pool
}

func NewListRepository(db *pgxpool.Pool) *ListRepository {
	return &ListRepository{db: db}
}

func (r *ListRepository) GetListsForUser(userID uuid.UUID) ([]models.ShoppingList, error) {
	rows, err := r.db.Query(context.Background(),
		`SELECT DISTINCT sl.id, sl.owner_id, sl.name, sl.share_token, sl.created_at, sl.updated_at
		 FROM shopping_lists sl
		 LEFT JOIN list_members lm ON lm.list_id=sl.id
		 WHERE sl.owner_id=$1 OR lm.user_id=$1
		 ORDER BY sl.updated_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []models.ShoppingList
	for rows.Next() {
		var l models.ShoppingList
		if err := rows.Scan(&l.ID, &l.OwnerID, &l.Name, &l.ShareToken, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		lists = append(lists, l)
	}
	return lists, nil
}

func (r *ListRepository) GetList(id uuid.UUID) (*models.ShoppingList, error) {
	var l models.ShoppingList
	err := r.db.QueryRow(context.Background(),
		`SELECT id, owner_id, name, share_token, created_at, updated_at FROM shopping_lists WHERE id=$1`, id,
	).Scan(&l.ID, &l.OwnerID, &l.Name, &l.ShareToken, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("list not found: %w", err)
	}
	return &l, nil
}

func (r *ListRepository) GetListByShareToken(token uuid.UUID) (*models.ShoppingList, error) {
	var l models.ShoppingList
	err := r.db.QueryRow(context.Background(),
		`SELECT id, owner_id, name, share_token, created_at, updated_at FROM shopping_lists WHERE share_token=$1`, token,
	).Scan(&l.ID, &l.OwnerID, &l.Name, &l.ShareToken, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("list not found: %w", err)
	}
	return &l, nil
}

func (r *ListRepository) CreateList(l *models.ShoppingList) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	_, err := r.db.Exec(context.Background(),
		`INSERT INTO shopping_lists (id, owner_id, name) VALUES ($1,$2,$3)`,
		l.ID, l.OwnerID, l.Name,
	)
	if err != nil {
		return err
	}
	// Reload share_token
	return r.db.QueryRow(context.Background(),
		`SELECT share_token, created_at, updated_at FROM shopping_lists WHERE id=$1`, l.ID,
	).Scan(&l.ShareToken, &l.CreatedAt, &l.UpdatedAt)
}

func (r *ListRepository) UpdateList(l *models.ShoppingList) error {
	_, err := r.db.Exec(context.Background(),
		`UPDATE shopping_lists SET name=$1, updated_at=now() WHERE id=$2`,
		l.Name, l.ID,
	)
	return err
}

func (r *ListRepository) DeleteList(id uuid.UUID) error {
	_, err := r.db.Exec(context.Background(), `DELETE FROM shopping_lists WHERE id=$1`, id)
	return err
}

func (r *ListRepository) AddMember(listID, userID uuid.UUID) error {
	_, err := r.db.Exec(context.Background(),
		`INSERT INTO list_members (list_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		listID, userID,
	)
	return err
}

func (r *ListRepository) IsMember(listID, userID uuid.UUID) (bool, error) {
	var owner uuid.UUID
	err := r.db.QueryRow(context.Background(),
		`SELECT owner_id FROM shopping_lists WHERE id=$1`, listID,
	).Scan(&owner)
	if err != nil {
		return false, err
	}
	if owner == userID {
		return true, nil
	}
	var exists bool
	_ = r.db.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM list_members WHERE list_id=$1 AND user_id=$2)`,
		listID, userID,
	).Scan(&exists)
	return exists, nil
}

// Items

func (r *ListRepository) GetItems(listID uuid.UUID) ([]models.ListItem, error) {
	rows, err := r.db.Query(context.Background(),
		`SELECT id, list_id, product_id, name, quantity, COALESCE(unit,''), checked, COALESCE(notes,''), added_by, created_at, updated_at
		 FROM list_items WHERE list_id=$1 ORDER BY created_at`,
		listID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.ListItem
	for rows.Next() {
		var it models.ListItem
		if err := rows.Scan(&it.ID, &it.ListID, &it.ProductID, &it.Name, &it.Quantity,
			&it.Unit, &it.Checked, &it.Notes, &it.AddedBy, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

func (r *ListRepository) AddItem(it *models.ListItem) error {
	if it.ID == uuid.Nil {
		it.ID = uuid.New()
	}
	return r.db.QueryRow(context.Background(),
		`INSERT INTO list_items (id, list_id, product_id, name, quantity, unit, notes, added_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING created_at, updated_at`,
		it.ID, it.ListID, it.ProductID, it.Name, it.Quantity, it.Unit, it.Notes, it.AddedBy,
	).Scan(&it.CreatedAt, &it.UpdatedAt)
}

func (r *ListRepository) UpdateItem(it *models.ListItem) error {
	_, err := r.db.Exec(context.Background(),
		`UPDATE list_items SET name=$1, quantity=$2, unit=$3, checked=$4, notes=$5, updated_at=now()
		 WHERE id=$6 AND list_id=$7`,
		it.Name, it.Quantity, it.Unit, it.Checked, it.Notes, it.ID, it.ListID,
	)
	return err
}

func (r *ListRepository) DeleteItem(itemID, listID uuid.UUID) error {
	_, err := r.db.Exec(context.Background(),
		`DELETE FROM list_items WHERE id=$1 AND list_id=$2`, itemID, listID,
	)
	return err
}

func (r *ListRepository) TouchList(listID uuid.UUID) error {
	_, err := r.db.Exec(context.Background(),
		`UPDATE shopping_lists SET updated_at=now() WHERE id=$1`, listID,
	)
	return err
}
