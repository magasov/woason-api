package chat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	httpx "woason-api/internal/httpx"
	"woason-api/internal/models"
	"woason-api/internal/store"
	"woason-api/internal/ws"
)

type Handler struct {
	Store *store.Store
	Hub   *ws.Hub
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListThreads(r.Context(), httpx.MustPrincipal(r).ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить чаты")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) Messages(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	thread, err := h.threadFor(r.Context(), p, httpx.Param(r, "peerId"))
	if err != nil {
		httpx.Error(w, statusOf(err), err.Error())
		return
	}
	items, err := h.Store.ListMessages(r.Context(), thread.ID, false, 500, 0)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить сообщения")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) Send(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	var req struct {
		Text string `json:"text"`
	}
	if err := httpx.Decode(r, &req); err != nil || strings.TrimSpace(req.Text) == "" {
		httpx.Error(w, http.StatusBadRequest, "нужен текст")
		return
	}
	msg, sellerID, buyerID, err := h.SendMessage(r.Context(), p, httpx.Param(r, "peerId"), strings.TrimSpace(req.Text))
	if err != nil {
		httpx.Error(w, statusOf(err), err.Error())
		return
	}
	if h.Hub != nil {
		out := ws.Message{Type: "chat.message", PeerID: httpx.Param(r, "peerId"), Payload: msg}
		h.Hub.SendToUser(sellerID, out)
		h.Hub.SendToUser(buyerID, out)
	}
	httpx.WriteJSON(w, http.StatusCreated, msg)
}

func (h *Handler) Read(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	thread, err := h.threadFor(r.Context(), p, httpx.Param(r, "peerId"))
	if err != nil {
		httpx.Error(w, statusOf(err), err.Error())
		return
	}
	if err := h.Store.MarkRead(r.Context(), thread.ID, p.ID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось отметить прочитанным")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (h *Handler) SendMessage(ctx context.Context, user *models.Principal, peerID, text string) (*models.ChatMessage, string, string, error) {
	if user == nil || strings.TrimSpace(text) == "" {
		return nil, "", "", bad("нужен текст")
	}
	if user.ID == peerID {
		return nil, "", "", bad("нельзя писать самому себе")
	}
	sellerID, buyerID, err := h.roles(ctx, user, peerID)
	if err != nil {
		return nil, "", "", err
	}
	thread, err := h.Store.GetOrCreateThread(ctx, sellerID, buyerID)
	if err != nil {
		return nil, "", "", err
	}
	msg := &models.ChatMessage{
		ID:       fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		ThreadID: thread.ID,
		SellerID: sellerID,
		BuyerID:  buyerID,
		FromID:   user.ID,
		Text:     strings.TrimSpace(text),
		Read:     true,
	}
	if err := h.Store.InsertMessage(ctx, msg); err != nil {
		return nil, "", "", err
	}
	return msg, sellerID, buyerID, nil
}

func (h *Handler) roles(ctx context.Context, user *models.Principal, peerID string) (sellerID, buyerID string, err error) {
	shop, _ := h.Store.GetShop(ctx, user.ID)
	peerShop, _ := h.Store.GetShop(ctx, peerID)
	if shop != nil && user.Role != models.RoleBuyer {
		return user.ID, peerID, nil
	}
	if peerShop != nil {
		return peerID, user.ID, nil
	}
	return "", "", bad("чат доступен с магазином")
}

func (h *Handler) threadFor(ctx context.Context, p *models.Principal, peerID string) (*models.ChatThread, error) {
	if p.ID == peerID {
		return nil, bad("нельзя писать самому себе")
	}
	sellerID, buyerID, err := h.roles(ctx, p, peerID)
	if err != nil {
		return nil, err
	}
	return h.Store.GetOrCreateThread(ctx, sellerID, buyerID)
}

type httpErr struct {
	msg string
}

func (e httpErr) Error() string { return e.msg }

func bad(msg string) error { return httpErr{msg: msg} }

func statusOf(err error) int {
	var e httpErr
	if errors.As(err, &e) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
