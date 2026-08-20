package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"woason-api/internal/models"
)

func (s *Store) GetShop(ctx context.Context, id string) (*models.Shop, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, shop_name, description, logo, banner, city, phone, kind, delivery, hidden, created_at
		FROM shops WHERE id=$1`, id)
	return scanShop(row)
}

func scanShop(row pgx.Row) (*models.Shop, error) {
	var sh models.Shop
	err := row.Scan(&sh.ID, &sh.ShopName, &sh.Description, &sh.Logo, &sh.Banner, &sh.City, &sh.Phone, &sh.Kind, &sh.Delivery, &sh.Hidden, &sh.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sh.Delivery = models.Strs(sh.Delivery)
	return &sh, nil
}

func (s *Store) UpsertShop(ctx context.Context, sh *models.Shop) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO shops (id, shop_name, description, logo, banner, city, phone, kind, delivery)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE SET
			shop_name=EXCLUDED.shop_name,
			description=EXCLUDED.description,
			logo=EXCLUDED.logo,
			banner=EXCLUDED.banner,
			city=EXCLUDED.city,
			phone=EXCLUDED.phone,
			kind=EXCLUDED.kind,
			delivery=EXCLUDED.delivery`,
		sh.ID, sh.ShopName, sh.Description, sh.Logo, sh.Banner, sh.City, sh.Phone, sh.Kind, sh.Delivery)
	return err
}

func (s *Store) PatchShop(ctx context.Context, id string, shopName, description, logo, banner, city, phone *string, delivery []string, hidden *bool) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE shops SET
			shop_name=COALESCE($2, shop_name),
			description=COALESCE($3, description),
			logo=COALESCE($4, logo),
			banner=COALESCE($5, banner),
			city=COALESCE($6, city),
			phone=COALESCE($7, phone),
			delivery=COALESCE($8, delivery),
			hidden=COALESCE($9, hidden)
		WHERE id=$1`, id, shopName, description, logo, banner, city, phone, delivery, hidden)
	return err
}

func (s *Store) ListPublicShops(ctx context.Context, limit, offset int) ([]models.Shop, int, error) {
	return s.listShops(ctx, false, limit, offset)
}

func (s *Store) AdminListShops(ctx context.Context, limit, offset int) ([]models.Shop, int, error) {
	return s.listShops(ctx, true, limit, offset)
}

func (s *Store) listShops(ctx context.Context, all bool, limit, offset int) ([]models.Shop, int, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, shop_name, description, logo, banner, city, phone, kind, delivery, hidden, created_at,
		       COUNT(*) OVER() AS total
		FROM shops
		WHERE $1 OR hidden=false
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, all, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []models.Shop
	var total int
	for rows.Next() {
		var sh models.Shop
		if err := rows.Scan(&sh.ID, &sh.ShopName, &sh.Description, &sh.Logo, &sh.Banner, &sh.City, &sh.Phone, &sh.Kind, &sh.Delivery, &sh.Hidden, &sh.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		sh.Delivery = models.Strs(sh.Delivery)
		items = append(items, sh)
	}
	return items, total, rows.Err()
}

func (s *Store) ListStories(ctx context.Context, sellerID string) ([]models.Story, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, seller_id, image, caption, created_at
		FROM stories WHERE seller_id=$1 ORDER BY created_at DESC`, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.Story
	for rows.Next() {
		var st models.Story
		if err := rows.Scan(&st.ID, &st.SellerID, &st.Image, &st.Caption, &st.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, st)
	}
	if items == nil {
		items = []models.Story{}
	}
	return items, rows.Err()
}

func (s *Store) CreateStory(ctx context.Context, st models.Story) error {
	return s.upsertStory(ctx, st, false)
}

func (s *Store) UpsertStory(ctx context.Context, st models.Story) error {
	return s.upsertStory(ctx, st, true)
}

func (s *Store) upsertStory(ctx context.Context, st models.Story, upsert bool) error {
	sql := `
		INSERT INTO stories (id, seller_id, image, caption, created_at)
		VALUES ($1,$2,$3,$4,$5)`
	if upsert {
		sql += ` ON CONFLICT (id) DO UPDATE SET image=EXCLUDED.image, caption=EXCLUDED.caption, created_at=EXCLUDED.created_at`
	}
	_, err := s.Pool.Exec(ctx, sql, st.ID, st.SellerID, st.Image, st.Caption, st.CreatedAt)
	return err
}
