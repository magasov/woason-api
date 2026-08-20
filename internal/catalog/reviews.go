package catalog

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"woason-api/internal/auth"
	httpx "woason-api/internal/httpx"
	"woason-api/internal/models"
	"woason-api/internal/store"
)

func (h *Handler) ProductReviews(w http.ResponseWriter, r *http.Request) {
	id := httpx.Param(r, "id")
	p, err := h.visibleProduct(r, id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	if p == nil {
		httpx.Error(w, http.StatusNotFound, "товар не найден")
		return
	}
	limit, offset := httpx.PageDefault(r, 50, 100)
	sort := r.URL.Query().Get("sort")
	items, total, err := h.Store.ListReviews(r.Context(), store.ReviewFilter{
		ProductID: p.ID,
		Sort:      sort,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		httpx.LogErr("list reviews", err)
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить отзывы")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) CreateReview(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	productID := httpx.Param(r, "id")
	var req struct {
		OrderID string   `json:"orderId"`
		Rating  int      `json:"rating"`
		Text    string   `json:"text"`
		Photos  []string `json:"photos"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if req.Rating < 1 || req.Rating > 5 {
		httpx.Error(w, http.StatusBadRequest, "оценка должна быть от 1 до 5")
		return
	}
	text := strings.TrimSpace(req.Text)
	n := utf8.RuneCountInString(text)
	if n < 8 || n > 2000 {
		httpx.Error(w, http.StatusBadRequest, "текст отзыва должен быть от 8 до 2000 символов")
		return
	}
	photos, err := normalizeReviewPhotos(req.Photos)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.OrderID) == "" {
		httpx.Error(w, http.StatusBadRequest, "нужен orderId")
		return
	}

	prod, err := h.Store.GetProduct(r.Context(), productID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	if prod == nil {
		httpx.Error(w, http.StatusNotFound, "товар не найден")
		return
	}

	order, err := h.Store.GetOrder(r.Context(), strings.TrimSpace(req.OrderID))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	if order == nil || order.BuyerID != p.ID {
		httpx.Error(w, http.StatusNotFound, "заказ не найден")
		return
	}
	hasItem := false
	for _, it := range order.Items {
		if it.ProductID == productID {
			hasItem = true
			break
		}
	}
	if !hasItem || order.Status != models.StatusDelivered {
		httpx.Error(w, http.StatusForbidden, "оставить отзыв можно только после получения заказа")
		return
	}
	if prod.SellerID == p.ID {
		httpx.Error(w, http.StatusForbidden, "нельзя оценить свой товар")
		return
	}

	exists, err := h.Store.HasReview(r.Context(), p.ID, productID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	if exists {
		httpx.Error(w, http.StatusConflict, "вы уже оставили отзыв на этот товар")
		return
	}

	u, err := h.Store.GetUserByID(r.Context(), p.ID)
	if err != nil || u == nil {
		httpx.Error(w, http.StatusUnauthorized, "нужна авторизация")
		return
	}
	rev := &models.Review{
		ProductID: productID,
		OrderID:   order.ID,
		UserID:    p.ID,
		Author:    u.Name,
		Rating:    req.Rating,
		Text:      text,
		Date:      time.Now().UTC().Format(time.RFC3339),
		Photos:    photos,
	}
	if err := h.Store.CreateReview(r.Context(), rev); err != nil {
		if errors.Is(err, store.ErrReviewExists) {
			httpx.Error(w, http.StatusConflict, "вы уже оставили отзыв на этот товар")
			return
		}
		httpx.LogErr("create review", err)
		httpx.Error(w, http.StatusInternalServerError, "не удалось сохранить отзыв")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, rev)
}

func (h *Handler) visibleProduct(r *http.Request, id string) (*models.Product, error) {
	p, err := h.Store.GetProduct(r.Context(), id)
	if err != nil {
		return nil, err
	}
	viewer := httpx.PrincipalFrom(r)
	if p == nil || (p.Hidden && !canSeeHidden(viewer, p.SellerID)) {
		return nil, nil
	}
	if !models.IsGoodsCategory(p.Category) && !canSeeHidden(viewer, p.SellerID) {
		return nil, nil
	}
	shop, err := h.Store.GetShop(r.Context(), p.SellerID)
	if err != nil {
		return nil, err
	}
	if shop != nil && shop.Hidden && !canSeeHidden(viewer, p.SellerID) {
		return nil, nil
	}
	return p, nil
}

func normalizeReviewPhotos(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, u := range in {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if !auth.ValidPhotoURL(u) {
			return nil, errors.New("некорректный URL фото")
		}
		out = append(out, u)
	}
	if len(out) > 4 {
		return nil, errors.New("можно прикрепить не больше 4 фото")
	}
	return out, nil
}
