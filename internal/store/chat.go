package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"woason-api/internal/models"
)

func (s *Store) GetOrCreateThread(ctx context.Context, sellerID, buyerID string) (*models.ChatThread, error) {
	var t models.ChatThread
	err := s.Pool.QueryRow(ctx, `
		SELECT id, seller_id, buyer_id FROM chat_threads WHERE seller_id=$1 AND buyer_id=$2`,
		sellerID, buyerID).Scan(&t.ID, &t.SellerID, &t.BuyerID)
	if err == nil {
		return &t, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	t = models.ChatThread{ID: newID("th"), SellerID: sellerID, BuyerID: buyerID}
	_, err = s.Pool.Exec(ctx, `INSERT INTO chat_threads (id, seller_id, buyer_id) VALUES ($1,$2,$3)`, t.ID, t.SellerID, t.BuyerID)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) GetThreadByID(ctx context.Context, id string) (*models.ChatThread, error) {
	var t models.ChatThread
	err := s.Pool.QueryRow(ctx, `SELECT id, seller_id, buyer_id FROM chat_threads WHERE id=$1`, id).
		Scan(&t.ID, &t.SellerID, &t.BuyerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) ListThreads(ctx context.Context, userID string) ([]models.ChatThread, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT t.id, t.seller_id, t.buyer_id,
			COALESCE((SELECT text FROM chat_messages m WHERE m.thread_id=t.id AND m.hidden=false ORDER BY created_at DESC LIMIT 1),''),
			COALESCE((SELECT created_at FROM chat_messages m WHERE m.thread_id=t.id ORDER BY created_at DESC LIMIT 1), now()),
			(SELECT COUNT(*) FROM chat_messages m WHERE m.thread_id=t.id AND m.hidden=false AND m.read=false AND m.from_id<>$1)
		FROM chat_threads t
		WHERE t.seller_id=$1 OR t.buyer_id=$1
		ORDER BY 5 DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.ChatThread{}
	for rows.Next() {
		var t models.ChatThread
		if err := rows.Scan(&t.ID, &t.SellerID, &t.BuyerID, &t.LastText, &t.LastAt, &t.Unread); err != nil {
			return nil, err
		}
		if t.SellerID == userID {
			t.PeerID = t.BuyerID
		} else {
			t.PeerID = t.SellerID
		}
		name := t.PeerID
		if shop, _ := s.GetShop(ctx, t.PeerID); shop != nil {
			name = shop.ShopName
		} else if u, _ := s.GetUserByID(ctx, t.PeerID); u != nil {
			name = u.Name
		}
		t.PeerName = name
		items = append(items, t)
	}
	return items, rows.Err()
}

func (s *Store) ListMessages(ctx context.Context, threadID string, includeHidden bool, limit, offset int) ([]models.ChatMessage, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT m.id, m.thread_id, t.seller_id, t.buyer_id, m.from_id, m.text, m.read, m.hidden, m.created_at
		FROM chat_messages m
		JOIN chat_threads t ON t.id=m.thread_id
		WHERE m.thread_id=$1 AND ($2 OR m.hidden=false)
		ORDER BY m.created_at ASC
		LIMIT $3 OFFSET $4`, threadID, includeHidden, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.ChatMessage{}
	for rows.Next() {
		var m models.ChatMessage
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.SellerID, &m.BuyerID, &m.FromID, &m.Text, &m.Read, &m.Hidden, &m.CreatedAt); err != nil {
			return nil, err
		}
		if !includeHidden {
			m.Hidden = false
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (s *Store) InsertMessage(ctx context.Context, m *models.ChatMessage) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO chat_messages (id, thread_id, from_id, text, read, hidden)
		VALUES ($1,$2,$3,$4,$5,false)`, m.ID, m.ThreadID, m.FromID, m.Text, m.Read)
	if err != nil {
		return err
	}
	return s.Pool.QueryRow(ctx, `SELECT created_at FROM chat_messages WHERE id=$1`, m.ID).Scan(&m.CreatedAt)
}

func (s *Store) MarkRead(ctx context.Context, threadID, readerID string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE chat_messages SET read=true
		WHERE thread_id=$1 AND from_id<>$2 AND read=false`, threadID, readerID)
	return err
}

func (s *Store) HideMessage(ctx context.Context, id string) error {
	ct, err := s.Pool.Exec(ctx, `UPDATE chat_messages SET hidden=true WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) AdminListThreads(ctx context.Context, limit, offset int) ([]models.ChatThread, int, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT t.id, t.seller_id, t.buyer_id,
			COALESCE((SELECT text FROM chat_messages m WHERE m.thread_id=t.id ORDER BY created_at DESC LIMIT 1),''),
			COALESCE((SELECT created_at FROM chat_messages m WHERE m.thread_id=t.id ORDER BY created_at DESC LIMIT 1), now()),
			COUNT(*) OVER()
		FROM chat_threads t
		ORDER BY 5 DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []models.ChatThread
	var total int
	for rows.Next() {
		var t models.ChatThread
		if err := rows.Scan(&t.ID, &t.SellerID, &t.BuyerID, &t.LastText, &t.LastAt, &total); err != nil {
			return nil, 0, err
		}
		t.PeerID = t.BuyerID
		items = append(items, t)
	}
	if items == nil {
		items = []models.ChatThread{}
	}
	return items, total, rows.Err()
}
