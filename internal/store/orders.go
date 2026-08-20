package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"woason-api/internal/models"
)

func (s *Store) CreateOrder(ctx context.Context, o *models.Order) error {
	return s.Tx(ctx, func(tx pgx.Tx) error {
		var track any
		if o.TrackNumber != "" {
			track = o.TrackNumber
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO orders (id, created_at, buyer_id, seller_id, city, address, delivery,
				delivery_price_kopecks, eta, track_number, status, total_kopecks)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			o.ID, o.CreatedAt, o.BuyerID, o.SellerID, o.City, o.Address, o.Delivery,
			models.Kopecks(o.DeliveryPrice), o.ETA, track, o.Status, models.Kopecks(o.Total)); err != nil {
			return err
		}
		for _, it := range o.Items {
			if _, err := tx.Exec(ctx, `
				INSERT INTO order_items (order_id, product_id, title, price_kopecks, qty, image)
				VALUES ($1,$2,$3,$4,$5,$6)`, o.ID, it.ProductID, it.Title, models.Kopecks(it.Price), it.Qty, it.Image); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO order_events (order_id, status, note) VALUES ($1,$2,$3)`, o.ID, o.Status, "создан")
		return err
	})
}

func (s *Store) GetOrder(ctx context.Context, id string) (*models.Order, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, created_at, buyer_id, seller_id, city, address, delivery,
			delivery_price_kopecks, eta, COALESCE(track_number,''), status, total_kopecks
		FROM orders WHERE id=$1`, id)
	o, err := scanOrder(row)
	if err != nil || o == nil {
		return o, err
	}
	items, err := s.orderItems(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return o, nil
}

func scanOrder(row interface{ Scan(dest ...any) error }) (*models.Order, error) {
	var o models.Order
	var deliv, total int64
	err := row.Scan(&o.ID, &o.CreatedAt, &o.BuyerID, &o.SellerID, &o.City, &o.Address, &o.Delivery,
		&deliv, &o.ETA, &o.TrackNumber, &o.Status, &total)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	o.DeliveryPrice = models.Rub(deliv)
	o.Total = models.Rub(total)
	return &o, nil
}

func (s *Store) orderItems(ctx context.Context, orderID string) ([]models.OrderItem, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT product_id, title, price_kopecks, qty, image FROM order_items WHERE order_id=$1`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.OrderItem{}
	for rows.Next() {
		var it models.OrderItem
		var price int64
		if err := rows.Scan(&it.ProductID, &it.Title, &price, &it.Qty, &it.Image); err != nil {
			return nil, err
		}
		it.Price = models.Rub(price)
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Store) ListOrders(ctx context.Context, buyerID, sellerID string, limit, offset int) ([]models.Order, int, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, created_at, buyer_id, seller_id, city, address, delivery,
			delivery_price_kopecks, eta, COALESCE(track_number,''), status, total_kopecks,
			COUNT(*) OVER() AS total
		FROM orders
		WHERE ($1='' OR buyer_id=$1)
		  AND ($2='' OR seller_id=$2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`, buyerID, sellerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []models.Order
	var total int
	for rows.Next() {
		var o models.Order
		var deliv, tot int64
		if err := rows.Scan(&o.ID, &o.CreatedAt, &o.BuyerID, &o.SellerID, &o.City, &o.Address, &o.Delivery,
			&deliv, &o.ETA, &o.TrackNumber, &o.Status, &tot, &total); err != nil {
			return nil, 0, err
		}
		o.DeliveryPrice = models.Rub(deliv)
		o.Total = models.Rub(tot)
		its, err := s.orderItems(ctx, o.ID)
		if err != nil {
			return nil, 0, err
		}
		o.Items = its
		items = append(items, o)
	}
	if items == nil {
		items = []models.Order{}
	}
	return items, total, rows.Err()
}

func (s *Store) UpdateOrderStatus(ctx context.Context, id, status, track, note string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE orders SET
			status=$2,
			track_number=COALESCE(NULLIF($3,''), track_number)
		WHERE id=$1`, id, status, track)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO order_events (order_id, status, note) VALUES ($1,$2,$3)`, id, status, note)
	return err
}

func (s *Store) CreatePayment(ctx context.Context, p *models.Payment, raw []byte) error {
	id, err := uuid.Parse(p.ID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, amount_kopecks, status, confirmation_url, raw_json)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		id, p.OrderID, models.Kopecks(p.Amount), p.Status, nullStr(p.ConfirmationURL), raw)
	return err
}

func (s *Store) GetPayment(ctx context.Context, id string) (*models.Payment, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id::text, order_id, amount_kopecks, status, COALESCE(confirmation_url,'')
		FROM payments WHERE id=$1`, id)
	var p models.Payment
	var amount int64
	err := row.Scan(&p.ID, &p.OrderID, &amount, &p.Status, &p.ConfirmationURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Amount = models.Rub(amount)
	return &p, nil
}

func (s *Store) GetPaymentByOrder(ctx context.Context, orderID string) (*models.Payment, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id::text, order_id, amount_kopecks, status, COALESCE(confirmation_url,'')
		FROM payments WHERE order_id=$1 ORDER BY created_at DESC LIMIT 1`, orderID)
	var p models.Payment
	var amount int64
	err := row.Scan(&p.ID, &p.OrderID, &amount, &p.Status, &p.ConfirmationURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Amount = models.Rub(amount)
	return &p, nil
}

func (s *Store) UpdatePayment(ctx context.Context, id, status string, raw []byte) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE payments SET status=$2, raw_json=COALESCE($3, raw_json) WHERE id=$1`, id, status, raw)
	return err
}

func (s *Store) ListPayments(ctx context.Context, limit, offset int) ([]models.Payment, int, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, order_id, amount_kopecks, status, COALESCE(confirmation_url,''), COUNT(*) OVER()
		FROM payments ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []models.Payment
	var total int
	for rows.Next() {
		var p models.Payment
		var amount int64
		if err := rows.Scan(&p.ID, &p.OrderID, &amount, &p.Status, &p.ConfirmationURL, &total); err != nil {
			return nil, 0, err
		}
		p.Amount = models.Rub(amount)
		items = append(items, p)
	}
	if items == nil {
		items = []models.Payment{}
	}
	return items, total, rows.Err()
}

func NewOrderID() string {
	return fmt.Sprintf("WOA-%d", nowNano())
}
