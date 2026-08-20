package shop

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"woason-api/internal/auth"
	httpx "woason-api/internal/httpx"
	"woason-api/internal/models"
	"woason-api/internal/store"
)

type Handler struct {
	Store *store.Store
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := httpx.Param(r, "id")
	sh, err := h.Store.GetShop(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	viewer := httpx.PrincipalFrom(r)
	if sh == nil || (sh.Hidden && !isOwnerOrAdmin(viewer, id)) {
		httpx.Error(w, http.StatusNotFound, "магазин не найден")
		return
	}
	if !isOwnerOrAdmin(viewer, id) {
		sh.Hidden = false
	}
	httpx.WriteJSON(w, http.StatusOK, sh)
}

func (h *Handler) Products(w http.ResponseWriter, r *http.Request) {
	id := httpx.Param(r, "id")
	sh, err := h.Store.GetShop(r.Context(), id)
	if err != nil || sh == nil {
		httpx.Error(w, http.StatusNotFound, "магазин не найден")
		return
	}
	viewer := httpx.PrincipalFrom(r)
	if sh.Hidden && !isOwnerOrAdmin(viewer, id) {
		httpx.Error(w, http.StatusNotFound, "магазин не найден")
		return
	}
	limit, offset := httpx.Page(r)
	category := models.NormalizeCategory(r.URL.Query().Get("category"))
	if category != "" && !models.IsGoodsCategory(category) && !isOwnerOrAdmin(viewer, id) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": []models.Product{}, "total": 0})
		return
	}
	items, total, err := h.Store.ListProducts(r.Context(), store.ProductFilter{
		SellerID:      id,
		Category:      category,
		Query:         r.URL.Query().Get("q"),
		Sort:          r.URL.Query().Get("sort"),
		IncludeHidden: isOwnerOrAdmin(viewer, id),
		GoodsOnly:     !isOwnerOrAdmin(viewer, id),
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить товары")
		return
	}
	for i := range items {
		items[i].TradeType = models.PublicTradeType(items[i].TradeType, viewer, items[i].SellerID)
		if !isOwnerOrAdmin(viewer, id) {
			items[i].Hidden = false
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) Reels(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	uid := ""
	if p := httpx.PrincipalFrom(r); p != nil {
		uid = p.ID
	}
	items, total, err := h.Store.ListReels(r.Context(), httpx.Param(r, "id"), uid, limit, offset)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить рилсы")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) Stories(w http.ResponseWriter, r *http.Request) {
	id := httpx.Param(r, "id")
	sh, err := h.Store.GetShop(r.Context(), id)
	if err != nil || sh == nil || sh.Hidden {
		viewer := httpx.PrincipalFrom(r)
		if sh == nil || (sh.Hidden && !isOwnerOrAdmin(viewer, id)) {
			httpx.Error(w, http.StatusNotFound, "магазин не найден")
			return
		}
	}
	items, err := h.Store.ListStories(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить сторис")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	sh, err := h.Store.GetShop(r.Context(), p.ID)
	if err != nil || sh == nil {
		httpx.Error(w, http.StatusForbidden, "магазин не найден")
		return
	}
	dash, err := h.Store.SellerDashboard(r.Context(), p.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	dash["shop"] = sh
	httpx.WriteJSON(w, http.StatusOK, dash)
}

func (h *Handler) PatchShop(w http.ResponseWriter, r *http.Request) {
	if httpx.IsMultipart(r) {
		httpx.Error(w, http.StatusBadRequest, "файлы загружайте через POST /api/v1/uploads, сюда передайте URL")
		return
	}
	p := httpx.MustPrincipal(r)
	var req struct {
		ShopName    *string  `json:"shopName"`
		Description *string  `json:"description"`
		Logo        *string  `json:"logo"`
		Banner      *string  `json:"banner"`
		City        *string  `json:"city"`
		Phone       *string  `json:"phone"`
		Delivery    []string `json:"delivery"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if req.Logo != nil {
		logo := strings.TrimSpace(*req.Logo)
		if !auth.ValidAssetRef(logo) {
			httpx.Error(w, http.StatusBadRequest, "логотип — URL после /api/v1/uploads (kind=avatar)")
			return
		}
		req.Logo = &logo
	}
	if req.Banner != nil {
		banner := strings.TrimSpace(*req.Banner)
		if !auth.ValidAssetRef(banner) {
			httpx.Error(w, http.StatusBadRequest, "баннер — URL после /api/v1/uploads (kind=banner)")
			return
		}
		req.Banner = &banner
	}
	if err := h.Store.PatchShop(r.Context(), p.ID, req.ShopName, req.Description, req.Logo, req.Banner, req.City, req.Phone, req.Delivery, nil); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось сохранить магазин")
		return
	}
	sh, _ := h.Store.GetShop(r.Context(), p.ID)
	httpx.WriteJSON(w, http.StatusOK, sh)
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	if httpx.IsMultipart(r) {
		httpx.Error(w, http.StatusBadRequest, "файлы загружайте через POST /api/v1/uploads, сюда передайте URL в image/images")
		return
	}
	p := httpx.MustPrincipal(r)
	sh, err := h.Store.GetShop(r.Context(), p.ID)
	if err != nil || sh == nil {
		httpx.Error(w, http.StatusForbidden, "сначала создайте магазин")
		return
	}
	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Price       int      `json:"price"`
		OldPrice    *int     `json:"oldPrice"`
		Condition   string   `json:"condition"`
		Category    string   `json:"category"`
		Image       string   `json:"image"`
		Images      []string `json:"images"`
		City        string   `json:"city"`
		WeightKg    float64  `json:"weightKg"`
		InStock     int      `json:"inStock"`
		Delivery    []string `json:"delivery"`
		Tags        []string `json:"tags"`
		TradeType   string   `json:"tradeType"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if strings.TrimSpace(req.Title) == "" || req.Price <= 0 {
		httpx.Error(w, http.StatusBadRequest, "нужны название и цена больше 0")
		return
	}
	req.Image = strings.TrimSpace(req.Image)
	images := make([]string, 0, len(req.Images)+1)
	for _, u := range req.Images {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if !auth.ValidPhotoURL(u) {
			httpx.Error(w, http.StatusBadRequest, "images — публичные URL после /api/v1/uploads")
			return
		}
		images = append(images, u)
	}
	if req.Image == "" && len(images) > 0 {
		req.Image = images[0]
	}
	if !auth.ValidPhotoURL(req.Image) {
		httpx.Error(w, http.StatusBadRequest, "укажите корректный URL фото")
		return
	}
	if len(images) == 0 {
		images = []string{req.Image}
	} else if images[0] != req.Image {
		found := false
		for _, u := range images {
			if u == req.Image {
				found = true
				break
			}
		}
		if !found {
			images = append([]string{req.Image}, images...)
		}
	}
	if len(images) > 12 {
		httpx.Error(w, http.StatusBadRequest, "слишком много фото (максимум 12)")
		return
	}
	if req.Condition != "new" && req.Condition != "used" {
		req.Condition = "new"
	}
	req.Category = models.NormalizeCategory(req.Category)
	if req.Category == "" {
		httpx.Error(w, http.StatusBadRequest, "укажите категорию")
		return
	}
	if models.IsBannedCategory(req.Category) {
		httpx.Error(w, http.StatusBadRequest, "категория недоступна: на WOAson только товары")
		return
	}
	if !models.IsGoodsCategory(req.Category) {
		httpx.Error(w, http.StatusBadRequest, "неизвестная категория")
		return
	}
	if !models.ValidDeliveryMethods(req.Delivery) {
		httpx.Error(w, http.StatusBadRequest, "доставка: cdek, pochta или pickup")
		return
	}
	trade := req.TradeType
	if trade == "" {
		trade = models.TradeRetail
	}
	if trade != models.TradeRetail && trade != models.TradeWholesale && trade != models.TradeDropship {
		httpx.Error(w, http.StatusBadRequest, "неизвестный тип продажи")
		return
	}
	kind := "shop"
	if req.Condition == "used" {
		kind = "private"
	}
	deliv := req.Delivery
	if len(deliv) == 0 {
		deliv = sh.Delivery
	}
	city := req.City
	if city == "" {
		city = sh.City
	}
	weight := req.WeightKg
	if weight <= 0 {
		weight = 0.5
	}
	stock := req.InStock
	if stock <= 0 {
		stock = 10
	}
	prod := &models.Product{
		ID:          "p-" + uuid.NewString()[:12],
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
		Price:       req.Price,
		OldPrice:    req.OldPrice,
		Rating:      5,
		SellerKind:  kind,
		Condition:   req.Condition,
		Category:    req.Category,
		Image:       strings.TrimSpace(req.Image),
		Images:      images,
		SellerID:    sh.ID,
		SellerName:  sh.ShopName,
		City:        city,
		WeightKg:    weight,
		InStock:     stock,
		Delivery:    deliv,
		Tags:        req.Tags,
		TradeType:   trade,
	}
	if prod.Description == "" {
		prod.Description = "Товар продавца WOAson"
	}
	if err := h.Store.CreateProduct(r.Context(), prod); err != nil {
		httpx.LogErr("create product", err)
		httpx.Error(w, http.StatusInternalServerError, "не удалось создать товар")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, prod)
}

func (h *Handler) SellerProducts(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	limit, offset := httpx.Page(r)
	items, total, err := h.Store.ListProducts(r.Context(), store.ProductFilter{
		SellerID:      p.ID,
		IncludeHidden: true,
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить товары")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) CreateStory(w http.ResponseWriter, r *http.Request) {
	if httpx.IsMultipart(r) {
		httpx.Error(w, http.StatusBadRequest, "файлы загружайте через POST /api/v1/uploads, сюда передайте URL в image")
		return
	}
	p := httpx.MustPrincipal(r)
	var req struct {
		Image   string `json:"image"`
		Caption string `json:"caption"`
	}
	if err := httpx.Decode(r, &req); err != nil || !auth.ValidPhotoURL(req.Image) {
		httpx.Error(w, http.StatusBadRequest, "нужна ссылка на фото")
		return
	}
	st := models.Story{
		ID:        "st-" + time.Now().Format("150405000"),
		SellerID:  p.ID,
		Image:     strings.TrimSpace(req.Image),
		Caption:   strings.TrimSpace(req.Caption),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if st.Caption == "" {
		st.Caption = "Сторис магазина"
	}
	if err := h.Store.CreateStory(r.Context(), st); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось опубликовать сторис")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, st)
}

func (h *Handler) CreateReel(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	sh, err := h.Store.GetShop(r.Context(), p.ID)
	if err != nil || sh == nil {
		httpx.Error(w, http.StatusForbidden, "магазин не найден")
		return
	}
	var req struct {
		ProductID string `json:"productId"`
		Title     string `json:"title"`
		Caption   string `json:"caption"`
	}
	if err := httpx.Decode(r, &req); err != nil || strings.TrimSpace(req.Title) == "" {
		httpx.Error(w, http.StatusBadRequest, "нужен заголовок")
		return
	}
	prod, err := h.Store.GetProduct(r.Context(), req.ProductID)
	if err != nil || prod == nil || prod.SellerID != p.ID || !models.IsGoodsCategory(prod.Category) {
		httpx.Error(w, http.StatusBadRequest, "укажите свой товар")
		return
	}
	reel := models.Reel{
		ID:          "reel-" + time.Now().Format("150405000"),
		ProductID:   prod.ID,
		SellerID:    sh.ID,
		SellerName:  sh.ShopName,
		Title:       strings.TrimSpace(req.Title),
		Caption:     strings.TrimSpace(req.Caption),
		Comments:    []models.ReelComment{},
		DurationSec: 18,
		Gradient:    []string{"#1c1917", "#e2571b"},
	}
	if err := h.Store.CreateReel(r.Context(), reel); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось добавить рилс")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, reel)
}

func (h *Handler) SellerReviews(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.PageDefault(r, 100, 100)
	items, total, err := h.Store.ListReviews(r.Context(), store.ReviewFilter{
		SellerID: httpx.MustPrincipal(r).ID,
		Sort:     "new",
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось загрузить отзывы")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) ReplyReview(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	var req struct {
		Text string `json:"text"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	text := strings.TrimSpace(req.Text)
	n := utf8.RuneCountInString(text)
	if n < 2 || n > 1000 {
		httpx.Error(w, http.StatusBadRequest, "ответ должен быть от 2 до 1000 символов")
		return
	}
	id := httpx.Param(r, "reviewId")
	rev, err := h.Store.GetReview(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	if rev == nil {
		httpx.Error(w, http.StatusNotFound, "отзыв не найден")
		return
	}
	prod, err := h.Store.GetProduct(r.Context(), rev.ProductID)
	if err != nil || prod == nil || prod.SellerID != p.ID {
		httpx.Error(w, http.StatusForbidden, "чужой отзыв")
		return
	}
	out, err := h.Store.ReplyToReview(r.Context(), id, p.ID, text)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось сохранить ответ")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func isOwnerOrAdmin(p *models.Principal, sellerID string) bool {
	if p == nil {
		return false
	}
	return p.Role == models.RoleAdmin || p.ID == sellerID
}
