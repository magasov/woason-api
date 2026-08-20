package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	"woason-api/internal/models"
)

func (s *Store) GetCart(ctx context.Context, userID string) ([]models.CartItem, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT c.product_id, c.qty
		FROM cart_items c
		JOIN products p ON p.id=c.product_id
		JOIN shops s ON s.id=p.seller_id
		WHERE c.user_id=$1 AND p.hidden=false AND s.hidden=false
		  AND p.category = ANY($2::text[])
		ORDER BY c.product_id`, userID, models.GoodsSlugs())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.CartItem{}
	for rows.Next() {
		var it models.CartItem
		if err := rows.Scan(&it.ProductID, &it.Qty); err != nil {
			return nil, err
		}
		p, err := s.GetProduct(ctx, it.ProductID)
		if err != nil {
			return nil, err
		}
		if p != nil && models.IsGoodsCategory(p.Category) {
			it.Product = p
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Store) AddCart(ctx context.Context, userID, productID string, qty int) error {
	if qty <= 0 {
		qty = 1
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO cart_items (user_id, product_id, qty) VALUES ($1,$2,$3)
		ON CONFLICT (user_id, product_id) DO UPDATE SET qty=cart_items.qty+EXCLUDED.qty`,
		userID, productID, qty)
	return err
}

func (s *Store) SetCartQty(ctx context.Context, userID, productID string, qty int) error {
	if qty <= 0 {
		_, err := s.Pool.Exec(ctx, `DELETE FROM cart_items WHERE user_id=$1 AND product_id=$2`, userID, productID)
		return err
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO cart_items (user_id, product_id, qty) VALUES ($1,$2,$3)
		ON CONFLICT (user_id, product_id) DO UPDATE SET qty=EXCLUDED.qty`, userID, productID, qty)
	return err
}

func (s *Store) ClearCart(ctx context.Context, userID string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM cart_items WHERE user_id=$1`, userID)
	return err
}

func (s *Store) PutCart(ctx context.Context, userID string, items []models.CartItem) error {
	return s.Tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM cart_items WHERE user_id=$1`, userID); err != nil {
			return err
		}
		for _, it := range items {
			if it.Qty <= 0 {
				continue
			}
			if _, err := tx.Exec(ctx, `INSERT INTO cart_items (user_id, product_id, qty) VALUES ($1,$2,$3)`, userID, it.ProductID, it.Qty); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ListFavorites(ctx context.Context, userID string) ([]models.Product, error) {
	rows, err := s.Pool.Query(ctx, `SELECT product_id FROM favorites WHERE user_id=$1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.Product{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		p, err := s.GetProduct(ctx, id)
		if err != nil {
			return nil, err
		}
		if p != nil && !p.Hidden && models.IsGoodsCategory(p.Category) {
			items = append(items, *p)
		}
	}
	return items, rows.Err()
}

func (s *Store) AddFavorite(ctx context.Context, userID, productID string) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO favorites (user_id, product_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, productID)
	return err
}

func (s *Store) DeleteFavorite(ctx context.Context, userID, productID string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM favorites WHERE user_id=$1 AND product_id=$2`, userID, productID)
	return err
}
