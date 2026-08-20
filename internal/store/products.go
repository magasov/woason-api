package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"woason-api/internal/models"
)

func fillPrices(p *models.Product, price int64, oldPtr *int64) {
	p.Price = models.Rub(price)
	if oldPtr != nil {
		v := models.Rub(*oldPtr)
		p.OldPrice = &v
	}
	p.Images = models.Strs(p.Images)
	p.Delivery = models.Strs(p.Delivery)
	p.Tags = models.Strs(p.Tags)
}

func scanProductRow(row interface{ Scan(dest ...any) error }, extra ...any) (*models.Product, error) {
	var p models.Product
	var price int64
	var oldPtr *int64
	dest := []any{
		&p.ID, &p.Title, &p.Description, &price, &oldPtr, &p.Rating, &p.ReviewsCount,
		&p.SellerKind, &p.Condition, &p.Category, &p.Image, &p.Images, &p.SellerID, &p.SellerName,
		&p.City, &p.WeightKg, &p.InStock, &p.Delivery, &p.Tags, &p.TradeType, &p.Hidden, &p.CreatedAt,
	}
	dest = append(dest, extra...)
	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	fillPrices(&p, price, oldPtr)
	return &p, nil
}

type ProductFilter struct {
	Category      string
	Condition     string
	Query         string
	City          string
	Sort          string
	SellerID      string
	IncludeHidden bool
	GoodsOnly     bool
	Limit         int
	Offset        int
}

func (s *Store) ListProducts(ctx context.Context, f ProductFilter) ([]models.Product, int, error) {
	sort := "p.created_at DESC"
	switch f.Sort {
	case "price_asc", "price":
		sort = "p.price_kopecks ASC"
	case "price_desc":
		sort = "p.price_kopecks DESC"
	case "rating":
		sort = "p.rating DESC, p.reviews_count DESC"
	case "new":
		sort = "p.created_at DESC"
	}
	q := strings.TrimSpace(f.Query)
	allowed := []string{}
	if f.GoodsOnly {
		allowed = models.GoodsSlugs()
	}
	sql := fmt.Sprintf(`
		SELECT p.id, p.title, p.description, p.price_kopecks, p.old_price_kopecks, p.rating, p.reviews_count,
			p.seller_kind, p.condition, p.category, p.image, p.images, p.seller_id, s.shop_name,
			p.city, p.weight_kg, p.in_stock, p.delivery, p.tags, p.trade_type, p.hidden, p.created_at,
			COUNT(*) OVER() AS total
		FROM products p
		JOIN shops s ON s.id=p.seller_id
		WHERE ($1 OR (p.hidden=false AND s.hidden=false))
		  AND ($2='' OR p.category=$2)
		  AND ($3='' OR p.condition=$3)
		  AND ($4='' OR p.city ILIKE $5)
		  AND ($6='' OR p.seller_id=$6)
		  AND ($7='' OR p.title ILIKE $8 OR p.description ILIKE $8 OR s.shop_name ILIKE $8
		       OR p.city ILIKE $8 OR EXISTS (SELECT 1 FROM unnest(p.tags) t WHERE t ILIKE $8))
		  AND (COALESCE(cardinality($11::text[]), 0)=0 OR p.category = ANY($11::text[]))
		ORDER BY %s
		LIMIT $9 OFFSET $10`, sort)

	rows, err := s.Pool.Query(ctx, sql,
		f.IncludeHidden, f.Category, f.Condition, f.City, like(f.City), f.SellerID,
		q, like(q), f.Limit, f.Offset, allowed)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []models.Product
	var total int
	for rows.Next() {
		p, err := scanProductRow(rows, &total)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *p)
	}
	if items == nil {
		items = []models.Product{}
	}
	return items, total, rows.Err()
}

func (s *Store) GetProduct(ctx context.Context, id string) (*models.Product, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT p.id, p.title, p.description, p.price_kopecks, p.old_price_kopecks, p.rating, p.reviews_count,
			p.seller_kind, p.condition, p.category, p.image, p.images, p.seller_id, s.shop_name,
			p.city, p.weight_kg, p.in_stock, p.delivery, p.tags, p.trade_type, p.hidden, p.created_at
		FROM products p
		JOIN shops s ON s.id=p.seller_id
		WHERE p.id=$1`, id)
	p, err := scanProductRow(row)
	if err != nil || p == nil {
		return p, err
	}
	revs, _, err := s.ListReviews(ctx, ReviewFilter{ProductID: p.ID, Sort: "new", Limit: 100})
	if err != nil {
		return nil, err
	}
	p.Reviews = revs
	return p, nil
}

func (s *Store) CreateProduct(ctx context.Context, p *models.Product) error {
	return s.upsertProduct(ctx, p, false)
}

func (s *Store) UpsertProduct(ctx context.Context, p *models.Product) error {
	return s.upsertProduct(ctx, p, true)
}

func (s *Store) upsertProduct(ctx context.Context, p *models.Product, upsert bool) error {
	var old any
	if p.OldPrice != nil {
		old = models.Kopecks(*p.OldPrice)
	}
	sql := `
		INSERT INTO products (
			id, title, description, price_kopecks, old_price_kopecks, rating, reviews_count,
			seller_kind, condition, category, image, images, seller_id, city, weight_kg,
			in_stock, delivery, tags, trade_type
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`
	if upsert {
		sql += `
		ON CONFLICT (id) DO UPDATE SET
			title=EXCLUDED.title,
			description=EXCLUDED.description,
			price_kopecks=EXCLUDED.price_kopecks,
			old_price_kopecks=EXCLUDED.old_price_kopecks,
			rating=EXCLUDED.rating,
			reviews_count=EXCLUDED.reviews_count,
			seller_kind=EXCLUDED.seller_kind,
			condition=EXCLUDED.condition,
			category=EXCLUDED.category,
			image=EXCLUDED.image,
			images=EXCLUDED.images,
			city=EXCLUDED.city,
			weight_kg=EXCLUDED.weight_kg,
			in_stock=EXCLUDED.in_stock,
			delivery=EXCLUDED.delivery,
			tags=EXCLUDED.tags,
			trade_type=EXCLUDED.trade_type`
	}
	_, err := s.Pool.Exec(ctx, sql,
		p.ID, p.Title, p.Description, models.Kopecks(p.Price), old, p.Rating, p.ReviewsCount,
		p.SellerKind, p.Condition, p.Category, p.Image, models.Strs(p.Images), p.SellerID, p.City, p.WeightKg,
		p.InStock, models.Strs(p.Delivery), models.Strs(p.Tags), p.TradeType)
	return err
}

func (s *Store) AdminPatchProduct(ctx context.Context, id string, title *string, price *int, hidden *bool) error {
	var priceK any
	if price != nil {
		priceK = models.Kopecks(*price)
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE products SET
			title=COALESCE($2, title),
			price_kopecks=COALESCE($3, price_kopecks),
			hidden=COALESCE($4, hidden)
		WHERE id=$1`, id, title, priceK, hidden)
	return err
}

func (s *Store) DeleteProduct(ctx context.Context, id string) error {
	ct, err := s.Pool.Exec(ctx, `DELETE FROM products WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
