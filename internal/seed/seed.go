package seed

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"woason-api/internal/config"
	"woason-api/internal/models"
	"woason-api/internal/store"
)

func Run(ctx context.Context, st *store.Store, cfg config.Config) error {
	if err := purgeDemoCatalog(ctx, st); err != nil {
		return err
	}
	return ensureAdmin(ctx, st, cfg)
}

// Старые демо-магазины сида: при старте вычищаем, если остались в БД.
var demoSellerIDs = []string{"seller-moda", "seller-tech", "seller-home", "seller-private"}
var demoSellerEmails = []string{
	"shop-moda@woason.ru", "shop-tech@woason.ru", "shop-home@woason.ru", "shop-private@woason.ru",
}

func purgeDemoCatalog(ctx context.Context, st *store.Store) error {
	_, err := st.Pool.Exec(ctx, `
		WITH demo AS (
			SELECT id FROM users
			WHERE id = ANY($1) OR lower(email) = ANY($2)
		)
		DELETE FROM orders
		WHERE seller_id IN (SELECT id FROM demo)
		   OR buyer_id IN (SELECT id FROM demo)
		   OR id IN (
				SELECT oi.order_id FROM order_items oi
				WHERE oi.product_id LIKE 'p-seed-%'
				   OR oi.product_id IN (SELECT p.id FROM products p WHERE p.seller_id IN (SELECT id FROM demo))
		   )`, demoSellerIDs, demoSellerEmails)
	if err != nil {
		return fmt.Errorf("purge demo orders: %w", err)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM products WHERE id LIKE 'p-seed-%'`); err != nil {
		return fmt.Errorf("purge demo products: %w", err)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM stories WHERE id LIKE 'st-seed-%'`); err != nil {
		return fmt.Errorf("purge demo stories: %w", err)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM reels WHERE id LIKE 'reel-seed-%'`); err != nil {
		return fmt.Errorf("purge demo reels: %w", err)
	}
	if _, err := st.Pool.Exec(ctx, `
		DELETE FROM users WHERE id = ANY($1) OR lower(email) = ANY($2)`, demoSellerIDs, demoSellerEmails); err != nil {
		return fmt.Errorf("purge demo users: %w", err)
	}
	return nil
}

func ensureAdmin(ctx context.Context, st *store.Store, cfg config.Config) error {
	email := strings.TrimSpace(strings.ToLower(cfg.AdminEmail))
	if email == "" || cfg.AdminPassword == "" {
		return nil
	}
	existing, err := st.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u := &models.User{
		ID:           "admin-woason",
		Name:         "Админ",
		Email:        email,
		Phone:        "",
		Role:         models.RoleAdmin,
		PasswordHash: string(hash),
	}
	if err := st.CreateUser(ctx, u); err != nil {
		return fmt.Errorf("admin: %w", err)
	}
	return nil
}
