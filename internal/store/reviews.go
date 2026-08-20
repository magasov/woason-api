package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"woason-api/internal/models"
)

var ErrReviewExists = errors.New("review exists")

type ReviewFilter struct {
	ProductID string
	UserID    string
	SellerID  string
	Sort      string
	Limit     int
	Offset    int
}

const reviewSelect = `
	r.id, r.product_id, COALESCE(r.order_id,''), COALESCE(r.user_id,''), r.author, r.rating, r.text, r.date,
	COALESCE(r.photos, '{}'::text[]), COALESCE(r.seller_reply,''), r.seller_reply_at, p.title, p.image`

func scanReview(row interface{ Scan(dest ...any) error }, extra ...any) (*models.Review, error) {
	var r models.Review
	var photos []string
	var replyAt *time.Time
	dest := []any{
		&r.ID, &r.ProductID, &r.OrderID, &r.UserID, &r.Author, &r.Rating, &r.Text, &r.Date,
		&photos, &r.SellerReply, &replyAt, &r.ProductTitle, &r.ProductImage,
	}
	dest = append(dest, extra...)
	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if len(photos) > 0 {
		r.Photos = photos
	}
	if replyAt != nil {
		r.SellerReplyAt = replyAt.UTC().Format(time.RFC3339)
	}
	return &r, nil
}

func reviewOrderSQL(sort string) string {
	switch sort {
	case "high":
		return "r.rating DESC, r.date DESC"
	case "low":
		return "r.rating ASC, r.date DESC"
	default:
		return "r.date DESC"
	}
}

func (s *Store) ListReviews(ctx context.Context, f ReviewFilter) ([]models.Review, int, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	sql := fmt.Sprintf(`
		SELECT %s, COUNT(*) OVER() AS total
		FROM reviews r
		JOIN products p ON p.id=r.product_id
		WHERE ($1='' OR r.product_id=$1)
		  AND ($2='' OR r.user_id=$2)
		  AND ($3='' OR p.seller_id=$3)
		ORDER BY %s
		LIMIT $4 OFFSET $5`, reviewSelect, reviewOrderSQL(f.Sort))
	rows, err := s.Pool.Query(ctx, sql, f.ProductID, f.UserID, f.SellerID, limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []models.Review{}
	var total int
	for rows.Next() {
		rev, err := scanReview(rows, &total)
		if err != nil {
			return nil, 0, err
		}
		if f.UserID == "" && f.SellerID == "" {
			rev.ProductTitle = ""
			rev.ProductImage = ""
		}
		items = append(items, *rev)
	}
	return items, total, rows.Err()
}

func (s *Store) GetReview(ctx context.Context, id string) (*models.Review, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT `+reviewSelect+`
		FROM reviews r
		JOIN products p ON p.id=r.product_id
		WHERE r.id=$1`, id)
	return scanReview(row)
}

func (s *Store) HasReview(ctx context.Context, userID, productID string) (bool, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM reviews WHERE user_id=$1 AND product_id=$2`, userID, productID).Scan(&n)
	return n > 0, err
}

func (s *Store) CreateReview(ctx context.Context, rev *models.Review) error {
	if rev.ID == "" {
		rev.ID = newID("rev")
	}
	photos := models.Strs(rev.Photos)
	err := s.Tx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO reviews (id, product_id, order_id, user_id, author, rating, text, date, photos)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			rev.ID, rev.ProductID, rev.OrderID, rev.UserID, rev.Author, rev.Rating, rev.Text, rev.Date, photos)
		if err != nil {
			return err
		}
		return recalcProductRatingTx(ctx, tx, rev.ProductID)
	})
	if isUniqueViolation(err) {
		return ErrReviewExists
	}
	return err
}

func (s *Store) DeleteReview(ctx context.Context, id string) error {
	var productID string
	err := s.Tx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `DELETE FROM reviews WHERE id=$1 RETURNING product_id`, id).Scan(&productID)
		if err != nil {
			return err
		}
		return recalcProductRatingTx(ctx, tx, productID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func (s *Store) ReplyToReview(ctx context.Context, id, sellerID, text string) (*models.Review, error) {
	now := time.Now().UTC()
	ct, err := s.Pool.Exec(ctx, `
		UPDATE reviews r SET seller_reply=$3, seller_reply_at=$4
		FROM products p
		WHERE r.id=$1 AND r.product_id=p.id AND p.seller_id=$2`, id, sellerID, text, now)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	return s.GetReview(ctx, id)
}

func (s *Store) RecalcProductRating(ctx context.Context, productID string) error {
	return s.Tx(ctx, func(tx pgx.Tx) error {
		return recalcProductRatingTx(ctx, tx, productID)
	})
}

func recalcProductRatingTx(ctx context.Context, tx pgx.Tx, productID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE products SET
			reviews_count = (SELECT COUNT(*) FROM reviews WHERE product_id=$1),
			rating = COALESCE((SELECT ROUND(AVG(rating)::numeric, 1) FROM reviews WHERE product_id=$1), 0)
		WHERE id=$1`, productID)
	return err
}

func (s *Store) ListPendingReviews(ctx context.Context, userID string, limit, offset int) ([]models.PendingReview, int, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT t.order_id, t.product_id, t.title, t.price_kopecks, t.image, t.delivered_at,
		       COUNT(*) OVER() AS total
		FROM (
			SELECT DISTINCT ON (oi.product_id)
				o.id AS order_id,
				oi.product_id,
				oi.title,
				oi.price_kopecks,
				oi.image,
				COALESCE(
					(SELECT MAX(e.created_at) FROM order_events e WHERE e.order_id=o.id AND e.status='delivered'),
					o.created_at
				) AS delivered_at
			FROM orders o
			JOIN order_items oi ON oi.order_id=o.id
			WHERE o.buyer_id=$1 AND o.status='delivered'
			  AND NOT EXISTS (
				SELECT 1 FROM reviews r WHERE r.user_id=$1 AND r.product_id=oi.product_id
			  )
			ORDER BY oi.product_id, delivered_at DESC
		) t
		ORDER BY t.delivered_at DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []models.PendingReview{}
	var total int
	for rows.Next() {
		var it models.PendingReview
		var price int64
		var delivered time.Time
		if err := rows.Scan(&it.OrderID, &it.ProductID, &it.Title, &price, &it.Image, &delivered, &total); err != nil {
			return nil, 0, err
		}
		it.Price = models.Rub(price)
		it.DeliveredAt = delivered.UTC().Format(time.RFC3339)
		items = append(items, it)
	}
	return items, total, rows.Err()
}

func (s *Store) UpsertReview(ctx context.Context, productID string, r models.Review) error {
	photos := models.Strs(r.Photos)
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO reviews (id, product_id, order_id, user_id, author, rating, text, date, photos, seller_reply)
		VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET
			author=EXCLUDED.author,
			rating=EXCLUDED.rating,
			text=EXCLUDED.text,
			date=EXCLUDED.date,
			photos=EXCLUDED.photos,
			seller_reply=EXCLUDED.seller_reply`,
		r.ID, productID, r.OrderID, r.UserID, r.Author, r.Rating, r.Text, r.Date, photos, r.SellerReply)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
