package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"woason-api/internal/models"
)

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return scanUser(s.Pool.QueryRow(ctx, `
		SELECT id, name, email, phone, avatar, password_hash, role, banned_at, created_at
		FROM users WHERE lower(email)=lower($1)`, email))
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	u, err := scanUser(s.Pool.QueryRow(ctx, `
		SELECT id, name, email, phone, avatar, password_hash, role, banned_at, created_at
		FROM users WHERE id=$1`, id))
	if err != nil {
		return nil, err
	}
	if u.Role == models.RoleSeller {
		shop, err := s.GetShop(ctx, u.ID)
		if err == nil {
			u.Seller = shop
		}
	}
	return u, nil
}

func scanUser(row pgx.Row) (*models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.Avatar, &u.PasswordHash, &u.Role, &u.BannedAt, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) CreateUser(ctx context.Context, u *models.User) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, phone, password_hash, role)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		u.ID, u.Name, u.Email, u.Phone, u.PasswordHash, u.Role)
	return err
}

func (s *Store) SaveRefresh(ctx context.Context, userID, hash string, exp time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1,$2,$3,$4)`, uuid.New(), userID, hash, exp)
	return err
}

func (s *Store) ConsumeRefresh(ctx context.Context, hash string) (string, error) {
	var userID string
	err := s.Pool.QueryRow(ctx, `
		UPDATE refresh_tokens
		SET revoked=true
		WHERE token_hash=$1 AND revoked=false AND expires_at>now()
		RETURNING user_id`, hash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return userID, err
}

func (s *Store) RevokeRefresh(ctx context.Context, hash string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE refresh_tokens SET revoked=true WHERE token_hash=$1`, hash)
	return err
}

func (s *Store) AdminListUsers(ctx context.Context, q, role string, banned *bool, limit, offset int) ([]models.User, int, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, name, email, phone, avatar, password_hash, role, banned_at, created_at,
		       COUNT(*) OVER() AS total
		FROM users
		WHERE ($1='' OR name ILIKE $2 OR email ILIKE $2)
		  AND ($3='' OR role=$3)
		  AND ($4::boolean IS NULL OR ($4=true AND banned_at IS NOT NULL) OR ($4=false AND banned_at IS NULL))
		ORDER BY created_at DESC
		LIMIT $5 OFFSET $6`, q, like(q), role, banned, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []models.User
	var total int
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.Avatar, &u.PasswordHash, &u.Role, &u.BannedAt, &u.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		u.PasswordHash = ""
		items = append(items, u)
	}
	return items, total, rows.Err()
}

func (s *Store) AdminPatchUser(ctx context.Context, id string, name, phone, role *string, banned *bool) (*models.User, error) {
	_, err := s.Pool.Exec(ctx, `
		UPDATE users SET
			name=COALESCE($2, name),
			phone=COALESCE($3, phone),
			role=COALESCE($4, role),
			banned_at=CASE
				WHEN $5::boolean IS NULL THEN banned_at
				WHEN $5 THEN COALESCE(banned_at, now())
				ELSE NULL
			END
		WHERE id=$1`, id, name, phone, role, banned)
	if err != nil {
		return nil, err
	}
	return s.GetUserByID(ctx, id)
}

func (s *Store) PatchMe(ctx context.Context, id string, name, phone, avatar *string) (*models.User, error) {
	_, err := s.Pool.Exec(ctx, `
		UPDATE users SET
			name=COALESCE($2, name),
			phone=COALESCE($3, phone),
			avatar=COALESCE($4, avatar)
		WHERE id=$1`, id, name, phone, avatar)
	if err != nil {
		return nil, err
	}
	return s.GetUserByID(ctx, id)
}
