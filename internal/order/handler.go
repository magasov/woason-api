package order

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"woason-api/internal/config"
	"woason-api/internal/delivery"
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

func (h *Handler) GetCart(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	items, err := h.Store.GetCart(r.Context(), p.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить корзину")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) AddCart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProductID string `json:"productId"`
		Qty       int    `json:"qty"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.ProductID == "" {
		httpx.Error(w, http.StatusBadRequest, "нужен productId")
		return
	}
	prod, err := h.Store.GetProduct(r.Context(), req.ProductID)
	if err != nil || prod == nil || prod.Hidden || !models.IsGoodsCategory(prod.Category) {
		httpx.Error(w, http.StatusNotFound, "товар не найден")
		return
	}
	if err := h.Store.AddCart(r.Context(), httpx.MustPrincipal(r).ID, req.ProductID, req.Qty); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось добавить в корзину")
		return
	}
	h.GetCart(w, r)
}

func (h *Handler) PutCart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []models.CartItem `json:"items"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if err := h.Store.PutCart(r.Context(), httpx.MustPrincipal(r).ID, req.Items); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось сохранить корзину")
		return
	}
	h.GetCart(w, r)
}

func (h *Handler) PatchCart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Qty int `json:"qty"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if err := h.Store.SetCartQty(r.Context(), httpx.MustPrincipal(r).ID, httpx.Param(r, "productId"), req.Qty); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось обновить корзину")
		return
	}
	h.GetCart(w, r)
}

func (h *Handler) DeleteCartItem(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.SetCartQty(r.Context(), httpx.MustPrincipal(r).ID, httpx.Param(r, "productId"), 0); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось удалить позицию")
		return
	}
	h.GetCart(w, r)
}

func (h *Handler) ClearCart(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.ClearCart(r.Context(), httpx.MustPrincipal(r).ID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось очистить корзину")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": []any{}, "total": 0})
}

func (h *Handler) Favorites(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListFavorites(r.Context(), httpx.MustPrincipal(r).ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить избранное")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProductID string `json:"productId"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.ProductID == "" {
		httpx.Error(w, http.StatusBadRequest, "нужен productId")
		return
	}
	prod, err := h.Store.GetProduct(r.Context(), req.ProductID)
	if err != nil || prod == nil || prod.Hidden || !models.IsGoodsCategory(prod.Category) {
		httpx.Error(w, http.StatusNotFound, "товар не найден")
		return
	}
	if err := h.Store.AddFavorite(r.Context(), httpx.MustPrincipal(r).ID, req.ProductID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось добавить в избранное")
		return
	}
	h.Favorites(w, r)
}

func (h *Handler) DeleteFavorite(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.DeleteFavorite(r.Context(), httpx.MustPrincipal(r).ID, httpx.Param(r, "productId")); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось удалить")
		return
	}
	h.Favorites(w, r)
}

func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	var req struct {
		Address  string `json:"address"`
		Delivery string `json:"delivery"`
		City     string `json:"city"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if req.Delivery != delivery.MethodCDEK && req.Delivery != delivery.MethodPochta && req.Delivery != delivery.MethodPickup {
		httpx.Error(w, http.StatusBadRequest, "укажите способ доставки")
		return
	}
	if req.Delivery != delivery.MethodPickup && strings.TrimSpace(req.Address) == "" {
		httpx.Error(w, http.StatusBadRequest, "укажите адрес")
		return
	}
	cart, err := h.Store.GetCart(r.Context(), p.ID)
	if err != nil || len(cart) == 0 {
		httpx.Error(w, http.StatusBadRequest, "корзина пуста")
		return
	}
	sellerID := cart[0].Product.SellerID
	var weight float64
	var goods int
	items := make([]models.OrderItem, 0, len(cart))
	for _, c := range cart {
		if c.Product.SellerID != sellerID {
			httpx.Error(w, http.StatusBadRequest, "в корзине товары разных продавцов — оформите отдельно")
			return
		}
		weight += c.Product.WeightKg * float64(c.Qty)
		goods += c.Product.Price * c.Qty
		items = append(items, models.OrderItem{
			ProductID: c.ProductID,
			Title:     c.Product.Title,
			Price:     c.Product.Price,
			Qty:       c.Qty,
			Image:     c.Product.Image,
		})
	}
	city := strings.TrimSpace(req.City)
	if city == "" {
		city = "Москва"
	}
	quote, err := delivery.QuoteDelivery(req.Delivery, city, weight)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	addr := strings.TrimSpace(req.Address)
	if addr == "" {
		addr = "Самовывоз, " + city
	}
	o := &models.Order{
		ID:            store.NewOrderID(),
		CreatedAt:     time.Now().UTC(),
		BuyerID:       p.ID,
		SellerID:      sellerID,
		Items:         items,
		City:          city,
		Address:       addr,
		Delivery:      req.Delivery,
		DeliveryPrice: quote.Price,
		ETA:           quote.ETA,
		Status:        models.StatusAwaitingPayment,
		Total:         goods + quote.Price,
	}
	if err := h.Store.CreateOrder(r.Context(), o); err != nil {
		httpx.LogErr("create order", err)
		httpx.Error(w, http.StatusInternalServerError, "не удалось создать заказ")
		return
	}

	var confirmation string
	if h.Config.PaymentsMock {
		pid := uuid.NewString()
		pay := &models.Payment{ID: pid, OrderID: o.ID, Amount: o.Total, Status: models.PaySucceeded}
		raw, _ := json.Marshal(map[string]string{"mock": "true"})
		if err := h.Store.CreatePayment(r.Context(), pay, raw); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "не удалось создать платёж")
			return
		}
		_ = h.Store.UpdateOrderStatus(r.Context(), o.ID, models.StatusAwaitingShipment, "", "оплачен (mock)")
		o.Status = models.StatusAwaitingShipment
		h.notifyPaid(o)
	} else {
		res, err := h.Pay.CreatePayment(payment.CreateInput{
			OrderID:     o.ID,
			AmountRub:   o.Total,
			Description: "Заказ " + o.ID + " · WOAson",
			ReturnURL:   strings.TrimRight(h.Config.YKassaReturnURL, "/") + "/" + o.ID,
		})
		if err != nil {
			_ = h.Store.UpdateOrderStatus(r.Context(), o.ID, models.StatusCancelled, "", "ошибка оплаты")
			httpx.Error(w, http.StatusBadGateway, err.Error())
			return
		}
		pay := &models.Payment{
			ID:              res.ID,
			OrderID:         o.ID,
			Amount:          o.Total,
			Status:          res.Status,
			ConfirmationURL: res.ConfirmationURL,
		}
		if err := h.Store.CreatePayment(r.Context(), pay, res.Raw); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "не удалось сохранить платёж")
			return
		}
		confirmation = res.ConfirmationURL
	}
	_ = h.Store.ClearCart(r.Context(), p.ID)
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"order":           o,
		"confirmationUrl": confirmation,
		"confirmationURL": confirmation,
	})
}

func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	o, err := h.Store.GetOrder(r.Context(), httpx.Param(r, "id"))
	if err != nil || o == nil {
		httpx.Error(w, http.StatusNotFound, "заказ не найден")
		return
	}
	if p.Role != models.RoleAdmin && p.ID != o.BuyerID && p.ID != o.SellerID {
		httpx.Error(w, http.StatusForbidden, "нет доступа к заказу")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, o)
}

func (h *Handler) CabinetDashboard(w http.ResponseWriter, r *http.Request) {
	dash, err := h.Store.BuyerDashboard(r.Context(), httpx.MustPrincipal(r).ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dash)
}

func (h *Handler) CabinetOrders(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	items, total, err := h.Store.ListOrders(r.Context(), httpx.MustPrincipal(r).ID, "", limit, offset)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить заказы")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) CabinetReviews(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.PageDefault(r, 100, 100)
	items, total, err := h.Store.ListReviews(r.Context(), store.ReviewFilter{
		UserID: httpx.MustPrincipal(r).ID,
		Sort:   "new",
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить отзывы")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) CabinetPendingReviews(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.PageDefault(r, 100, 100)
	items, total, err := h.Store.ListPendingReviews(r.Context(), httpx.MustPrincipal(r).ID, limit, offset)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить отзывы")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) SellerOrders(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	items, total, err := h.Store.ListOrders(r.Context(), "", httpx.MustPrincipal(r).ID, limit, offset)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить заказы")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) PrintLabel(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	var req struct {
		Method string `json:"method"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if req.Method != delivery.MethodCDEK && req.Method != delivery.MethodPochta && req.Method != delivery.MethodPickup {
		httpx.Error(w, http.StatusBadRequest, "укажите method")
		return
	}
	o, err := h.Store.GetOrder(r.Context(), httpx.Param(r, "id"))
	if err != nil || o == nil || o.SellerID != p.ID {
		httpx.Error(w, http.StatusNotFound, "заказ не найден")
		return
	}
	track := delivery.MakeTrackNumber(req.Method, time.Now().UnixNano()%900000000+100000000)
	if err := h.Store.UpdateOrderStatus(r.Context(), o.ID, models.StatusLabelPrinted, track, "этикетка"); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось создать этикетку")
		return
	}
	_, _ = h.Store.Pool.Exec(r.Context(), `UPDATE orders SET delivery=$2 WHERE id=$1`, o.ID, req.Method)
	o, _ = h.Store.GetOrder(r.Context(), o.ID)
	if h.Hub != nil && o != nil {
		h.Hub.SendOrder(o.BuyerID, o.SellerID, ws.Message{Type: "order.updated", OrderID: o.ID, Payload: o})
	}
	httpx.WriteJSON(w, http.StatusOK, o)
}

func (h *Handler) SellerStatus(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	var req struct {
		Status string `json:"status"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.Status == "" {
		httpx.Error(w, http.StatusBadRequest, "нужен status")
		return
	}
	allowed := map[string]bool{
		models.StatusAwaitingShipment: true,
		models.StatusLabelPrinted:     true,
		models.StatusInTransit:        true,
		models.StatusDelivered:        true,
		models.StatusCancelled:        true,
	}
	if !allowed[req.Status] {
		httpx.Error(w, http.StatusBadRequest, "недопустимый статус")
		return
	}
	o, err := h.Store.GetOrder(r.Context(), httpx.Param(r, "id"))
	if err != nil || o == nil || o.SellerID != p.ID {
		httpx.Error(w, http.StatusNotFound, "заказ не найден")
		return
	}
	if err := h.Store.UpdateOrderStatus(r.Context(), o.ID, req.Status, "", "продавец"); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось обновить статус")
		return
	}
	o, _ = h.Store.GetOrder(r.Context(), o.ID)
	if h.Hub != nil && o != nil {
		h.Hub.SendOrder(o.BuyerID, o.SellerID, ws.Message{Type: "order.updated", OrderID: o.ID, Payload: o})
	}
	httpx.WriteJSON(w, http.StatusOK, o)
}

func (h *Handler) YookassaWebhook(w http.ResponseWriter, r *http.Request) {
	var hook payment.Webhook
	if err := json.NewDecoder(r.Body).Decode(&hook); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный webhook")
		return
	}
	var obj payment.PaymentObject
	if err := json.Unmarshal(hook.Object, &obj); err != nil || obj.ID == "" {
		httpx.Error(w, http.StatusBadRequest, "некорректный объект платежа")
		return
	}
	pay, err := h.Store.GetPayment(r.Context(), obj.ID)
	if err != nil || pay == nil {
		httpx.Error(w, http.StatusNotFound, "платёж не найден")
		return
	}
	raw, _ := json.Marshal(hook)
	status := obj.Status
	if status == "" {
		switch hook.Event {
		case "payment.succeeded":
			status = models.PaySucceeded
		case "payment.canceled":
			status = models.PayCanceled
		default:
			status = models.PayPending
		}
	}
	if status == "canceled" {
		status = models.PayCanceled
	}
	if err := h.Store.UpdatePayment(r.Context(), pay.ID, status, raw); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось обновить платёж")
		return
	}
	o, err := h.Store.GetOrder(r.Context(), pay.OrderID)
	if err != nil || o == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	if hook.Event == "payment.succeeded" || status == models.PaySucceeded {
		_ = h.Store.UpdateOrderStatus(r.Context(), o.ID, models.StatusAwaitingShipment, "", "оплачен")
		o.Status = models.StatusAwaitingShipment
		h.notifyPaid(o)
	}
	if hook.Event == "payment.canceled" || status == models.PayCanceled {
		_ = h.Store.UpdateOrderStatus(r.Context(), o.ID, models.StatusCancelled, "", "оплата отменена")
		o.Status = models.StatusCancelled
		h.Hub.SendOrder(o.BuyerID, o.SellerID, ws.Message{Type: "order.updated", OrderID: o.ID, Payload: o})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (h *Handler) notifyPaid(o *models.Order) {
	if h.Hub == nil {
		return
	}
	h.Hub.SendOrder(o.BuyerID, o.SellerID, ws.Message{Type: "payment.succeeded", OrderID: o.ID, Payload: o})
	h.Hub.SendOrder(o.BuyerID, o.SellerID, ws.Message{Type: "order.updated", OrderID: o.ID, Payload: o})
}
