package catalog

import (
	"net/http"
	"strings"
	"time"

	httpx "woason-api/internal/httpx"
	"woason-api/internal/models"
	"woason-api/internal/store"
)

type Handler struct {
	Store *store.Store
}

func (h *Handler) Categories(w http.ResponseWriter, r *http.Request) {
	items := models.Categories
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) Products(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	q := r.URL.Query()
	cond := q.Get("condition")
	if cond == "all" {
		cond = ""
	}
	category := models.NormalizeCategory(q.Get("category"))
	if category != "" && !models.IsGoodsCategory(category) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": []models.Product{}, "total": 0})
		return
	}
	items, total, err := h.Store.ListProducts(r.Context(), store.ProductFilter{
		Category:  category,
		Condition: cond,
		Query:     q.Get("q"),
		City:      q.Get("city"),
		Sort:      q.Get("sort"),
		GoodsOnly: true,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		httpx.LogErr("list products", err)
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить товары")
		return
	}
	viewer := httpx.PrincipalFrom(r)
	for i := range items {
		items[i].TradeType = models.PublicTradeType(items[i].TradeType, viewer, items[i].SellerID)
		items[i].Hidden = false
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) Product(w http.ResponseWriter, r *http.Request) {
	id := httpx.Param(r, "id")
	p, err := h.Store.GetProduct(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	viewer := httpx.PrincipalFrom(r)
	if p == nil || (p.Hidden && !canSeeHidden(viewer, p.SellerID)) {
		httpx.Error(w, http.StatusNotFound, "товар не найден")
		return
	}
	if !models.IsGoodsCategory(p.Category) && !canSeeHidden(viewer, p.SellerID) {
		httpx.Error(w, http.StatusNotFound, "товар не найден")
		return
	}
	shop, _ := h.Store.GetShop(r.Context(), p.SellerID)
	if shop != nil && shop.Hidden && !canSeeHidden(viewer, p.SellerID) {
		httpx.Error(w, http.StatusNotFound, "товар не найден")
		return
	}
	p.TradeType = models.PublicTradeType(p.TradeType, viewer, p.SellerID)
	if !canSeeHidden(viewer, p.SellerID) {
		p.Hidden = false
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

func canSeeHidden(p *models.Principal, sellerID string) bool {
	if p == nil {
		return false
	}
	return p.Role == models.RoleAdmin || p.ID == sellerID
}

func (h *Handler) Reels(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	uid := ""
	if p := httpx.PrincipalFrom(r); p != nil {
		uid = p.ID
	}
	items, total, err := h.Store.ListReels(r.Context(), "", uid, limit, offset)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить рилсы")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) Reel(w http.ResponseWriter, r *http.Request) {
	uid := ""
	if p := httpx.PrincipalFrom(r); p != nil {
		uid = p.ID
	}
	reel, err := h.Store.GetReel(r.Context(), httpx.Param(r, "id"), uid)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	if reel == nil {
		httpx.Error(w, http.StatusNotFound, "рилс не найден")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, reel)
}

func (h *Handler) LikeReel(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	reel, err := h.Store.ToggleReelLike(r.Context(), p.ID, httpx.Param(r, "id"))
	if err != nil || reel == nil {
		httpx.Error(w, http.StatusNotFound, "рилс не найден")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, reel)
}

func (h *Handler) CommentReel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := httpx.Decode(r, &req); err != nil || strings.TrimSpace(req.Text) == "" {
		httpx.Error(w, http.StatusBadRequest, "нужен текст комментария")
		return
	}
	p := httpx.MustPrincipal(r)
	u, err := h.Store.GetUserByID(r.Context(), p.ID)
	if err != nil || u == nil {
		httpx.Error(w, http.StatusUnauthorized, "нужна авторизация")
		return
	}
	reel, err := h.Store.GetReel(r.Context(), httpx.Param(r, "id"), p.ID)
	if err != nil || reel == nil {
		httpx.Error(w, http.StatusNotFound, "рилс не найден")
		return
	}
	c, err := h.Store.AddReelComment(r.Context(), reel.ID, u.Name, strings.TrimSpace(req.Text),
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось сохранить комментарий")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, c)
}
