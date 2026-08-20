package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"woason-api/internal/models"
)

func (s *Store) ListReels(ctx context.Context, sellerID string, userID string, limit, offset int) ([]models.Reel, int, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT r.id, r.product_id, r.seller_id, s.shop_name, r.title, r.caption, r.likes, r.duration_sec, r.gradient,
		       COUNT(*) OVER() AS total
		FROM reels r
		JOIN shops s ON s.id=r.seller_id
		JOIN products p ON p.id=r.product_id
		WHERE ($1='' OR r.seller_id=$1)
		  AND s.hidden=false AND p.hidden=false
		  AND p.category = ANY($4::text[])
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3`, sellerID, limit, offset, models.GoodsSlugs())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []models.Reel
	var total int
	for rows.Next() {
		var r models.Reel
		if err := rows.Scan(&r.ID, &r.ProductID, &r.SellerID, &r.SellerName, &r.Title, &r.Caption, &r.Likes, &r.DurationSec, &r.Gradient, &total); err != nil {
			return nil, 0, err
		}
		r.Gradient = models.Strs(r.Gradient)
		r.Comments = []models.ReelComment{}
		items = append(items, r)
	}
	if items == nil {
		items = []models.Reel{}
	}
	for i := range items {
		comments, err := s.ListReelComments(ctx, items[i].ID)
		if err != nil {
			return nil, 0, err
		}
		items[i].Comments = comments
		if userID != "" {
			liked, _ := s.HasReelLike(ctx, userID, items[i].ID)
			items[i].Liked = liked
		}
	}
	return items, total, rows.Err()
}

func (s *Store) GetReel(ctx context.Context, id, userID string) (*models.Reel, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT r.id, r.product_id, r.seller_id, s.shop_name, r.title, r.caption, r.likes, r.duration_sec, r.gradient
		FROM reels r
		JOIN shops s ON s.id=r.seller_id
		WHERE r.id=$1`, id)
	var r models.Reel
	err := row.Scan(&r.ID, &r.ProductID, &r.SellerID, &r.SellerName, &r.Title, &r.Caption, &r.Likes, &r.DurationSec, &r.Gradient)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Gradient = models.Strs(r.Gradient)
	comments, err := s.ListReelComments(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	r.Comments = comments
	if userID != "" {
		r.Liked, _ = s.HasReelLike(ctx, userID, r.ID)
	}
	return &r, nil
}

func (s *Store) ListReelComments(ctx context.Context, reelID string) ([]models.ReelComment, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, author, text, created_at FROM reel_comments WHERE reel_id=$1 ORDER BY created_at ASC`, reelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.ReelComment{}
	for rows.Next() {
		var c models.ReelComment
		if err := rows.Scan(&c.ID, &c.Author, &c.Text, &c.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *Store) HasReelLike(ctx context.Context, userID, reelID string) (bool, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT COUNT(1) FROM reel_likes WHERE user_id=$1 AND reel_id=$2`, userID, reelID).Scan(&n)
	return n > 0, err
}

func (s *Store) ToggleReelLike(ctx context.Context, userID, reelID string) (*models.Reel, error) {
	liked, err := s.HasReelLike(ctx, userID, reelID)
	if err != nil {
		return nil, err
	}
	if liked {
		_, err = s.Pool.Exec(ctx, `DELETE FROM reel_likes WHERE user_id=$1 AND reel_id=$2`, userID, reelID)
		if err != nil {
			return nil, err
		}
		_, err = s.Pool.Exec(ctx, `UPDATE reels SET likes=GREATEST(likes-1,0) WHERE id=$1`, reelID)
	} else {
		_, err = s.Pool.Exec(ctx, `INSERT INTO reel_likes (user_id, reel_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, reelID)
		if err != nil {
			return nil, err
		}
		_, err = s.Pool.Exec(ctx, `UPDATE reels SET likes=likes+1 WHERE id=$1`, reelID)
	}
	if err != nil {
		return nil, err
	}
	return s.GetReel(ctx, reelID, userID)
}

func (s *Store) AddReelComment(ctx context.Context, reelID, author, text, createdAt string) (*models.ReelComment, error) {
	c := models.ReelComment{ID: newID("cm"), Author: author, Text: text, CreatedAt: createdAt}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO reel_comments (id, reel_id, author, text, created_at) VALUES ($1,$2,$3,$4,$5)`,
		c.ID, reelID, c.Author, c.Text, c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) CreateReel(ctx context.Context, r models.Reel) error {
	return s.upsertReel(ctx, r, false)
}

func (s *Store) UpsertReel(ctx context.Context, r models.Reel) error {
	return s.upsertReel(ctx, r, true)
}

func (s *Store) upsertReel(ctx context.Context, r models.Reel, upsert bool) error {
	sql := `
		INSERT INTO reels (id, product_id, seller_id, title, caption, likes, duration_sec, gradient)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	if upsert {
		sql += `
		ON CONFLICT (id) DO UPDATE SET
			product_id=EXCLUDED.product_id, title=EXCLUDED.title, caption=EXCLUDED.caption,
			duration_sec=EXCLUDED.duration_sec, gradient=EXCLUDED.gradient`
	}
	_, err := s.Pool.Exec(ctx, sql,
		r.ID, r.ProductID, r.SellerID, r.Title, r.Caption, r.Likes, r.DurationSec, r.Gradient)
	return err
}
