package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	httpx "woason-api/internal/httpx"
	"woason-api/internal/models"
	"woason-api/internal/store"
)

type Handler struct {
	Store  *store.Store
	Tokens *Tokens
}

type registerReq struct {
	Name            string   `json:"name"`
	Email           string   `json:"email"`
	Phone           string   `json:"phone"`
	Password        string   `json:"password"`
	Role            string   `json:"role"`
	ShopName        string   `json:"shopName"`
	ShopDescription string   `json:"shopDescription"`
	Delivery        []string `json:"delivery"`
	City            string   `json:"city"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshReq struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	if !ValidName(req.Name) || !ValidEmail(req.Email) || !ValidPhone(req.Phone) || !ValidPassword(req.Password) {
		httpx.Error(w, http.StatusBadRequest, "проверьте имя, email, телефон и пароль")
		return
	}
	if req.Role != models.RoleBuyer && req.Role != models.RoleSeller {
		httpx.Error(w, http.StatusBadRequest, "роль должна быть buyer или seller")
		return
	}
	exists, err := h.Store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	if exists != nil {
		httpx.Error(w, http.StatusConflict, "такой email уже зарегистрирован")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	u := &models.User{
		ID:           "user-" + uuid.NewString()[:8],
		Name:         req.Name,
		Email:        req.Email,
		Phone:        req.Phone,
		Role:         req.Role,
		PasswordHash: string(hash),
	}
	if err := h.Store.CreateUser(r.Context(), u); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось создать пользователя")
		return
	}
	if req.Role == models.RoleSeller {
		name := strings.TrimSpace(req.ShopName)
		if name == "" {
			name = req.Name
		}
		deliv := req.Delivery
		if len(deliv) == 0 {
			deliv = []string{"cdek", "pochta"}
		}
		if !models.ValidDeliveryMethods(deliv) {
			httpx.Error(w, http.StatusBadRequest, "доставка: cdek, pochta или pickup")
			return
		}
		city := strings.TrimSpace(req.City)
		if city == "" {
			city = "Москва"
		}
		shop := &models.Shop{
			ID:          u.ID,
			ShopName:    name,
			Description: req.ShopDescription,
			Logo:        "🛍️",
			City:        city,
			Phone:       req.Phone,
			Kind:        "shop",
			Delivery:    deliv,
		}
		if err := h.Store.UpsertShop(r.Context(), shop); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "не удалось создать магазин")
			return
		}
		u.Seller = shop
	}
	h.respondTokens(w, r, u)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	u, err := h.Store.GetUserByEmail(r.Context(), strings.TrimSpace(req.Email))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	if u == nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		httpx.Error(w, http.StatusUnauthorized, "неверный email или пароль")
		return
	}
	if u.BannedAt != nil {
		httpx.Error(w, http.StatusForbidden, "аккаунт заблокирован")
		return
	}
	full, err := h.Store.GetUserByID(r.Context(), u.ID)
	if err != nil || full == nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	h.respondTokens(w, r, full)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	_ = httpx.Decode(r, &req)
	if req.RefreshToken != "" {
		_ = h.Store.RevokeRefresh(r.Context(), HashRefresh(req.RefreshToken))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if err := httpx.Decode(r, &req); err != nil || req.RefreshToken == "" {
		httpx.Error(w, http.StatusBadRequest, "нужен refreshToken")
		return
	}
	userID, err := h.Store.ConsumeRefresh(r.Context(), HashRefresh(req.RefreshToken))
	if err != nil || userID == "" {
		httpx.Error(w, http.StatusUnauthorized, "сессия истекла")
		return
	}
	u, err := h.Store.GetUserByID(r.Context(), userID)
	if err != nil || u == nil {
		httpx.Error(w, http.StatusUnauthorized, "сессия истекла")
		return
	}
	if u.BannedAt != nil {
		httpx.Error(w, http.StatusForbidden, "аккаунт заблокирован")
		return
	}
	h.respondTokens(w, r, u)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	u, err := h.Store.GetUserByID(r.Context(), p.ID)
	if err != nil || u == nil {
		httpx.Error(w, http.StatusUnauthorized, "сессия недействительна")
		return
	}
	u.PasswordHash = ""
	httpx.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) PatchMe(w http.ResponseWriter, r *http.Request) {
	if httpx.IsMultipart(r) {
		httpx.Error(w, http.StatusBadRequest, "файлы загружайте через POST /api/v1/uploads, сюда передайте URL")
		return
	}
	p := httpx.MustPrincipal(r)
	var req struct {
		Name   *string `json:"name"`
		Phone  *string `json:"phone"`
		Avatar *string `json:"avatar"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if !ValidName(name) {
			httpx.Error(w, http.StatusBadRequest, "проверьте имя")
			return
		}
		req.Name = &name
	}
	if req.Phone != nil {
		phone := strings.TrimSpace(*req.Phone)
		if phone != "" && !ValidPhone(phone) {
			httpx.Error(w, http.StatusBadRequest, "проверьте телефон")
			return
		}
		req.Phone = &phone
	}
	if req.Avatar != nil {
		avatar := strings.TrimSpace(*req.Avatar)
		if avatar != "" && !ValidPhotoURL(avatar) {
			httpx.Error(w, http.StatusBadRequest, "аватар — публичный URL после /api/v1/uploads")
			return
		}
		req.Avatar = &avatar
	}
	u, err := h.Store.PatchMe(r.Context(), p.ID, req.Name, req.Phone, req.Avatar)
	if err != nil || u == nil {
		httpx.Error(w, http.StatusInternalServerError, "не удалось сохранить профиль")
		return
	}
	u.PasswordHash = ""
	httpx.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) respondTokens(w http.ResponseWriter, r *http.Request, u *models.User) {
	access, err := h.Tokens.IssueAccess(u)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	raw, hash, err := NewRefreshToken()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	if err := h.Store.SaveRefresh(r.Context(), u.ID, hash, time.Now().Add(RefreshTTL())); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	out := *u
	out.PasswordHash = ""
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"accessToken":  access,
		"refreshToken": raw,
		"user":         out,
	})
}
