package store

import (
	"context"

	"woason-api/internal/models"
)

type AdminStats struct {
	Users          int `json:"users"`
	Sellers        int `json:"sellers"`
	Buyers         int `json:"buyers"`
	Admins         int `json:"admins"`
	Banned         int `json:"banned"`
	Shops          int `json:"shops"`
	HiddenShops    int `json:"hiddenShops"`
	Products       int `json:"products"`
	HiddenProducts int `json:"hiddenProducts"`
	Orders         int `json:"orders"`
	Payments       int `json:"payments"`
	GMV            int `json:"gmv"`
	Chats          int `json:"chats"`
}

func (s *Store) AdminStats(ctx context.Context) (AdminStats, error) {
	var st AdminStats
	var gmv int64
	err := s.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM users WHERE role='seller'),
			(SELECT COUNT(*) FROM users WHERE role='buyer'),
			(SELECT COUNT(*) FROM users WHERE role='admin'),
			(SELECT COUNT(*) FROM users WHERE banned_at IS NOT NULL),
			(SELECT COUNT(*) FROM shops),
			(SELECT COUNT(*) FROM shops WHERE hidden),
			(SELECT COUNT(*) FROM products),
			(SELECT COUNT(*) FROM products WHERE hidden),
			(SELECT COUNT(*) FROM orders),
			(SELECT COUNT(*) FROM payments),
			(SELECT COALESCE(SUM(total_kopecks),0) FROM orders WHERE status NOT IN ('cancelled','refunded')),
			(SELECT COUNT(*) FROM chat_threads)
	`).Scan(&st.Users, &st.Sellers, &st.Buyers, &st.Admins, &st.Banned,
		&st.Shops, &st.HiddenShops, &st.Products, &st.HiddenProducts,
		&st.Orders, &st.Payments, &gmv, &st.Chats)
	st.GMV = models.Rub(gmv)
	return st, err
}

func (s *Store) SellerDashboard(ctx context.Context, sellerID string) (map[string]any, error) {
	var products, orders, fresh, transit int
	var turnover, delivered int64
	err := s.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM products WHERE seller_id=$1),
			(SELECT COUNT(*) FROM orders WHERE seller_id=$1),
			(SELECT COUNT(*) FROM orders WHERE seller_id=$1 AND status IN ('placed','awaiting_payment','paid','awaiting_shipment')),
			(SELECT COUNT(*) FROM orders WHERE seller_id=$1 AND status IN ('label_printed','in_transit')),
			(SELECT COALESCE(SUM(total_kopecks),0) FROM orders WHERE seller_id=$1 AND status NOT IN ('cancelled','refunded')),
			(SELECT COALESCE(SUM(total_kopecks),0) FROM orders WHERE seller_id=$1 AND status='delivered')
	`, sellerID).Scan(&products, &orders, &fresh, &transit, &turnover, &delivered)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"productsCount": products,
		"ordersCount":   orders,
		"newOrders":     fresh,
		"inTransit":     transit,
		"turnover":      models.Rub(turnover),
		"deliveredSum":  models.Rub(delivered),
	}, nil
}

func (s *Store) BuyerDashboard(ctx context.Context, userID string) (models.BuyerDashboard, error) {
	var d models.BuyerDashboard
	var spent int64
	goods := models.GoodsSlugs()
	err := s.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM favorites f
			 JOIN products p ON p.id=f.product_id
			 WHERE f.user_id=$1 AND p.hidden=false AND p.category = ANY($2::text[])),
			(SELECT COALESCE(SUM(c.qty),0) FROM cart_items c
			 JOIN products p ON p.id=c.product_id
			 WHERE c.user_id=$1 AND p.hidden=false AND p.category = ANY($2::text[])),
			(SELECT COUNT(*) FROM orders WHERE buyer_id=$1),
			(SELECT COALESCE(SUM(total_kopecks),0) FROM orders
			 WHERE buyer_id=$1 AND status NOT IN ('cancelled','refunded','awaiting_payment','placed')),
			(SELECT COUNT(*) FROM orders WHERE buyer_id=$1 AND status IN ('label_printed','in_transit')),
			(SELECT COUNT(DISTINCT oi.product_id) FROM orders o
			 JOIN order_items oi ON oi.order_id=o.id
			 WHERE o.buyer_id=$1 AND o.status='delivered'
			   AND NOT EXISTS (
				 SELECT 1 FROM reviews r WHERE r.user_id=$1 AND r.product_id=oi.product_id
			   ))
	`, userID, goods).Scan(
		&d.FavoritesCount, &d.CartCount, &d.OrdersCount, &spent, &d.InTransit, &d.WaitingReviews)
	if err != nil {
		return d, err
	}
	d.Spent = models.Rub(spent)

	rows, err := s.Pool.Query(ctx, `
		SELECT to_char(d, 'YYYY-MM-DD'), COALESCE(SUM(o.total_kopecks), 0)
		FROM generate_series((CURRENT_DATE - INTERVAL '6 days')::date, CURRENT_DATE, interval '1 day') AS d
		LEFT JOIN orders o ON o.buyer_id=$1
		  AND o.created_at::date = d::date
		  AND o.status NOT IN ('cancelled','refunded','awaiting_payment','placed')
		GROUP BY d
		ORDER BY d`, userID)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	week := make([]models.WeekPoint, 0, 7)
	for rows.Next() {
		var p models.WeekPoint
		var sum int64
		if err := rows.Scan(&p.Date, &sum); err != nil {
			return d, err
		}
		p.Sum = models.Rub(sum)
		week = append(week, p)
	}
	d.Week = week
	return d, rows.Err()
}
