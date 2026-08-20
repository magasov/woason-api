package admin

import (
	"net/http"

	"woason-api/internal/config"
	httpx "woason-api/internal/httpx"
	"woason-api/internal/models"
	"woason-api/internal/payment"
	"woason-api/internal/store"
	"woason-api/internal/ws"
)

type Handler struct {
	Store  *store.Store
	Pay    *payment.Client
	Hub    *ws.Hub
	Config config.Config
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	st, err := h.Store.AdminStats(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось получить статистику")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, st)
}

func (h *Handler) Users(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	q := r.URL.Query()
	var banned *bool
	if v := q.Get("banned"); v == "true" || v == "false" {
		b := v == "true"
		banned = &b
	}
	items, total, err := h.Store.AdminListUsers(r.Context(), q.Get("q"), q.Get("role"), banned, limit, offset)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить пользователей")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) PatchUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Banned *bool   `json:"banned"`
		Role   *string `json:"role"`
		Name   *string `json:"name"`
		Phone  *string `json:"phone"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if req.Role != nil {
		switch *req.Role {
		case models.RoleBuyer, models.RoleSeller, models.RoleAdmin:
		default:
			httpx.Error(w, http.StatusBadRequest, "неизвестная роль")
			return
		}
	}
	u, err := h.Store.AdminPatchUser(r.Context(), httpx.Param(r, "id"), req.Name, req.Phone, req.Role, req.Banned)
	if err != nil || u == nil {
		httpx.Error(w, http.StatusNotFound, "пользователь не найден")
		return
	}
	u.PasswordHash = ""
	httpx.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) Shops(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	items, total, err := h.Store.AdminListShops(r.Context(), limit, offset)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить магазины")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) PatchShop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hidden      *bool   `json:"hidden"`
		ShopName    *string `json:"shopName"`
		Description *string `json:"description"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if err := h.Store.PatchShop(r.Context(), httpx.Param(r, "id"), req.ShopName, req.Description, nil, nil, nil, nil, nil, req.Hidden); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось сохранить")
		return
	}
	sh, err := h.Store.GetShop(r.Context(), httpx.Param(r, "id"))
	if err != nil || sh == nil {
		httpx.Error(w, http.StatusNotFound, "магазин не найден")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sh)
}

func (h *Handler) Products(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	items, total, err := h.Store.ListProducts(r.Context(), store.ProductFilter{
		IncludeHidden: true,
		Query:         r.URL.Query().Get("q"),
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить товары")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) PatchProduct(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hidden *bool   `json:"hidden"`
		Price  *int    `json:"price"`
		Title  *string `json:"title"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if req.Price != nil && *req.Price <= 0 {
		httpx.Error(w, http.StatusBadRequest, "цена должна быть больше 0")
		return
	}
	if err := h.Store.AdminPatchProduct(r.Context(), httpx.Param(r, "id"), req.Title, req.Price, req.Hidden); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось сохранить")
		return
	}
	p, err := h.Store.GetProduct(r.Context(), httpx.Param(r, "id"))
	if err != nil || p == nil {
		httpx.Error(w, http.StatusNotFound, "товар не найден")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.DeleteProduct(r.Context(), httpx.Param(r, "id")); err != nil {
		httpx.Error(w, http.StatusNotFound, "товар не найден")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (h *Handler) Orders(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	items, total, err := h.Store.ListOrders(r.Context(), "", "", limit, offset)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить заказы")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) PatchOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status      *string `json:"status"`
		TrackNumber *string `json:"trackNumber"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	o, err := h.Store.GetOrder(r.Context(), httpx.Param(r, "id"))
	if err != nil || o == nil {
		httpx.Error(w, http.StatusNotFound, "заказ не найден")
		return
	}
	status := o.Status
	if req.Status != nil {
		status = *req.Status
	}
	track := ""
	if req.TrackNumber != nil {
		track = *req.TrackNumber
	}
	if err := h.Store.UpdateOrderStatus(r.Context(), o.ID, status, track, "admin"); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось обновить заказ")
		return
	}
	o, _ = h.Store.GetOrder(r.Context(), o.ID)
	if h.Hub != nil {
		h.Hub.SendOrder(o.BuyerID, o.SellerID, ws.Message{Type: "order.updated", OrderID: o.ID, Payload: o})
	}
	httpx.WriteJSON(w, http.StatusOK, o)
}

func (h *Handler) Payments(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	items, total, err := h.Store.ListPayments(r.Context(), limit, offset)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить платежи")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) Refund(w http.ResponseWriter, r *http.Request) {
	pay, err := h.Store.GetPayment(r.Context(), httpx.Param(r, "id"))
	if err != nil || pay == nil {
		httpx.Error(w, http.StatusNotFound, "платёж не найден")
		return
	}
	if pay.Status != models.PaySucceeded {
		httpx.Error(w, http.StatusBadRequest, "возврат только для succeeded")
		return
	}
	if !h.Config.PaymentsMock {
		if _, err := h.Pay.Refund(pay.ID, pay.Amount); err != nil {
			httpx.Error(w, http.StatusBadGateway, err.Error())
			return
		}
	}
	_ = h.Store.UpdatePayment(r.Context(), pay.ID, models.PayCanceled, nil)
	_ = h.Store.UpdateOrderStatus(r.Context(), pay.OrderID, models.StatusRefunded, "", "возврат")
	o, _ := h.Store.GetOrder(r.Context(), pay.OrderID)
	if o != nil && h.Hub != nil {
		h.Hub.SendOrder(o.BuyerID, o.SellerID, ws.Message{Type: "order.updated", OrderID: o.ID, Payload: o})
	}
	pay.Status = models.PayCanceled
	httpx.WriteJSON(w, http.StatusOK, pay)
}

func (h *Handler) Chats(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	items, total, err := h.Store.AdminListThreads(r.Context(), limit, offset)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить чаты")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) ChatMessages(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListMessages(r.Context(), httpx.Param(r, "threadId"), true, 500, 0)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить сообщения")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) HideMessage(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.HideMessage(r.Context(), httpx.Param(r, "id")); err != nil {
		httpx.Error(w, http.StatusNotFound, "сообщение не найдено")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}
