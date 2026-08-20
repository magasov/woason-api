package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"

	"woason-api/internal/auth"
	"woason-api/internal/config"
	"woason-api/internal/db"
	api "woason-api/internal/http"
	"woason-api/internal/payment"
	"woason-api/internal/seed"
	"woason-api/internal/store"
	"woason-api/internal/ws"
)

type tokenResp struct {
	AccessToken string         `json:"accessToken"`
	User        map[string]any `json:"user"`
}

func testAPI(t *testing.T) http.Handler {
	t.Helper()
	_ = godotenv.Load("../../.env")
	os.Setenv("PAYMENTS_MOCK", "true")
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("нет DATABASE_URL")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.PaymentsMock = true
	cfg.UploadDir = t.TempDir()
	cfg.PublicBaseURL = "http://localhost:8080"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := db.ConnectOnce(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skip("postgres недоступен")
	}
	t.Cleanup(pool.Close)
	if err := db.MigrateUp(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		cfg.MigrationsPath = "../../migrations"
		if err := db.MigrateUp(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
			t.Fatal(err)
		}
	}
	st := store.New(pool)
	if err := seed.Run(context.Background(), st, cfg); err != nil {
		t.Fatal(err)
	}
	return api.NewRouter(api.Deps{
		Config: cfg,
		Store:  st,
		Tokens: auth.NewTokens(cfg.JWTSecret),
		Hub:    ws.NewHub(),
		Pay:    payment.New(cfg),
	})
}

func postJSON(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func login(t *testing.T, h http.Handler, email, password string) tokenResp {
	t.Helper()
	rec := postJSON(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": email, "password": password})
	if rec.Code != 200 {
		t.Fatalf("login %s: %d %s", email, rec.Code, rec.Body.String())
	}
	var tok tokenResp
	if err := json.Unmarshal(rec.Body.Bytes(), &tok); err != nil || tok.AccessToken == "" {
		t.Fatalf("no token: %s", rec.Body.String())
	}
	return tok
}

func removeProduct(t *testing.T, h http.Handler, productID string) {
	t.Helper()
	if productID == "" {
		return
	}
	t.Cleanup(func() {
		admin := login(t, h, "admin@woason.ru", "123456")
		rec := postJSON(t, h, http.MethodDelete, "/api/v1/admin/products/"+productID, admin.AccessToken, nil)
		if rec.Code != 200 && rec.Code != http.StatusNotFound {
			t.Logf("cleanup product %s: %d %s", productID, rec.Code, rec.Body.String())
		}
	})
}

func TestDocsPortal(t *testing.T) {
	h := api.NewRouter(api.Deps{
		Tokens: auth.NewTokens("test-secret"),
		Hub:    ws.NewHub(),
		Store:  &store.Store{},
		Config: config.Config{FrontendURL: "http://localhost:3000"},
	})
	cases := map[string]string{
		"/docs":              "WOAson API Reference",
		"/docs/":             "Справочник методов",
		"/docs/terms":        "Условия использования",
		"/docs/openapi.yaml": "openapi: 3.0.3",
	}
	for path, want := range cases {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s: want 200, got %d %s", path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("%s: missing %q", path, want)
		}
	}
	spec := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	srec := httptest.NewRecorder()
	h.ServeHTTP(srec, spec)
	for _, fragment := range []string{"/api/v1/auth/login", "/api/v1/checkout", "/api/v1/admin/stats", "BearerAuth"} {
		if !strings.Contains(srec.Body.String(), fragment) {
			t.Fatalf("openapi missing %s", fragment)
		}
	}
}

func TestCategoriesGoodsOnly(t *testing.T) {
	h := api.NewRouter(api.Deps{
		Tokens: auth.NewTokens("test-secret"),
		Hub:    ws.NewHub(),
		Store:  &store.Store{},
		Config: config.Config{FrontendURL: "http://localhost:3000"},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("want 200, got %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 22 || len(body.Items) != 22 {
		t.Fatalf("want 22 categories, got total=%d len=%d", body.Total, len(body.Items))
	}
	banned := map[string]bool{"avto": true, "nedvizhimost": true, "transport": true, "uslugi": true, "rabota": true, "travel": true}
	for _, it := range body.Items {
		slug, _ := it["slug"].(string)
		if banned[slug] {
			t.Fatalf("classified slug in categories: %s", slug)
		}
		if it["group"] == nil || it["group"] == "" {
			t.Fatalf("category %s missing group", slug)
		}
		if it["name"] == nil || it["name"] == "" {
			t.Fatalf("category %s missing name", slug)
		}
	}
}

func TestRejectBannedCategory(t *testing.T) {
	h := testAPI(t)
	email := fmt.Sprintf("seller-ban-%d@woason.ru", time.Now().UnixNano())
	reg := postJSON(t, h, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "Продавец", "email": email, "phone": "+79001112233",
		"password": "123456", "role": "seller", "shopName": "Тест", "city": "Казань",
		"delivery": []string{"cdek"},
	})
	if reg.Code != 200 && reg.Code != 201 {
		t.Fatalf("register: %d %s", reg.Code, reg.Body.String())
	}
	seller := login(t, h, email, "123456")
	for _, cat := range []string{"avto", "nedvizhimost", "uslugi", "rabota", "travel", "transport"} {
		rec := postJSON(t, h, http.MethodPost, "/api/v1/seller/products", seller.AccessToken, map[string]any{
			"title": "Нельзя", "price": 1000, "category": cat, "condition": "new",
			"image": "https://example.com/p.jpg",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("category %s: want 400, got %d %s", cat, rec.Code, rec.Body.String())
		}
	}
}

func createSellerProduct(t *testing.T, h http.Handler, token, title, category, trade string) {
	t.Helper()
	rec := postJSON(t, h, http.MethodPost, "/api/v1/seller/products", token, map[string]any{
		"title": title, "price": 1000, "category": category, "condition": "new",
		"image": "https://example.com/p.jpg", "tradeType": trade,
	})
	if rec.Code != http.StatusCreated && rec.Code != 200 {
		t.Fatalf("product %s: %d %s", title, rec.Code, rec.Body.String())
	}
	var prod map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &prod)
	id, _ := prod["id"].(string)
	removeProduct(t, h, id)
}

func TestProductsFilterSearchAndDashboard(t *testing.T) {
	h := testAPI(t)
	sellerEmail := fmt.Sprintf("seller-filter-%d@woason.ru", time.Now().UnixNano())
	reg := postJSON(t, h, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "Продавец", "email": sellerEmail, "phone": "+79001112233",
		"password": "123456", "role": "seller", "shopName": "Фильтр", "city": "Москва",
		"delivery": []string{"cdek"},
	})
	if reg.Code != 200 && reg.Code != 201 {
		t.Fatalf("register: %d %s", reg.Code, reg.Body.String())
	}
	seller := login(t, h, sellerEmail, "123456")
	createSellerProduct(t, h, seller.AccessToken, "Футболка тест", "odezhda", "retail")
	createSellerProduct(t, h, seller.AccessToken, "Наушники Pulse", "elektronika", "retail")
	createSellerProduct(t, h, seller.AccessToken, "Геймпад дроп", "igry", "dropship")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?category=avto", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("avto filter: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"total":0`) && !strings.Contains(rec.Body.String(), `"total": 0`) {
		t.Fatalf("avto must be empty: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/products?category=odezhda&sort=new&limit=10", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("odezhda: %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Total < 1 {
		t.Fatalf("expected odezhda products, got %s", rec.Body.String())
	}
	for _, it := range list.Items {
		if it["category"] != "odezhda" {
			t.Fatalf("unexpected category %v", it["category"])
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/products?q=наушники&sort=rating", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("search: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "pulse") && !strings.Contains(rec.Body.String(), "науш") {
		t.Fatalf("search miss: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=5&offset=0&sort=price", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("sort price: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	for _, it := range list.Items {
		if tt, _ := it["tradeType"].(string); tt == "dropship" {
			t.Fatalf("public dropship leak: %+v", it)
		}
	}

	email := fmt.Sprintf("buyer-dash-%d@woason.ru", time.Now().UnixNano())
	buyerReg := postJSON(t, h, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "Покупатель", "email": email, "phone": "+79002223344",
		"password": "123456", "role": "buyer",
	})
	if buyerReg.Code != 200 && buyerReg.Code != 201 {
		t.Fatalf("register buyer: %d %s", buyerReg.Code, buyerReg.Body.String())
	}
	buyer := login(t, h, email, "123456")
	dash := httptest.NewRequest(http.MethodGet, "/api/v1/cabinet/dashboard", nil)
	dash.Header.Set("Authorization", "Bearer "+buyer.AccessToken)
	drec := httptest.NewRecorder()
	h.ServeHTTP(drec, dash)
	if drec.Code != 200 {
		t.Fatalf("dashboard: %d %s", drec.Code, drec.Body.String())
	}
	for _, key := range []string{"favoritesCount", "cartCount", "ordersCount", "spent", "inTransit", "waitingReviews", "week"} {
		if !strings.Contains(drec.Body.String(), `"`+key+`"`) {
			t.Fatalf("dashboard missing %s: %s", key, drec.Body.String())
		}
	}
}

func TestAdminStatsUnauthorized(t *testing.T) {
	h := api.NewRouter(api.Deps{
		Tokens: auth.NewTokens("test-secret"),
		Hub:    ws.NewHub(),
		Store:  &store.Store{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminStatsOK(t *testing.T) {
	h := testAPI(t)
	tok := login(t, h, "admin@woason.ru", "123456")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("want 200, got %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"users"`) {
		t.Fatalf("stats body: %s", rec.Body.String())
	}
}

func TestCheckoutMockPayment(t *testing.T) {
	h := testAPI(t)
	suffix := time.Now().UnixNano()
	sellerRec := postJSON(t, h, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "Продавец", "email": "seller-test@woason.ru", "phone": "+79001112233",
		"password": "123456", "role": "seller", "shopName": "Тест", "delivery": []string{"cdek"},
	})
	if sellerRec.Code != 200 && sellerRec.Code != 201 {
		if sellerRec.Code == 409 {
			login(t, h, "seller-test@woason.ru", "123456")
		} else {
			t.Fatalf("register seller: %d %s", sellerRec.Code, sellerRec.Body.String())
		}
	}
	seller := login(t, h, "seller-test@woason.ru", "123456")
	prodRec := postJSON(t, h, http.MethodPost, "/api/v1/seller/products", seller.AccessToken, map[string]any{
		"title": "Тест товар", "price": 500, "category": "dom", "condition": "new",
		"image": "https://example.com/p.jpg",
	})
	if prodRec.Code >= 300 {
		t.Fatalf("product: %d %s", prodRec.Code, prodRec.Body.String())
	}
	var prod map[string]any
	_ = json.Unmarshal(prodRec.Body.Bytes(), &prod)
	productID, _ := prod["id"].(string)
	if productID == "" {
		t.Fatalf("no product id: %s", prodRec.Body.String())
	}
	removeProduct(t, h, productID)

	buyerEmail := "buyer-test@woason.ru"
	buyerRec := postJSON(t, h, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "Покупатель", "email": buyerEmail, "phone": "+79002223344",
		"password": "123456", "role": "buyer",
	})
	if buyerRec.Code != 200 && buyerRec.Code != 201 && buyerRec.Code != 409 {
		t.Fatalf("register buyer: %d %s", buyerRec.Code, buyerRec.Body.String())
	}
	_ = suffix
	buyer := login(t, h, buyerEmail, "123456")
	cartRec := postJSON(t, h, http.MethodPost, "/api/v1/cart", buyer.AccessToken, map[string]any{"productId": productID, "qty": 1})
	if cartRec.Code >= 300 {
		t.Fatalf("cart: %d %s", cartRec.Code, cartRec.Body.String())
	}
	chk := postJSON(t, h, http.MethodPost, "/api/v1/checkout", buyer.AccessToken, map[string]string{
		"address": "ул. Арбат, 1", "delivery": "cdek", "city": "Москва",
	})
	if chk.Code != http.StatusCreated && chk.Code != 200 {
		t.Fatalf("checkout: %d %s", chk.Code, chk.Body.String())
	}
	if !strings.Contains(chk.Body.String(), "awaiting_shipment") && !strings.Contains(chk.Body.String(), "paid") {
		t.Fatalf("expected paid/awaiting_shipment: %s", chk.Body.String())
	}
}

func TestChatWebSocket(t *testing.T) {
	h := testAPI(t)
	sellerRec := postJSON(t, h, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "Продавец", "email": "seller-ws@woason.ru", "phone": "+79001112233",
		"password": "123456", "role": "seller", "shopName": "Чат", "delivery": []string{"cdek"},
	})
	if sellerRec.Code != 200 && sellerRec.Code != 201 && sellerRec.Code != 409 {
		t.Fatalf("register seller: %d %s", sellerRec.Code, sellerRec.Body.String())
	}
	seller := login(t, h, "seller-ws@woason.ru", "123456")
	sellerID, _ := seller.User["id"].(string)
	buyerRec := postJSON(t, h, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "Покупатель", "email": "buyer-ws@woason.ru", "phone": "+79002223344",
		"password": "123456", "role": "buyer",
	})
	if buyerRec.Code != 200 && buyerRec.Code != 201 && buyerRec.Code != 409 {
		t.Fatalf("register buyer: %d %s", buyerRec.Code, buyerRec.Body.String())
	}
	buyer := login(t, h, "buyer-ws@woason.ru", "123456")

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?token=" + buyer.AccessToken
	c, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := c.WriteJSON(map[string]string{"type": "subscribe", "channel": "chat", "peerId": sellerID}); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteJSON(map[string]string{"type": "chat.send", "peerId": sellerID, "text": "тест ws"}); err != nil {
		t.Fatal(err)
	}
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "chat.message") && !strings.Contains(string(data), "тест ws") {
		t.Fatalf("ws payload: %s", data)
	}
}

var png1x1 = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func postMultipart(t *testing.T, h http.Handler, path, token, kind string, files [][]byte, names []string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if kind != "" {
		if err := mw.WriteField("kind", kind); err != nil {
			t.Fatal(err)
		}
	}
	for i, data := range files {
		name := names[i]
		fw, err := mw.CreateFormFile("files", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUploadUnauthorized(t *testing.T) {
	h := api.NewRouter(api.Deps{
		Tokens: auth.NewTokens("test-secret"),
		Hub:    ws.NewHub(),
		Store:  &store.Store{},
		Config: config.Config{FrontendURL: "http://localhost:3000", UploadDir: t.TempDir(), PublicBaseURL: "http://localhost:8080"},
	})
	rec := postMultipart(t, h, "/api/v1/uploads", "", "product", [][]byte{png1x1}, []string{"a.png"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestUploadsAndJSONAssets(t *testing.T) {
	h := testAPI(t)
	email := fmt.Sprintf("seller-up-%d@woason.ru", time.Now().UnixNano())
	reg := postJSON(t, h, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "Продавец", "email": email, "phone": "+79001112233",
		"password": "123456", "role": "seller", "shopName": "Аплоад", "city": "Казань",
		"delivery": []string{"cdek"},
	})
	if reg.Code != 200 && reg.Code != 201 {
		t.Fatalf("register: %d %s", reg.Code, reg.Body.String())
	}
	seller := login(t, h, email, "123456")

	badKind := postMultipart(t, h, "/api/v1/uploads", seller.AccessToken, "video", [][]byte{png1x1}, []string{"a.png"})
	if badKind.Code != http.StatusBadRequest {
		t.Fatalf("kind: want 400, got %d %s", badKind.Code, badKind.Body.String())
	}
	notImg := postMultipart(t, h, "/api/v1/uploads", seller.AccessToken, "product", [][]byte{[]byte("not-an-image")}, []string{"a.txt"})
	if notImg.Code != http.StatusBadRequest {
		t.Fatalf("txt: want 400, got %d %s", notImg.Code, notImg.Body.String())
	}
	tooMany := make([][]byte, 13)
	names := make([]string, 13)
	for i := range tooMany {
		tooMany[i] = png1x1
		names[i] = fmt.Sprintf("%d.png", i)
	}
	many := postMultipart(t, h, "/api/v1/uploads", seller.AccessToken, "product", tooMany, names)
	if many.Code != http.StatusBadRequest {
		t.Fatalf("13 files: want 400, got %d %s", many.Code, many.Body.String())
	}

	up := postMultipart(t, h, "/api/v1/uploads", seller.AccessToken, "product",
		[][]byte{png1x1, png1x1}, []string{"1.png", "2.png"})
	if up.Code != 200 {
		t.Fatalf("upload: %d %s", up.Code, up.Body.String())
	}
	var upBody struct {
		URLs  []string `json:"urls"`
		Items []struct {
			URL      string `json:"url"`
			Filename string `json:"filename"`
		} `json:"items"`
	}
	if err := json.Unmarshal(up.Body.Bytes(), &upBody); err != nil {
		t.Fatal(err)
	}
	if len(upBody.URLs) != 2 || len(upBody.Items) != 2 {
		t.Fatalf("want 2 urls: %s", up.Body.String())
	}
	if !strings.Contains(upBody.URLs[0], "/uploads/product/") {
		t.Fatalf("url path: %s", upBody.URLs[0])
	}

	path := strings.TrimPrefix(upBody.URLs[0], "http://localhost:8080")
	imgReq := httptest.NewRequest(http.MethodGet, path, nil)
	imgReq.Header.Set("Origin", "http://localhost:3000")
	imgRec := httptest.NewRecorder()
	h.ServeHTTP(imgRec, imgReq)
	if imgRec.Code != 200 {
		t.Fatalf("static: %d %s", imgRec.Code, imgRec.Body.String())
	}
	got, _ := io.ReadAll(imgRec.Body)
	if !bytes.Equal(got, png1x1) {
		t.Fatalf("static body mismatch, %d bytes", len(got))
	}
	if imgRec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("cors: %v", imgRec.Header())
	}

	blob := postJSON(t, h, http.MethodPost, "/api/v1/seller/products", seller.AccessToken, map[string]any{
		"title": "Блоб", "price": 100, "category": "dom", "condition": "new",
		"image": "blob:http://localhost:3000/abc",
	})
	if blob.Code != http.StatusBadRequest {
		t.Fatalf("blob product: want 400, got %d %s", blob.Code, blob.Body.String())
	}

	prod := postJSON(t, h, http.MethodPost, "/api/v1/seller/products", seller.AccessToken, map[string]any{
		"title": "С фото", "description": "тест", "price": 1500, "oldPrice": 2000,
		"category": "dom", "condition": "new",
		"image": upBody.URLs[0], "images": upBody.URLs,
		"city": "Казань", "weightKg": 0.4, "inStock": 3,
		"delivery": []string{"cdek"}, "tags": []string{"тест"}, "tradeType": "retail",
	})
	if prod.Code != http.StatusCreated && prod.Code != 200 {
		t.Fatalf("product: %d %s", prod.Code, prod.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(prod.Body.Bytes(), &created)
	if id, _ := created["id"].(string); id != "" {
		removeProduct(t, h, id)
	}

	av := postMultipart(t, h, "/api/v1/uploads", seller.AccessToken, "avatar", [][]byte{png1x1}, []string{"ava.png"})
	if av.Code != 200 {
		t.Fatalf("avatar upload: %d %s", av.Code, av.Body.String())
	}
	var avBody struct {
		URLs []string `json:"urls"`
	}
	_ = json.Unmarshal(av.Body.Bytes(), &avBody)
	me := postJSON(t, h, http.MethodPatch, "http://localhost:8080/api/v1/me", seller.AccessToken, map[string]any{
		"name": "Продавец Аватар", "avatar": avBody.URLs[0],
	})
	if me.Code != 200 {
		t.Fatalf("patch me: %d %s", me.Code, me.Body.String())
	}
	if !strings.Contains(me.Body.String(), avBody.URLs[0]) {
		t.Fatalf("me avatar: %s", me.Body.String())
	}

	bn := postMultipart(t, h, "/api/v1/uploads", seller.AccessToken, "banner", [][]byte{png1x1}, []string{"bn.png"})
	var bnBody struct {
		URLs []string `json:"urls"`
	}
	_ = json.Unmarshal(bn.Body.Bytes(), &bnBody)
	shop := postJSON(t, h, http.MethodPatch, "/api/v1/seller/shop", seller.AccessToken, map[string]any{
		"logo": avBody.URLs[0], "banner": bnBody.URLs[0],
	})
	if shop.Code != 200 {
		t.Fatalf("patch shop: %d %s", shop.Code, shop.Body.String())
	}

	st := postMultipart(t, h, "/api/v1/uploads", seller.AccessToken, "story", [][]byte{png1x1}, []string{"st.png"})
	var stBody struct {
		URLs []string `json:"urls"`
	}
	_ = json.Unmarshal(st.Body.Bytes(), &stBody)
	story := postJSON(t, h, http.MethodPost, "/api/v1/seller/stories", seller.AccessToken, map[string]any{
		"image": stBody.URLs[0], "caption": "сторис",
	})
	if story.Code != http.StatusCreated && story.Code != 200 {
		t.Fatalf("story: %d %s", story.Code, story.Body.String())
	}

	mp := httptest.NewRequest(http.MethodPost, "/api/v1/seller/products", strings.NewReader("--x"))
	mp.Header.Set("Authorization", "Bearer "+seller.AccessToken)
	mp.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	mprec := httptest.NewRecorder()
	h.ServeHTTP(mprec, mp)
	if mprec.Code != http.StatusBadRequest {
		t.Fatalf("multipart product: want 400, got %d %s", mprec.Code, mprec.Body.String())
	}
}

func TestReviewsPublicAndDeliveredGate(t *testing.T) {
	h := testAPI(t)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=5", nil)
	listRec := httptest.NewRecorder()
	h.ServeHTTP(listRec, listReq)
	if listRec.Code != 200 {
		t.Fatalf("catalog: %d %s", listRec.Code, listRec.Body.String())
	}
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &catalog); err != nil || len(catalog.Items) == 0 {
		t.Fatalf("need a public product: %s", listRec.Body.String())
	}
	publicID := catalog.Items[0].ID

	guestProd := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+publicID, nil)
	grec := httptest.NewRecorder()
	h.ServeHTTP(grec, guestProd)
	if grec.Code != 200 {
		t.Fatalf("guest product: %d %s", grec.Code, grec.Body.String())
	}
	if strings.Contains(grec.Body.String(), `"error"`) && grec.Code != 200 {
		t.Fatalf("guest must not get auth error: %s", grec.Body.String())
	}

	guestList := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+publicID+"/reviews?limit=50&sort=new", nil)
	lrec := httptest.NewRecorder()
	h.ServeHTTP(lrec, guestList)
	if lrec.Code != 200 {
		t.Fatalf("guest reviews: %d %s", lrec.Code, lrec.Body.String())
	}
	var pub struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(lrec.Body.Bytes(), &pub); err != nil {
		t.Fatal(err)
	}

	missing := httptest.NewRequest(http.MethodGet, "/api/v1/products/no-such-product/reviews", nil)
	mrec := httptest.NewRecorder()
	h.ServeHTTP(mrec, missing)
	if mrec.Code != http.StatusNotFound {
		t.Fatalf("missing product reviews: want 404, got %d %s", mrec.Code, mrec.Body.String())
	}

	anon := postJSON(t, h, http.MethodPost, "/api/v1/products/"+publicID+"/reviews", "", map[string]any{
		"orderId": "WOA-1", "rating": 5, "text": "Отличный товар, всем советую",
	})
	if anon.Code != http.StatusUnauthorized {
		t.Fatalf("guest POST: want 401, got %d %s", anon.Code, anon.Body.String())
	}

	suffix := time.Now().UnixNano()
	sellerEmail := fmt.Sprintf("seller-rev-%d@woason.ru", suffix)
	buyerEmail := fmt.Sprintf("buyer-rev-%d@woason.ru", suffix)
	regS := postJSON(t, h, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "Продавец Отзывов", "email": sellerEmail, "phone": "+79001112233",
		"password": "123456", "role": "seller", "shopName": "Отзывы", "city": "Казань",
		"delivery": []string{"cdek"},
	})
	if regS.Code != 200 && regS.Code != 201 {
		t.Fatalf("register seller: %d %s", regS.Code, regS.Body.String())
	}
	regB := postJSON(t, h, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "Покупатель Отзывов", "email": buyerEmail, "phone": "+79002223344",
		"password": "123456", "role": "buyer",
	})
	if regB.Code != 200 && regB.Code != 201 {
		t.Fatalf("register buyer: %d %s", regB.Code, regB.Body.String())
	}
	seller := login(t, h, sellerEmail, "123456")
	buyer := login(t, h, buyerEmail, "123456")

	withTok := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+publicID+"/reviews", nil)
	withTok.Header.Set("Authorization", "Bearer "+buyer.AccessToken)
	trec := httptest.NewRecorder()
	h.ServeHTTP(trec, withTok)
	var withTokBody struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(trec.Body.Bytes(), &withTokBody)
	if withTokBody.Total != pub.Total {
		t.Fatalf("token must not shrink public list: guest=%d auth=%d", pub.Total, withTokBody.Total)
	}

	prodRec := postJSON(t, h, http.MethodPost, "/api/v1/seller/products", seller.AccessToken, map[string]any{
		"title": "Товар для отзыва", "price": 900, "category": "dom", "condition": "new",
		"image": "https://example.com/rev.jpg",
	})
	if prodRec.Code >= 300 {
		t.Fatalf("product: %d %s", prodRec.Code, prodRec.Body.String())
	}
	var prod map[string]any
	_ = json.Unmarshal(prodRec.Body.Bytes(), &prod)
	productID, _ := prod["id"].(string)
	if productID == "" {
		t.Fatalf("no product id: %s", prodRec.Body.String())
	}
	removeProduct(t, h, productID)

	cartRec := postJSON(t, h, http.MethodPost, "/api/v1/cart", buyer.AccessToken, map[string]any{"productId": productID, "qty": 1})
	if cartRec.Code >= 300 {
		t.Fatalf("cart: %d %s", cartRec.Code, cartRec.Body.String())
	}
	chk := postJSON(t, h, http.MethodPost, "/api/v1/checkout", buyer.AccessToken, map[string]string{
		"address": "ул. Арбат, 1", "delivery": "cdek", "city": "Москва",
	})
	if chk.Code != http.StatusCreated && chk.Code != 200 {
		t.Fatalf("checkout: %d %s", chk.Code, chk.Body.String())
	}
	var chkBody struct {
		Order struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"order"`
	}
	if err := json.Unmarshal(chk.Body.Bytes(), &chkBody); err != nil || chkBody.Order.ID == "" {
		t.Fatalf("order id: %s", chk.Body.String())
	}

	tooShort := postJSON(t, h, http.MethodPost, "/api/v1/products/"+productID+"/reviews", buyer.AccessToken, map[string]any{
		"orderId": chkBody.Order.ID, "rating": 5, "text": "коротко",
	})
	if tooShort.Code != http.StatusBadRequest {
		t.Fatalf("short text: want 400, got %d %s", tooShort.Code, tooShort.Body.String())
	}
	badRating := postJSON(t, h, http.MethodPost, "/api/v1/products/"+productID+"/reviews", buyer.AccessToken, map[string]any{
		"orderId": chkBody.Order.ID, "rating": 6, "text": "Нормальный товар, упаковка целая",
	})
	if badRating.Code != http.StatusBadRequest {
		t.Fatalf("rating: want 400, got %d %s", badRating.Code, badRating.Body.String())
	}

	before := postJSON(t, h, http.MethodPost, "/api/v1/products/"+productID+"/reviews", buyer.AccessToken, map[string]any{
		"orderId": chkBody.Order.ID, "rating": 5, "text": "Нормальный товар, упаковка целая",
	})
	if before.Code != http.StatusForbidden || !strings.Contains(before.Body.String(), "после получения") {
		t.Fatalf("not delivered: want 403, got %d %s", before.Code, before.Body.String())
	}

	fullBefore := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+productID+"/reviews", nil)
	frec := httptest.NewRecorder()
	h.ServeHTTP(frec, fullBefore)
	if frec.Code != 200 {
		t.Fatalf("GET still public after 403: %d %s", frec.Code, frec.Body.String())
	}

	st := postJSON(t, h, http.MethodPost, "/api/v1/seller/orders/"+chkBody.Order.ID+"/status", seller.AccessToken, map[string]string{
		"status": "delivered",
	})
	if st.Code != 200 {
		t.Fatalf("deliver: %d %s", st.Code, st.Body.String())
	}

	pending := httptest.NewRequest(http.MethodGet, "/api/v1/cabinet/reviews/pending", nil)
	pending.Header.Set("Authorization", "Bearer "+buyer.AccessToken)
	prec := httptest.NewRecorder()
	h.ServeHTTP(prec, pending)
	if prec.Code != 200 || !strings.Contains(prec.Body.String(), productID) {
		t.Fatalf("pending: %d %s", prec.Code, prec.Body.String())
	}

	up := postMultipart(t, h, "/api/v1/uploads", buyer.AccessToken, "review", [][]byte{png1x1}, []string{"r.png"})
	if up.Code != 200 {
		t.Fatalf("review upload: %d %s", up.Code, up.Body.String())
	}
	var upBody struct {
		URLs []string `json:"urls"`
	}
	_ = json.Unmarshal(up.Body.Bytes(), &upBody)
	if len(upBody.URLs) != 1 || !strings.Contains(upBody.URLs[0], "/uploads/review/") {
		t.Fatalf("review url: %s", up.Body.String())
	}
	tooMany := postMultipart(t, h, "/api/v1/uploads", buyer.AccessToken, "review",
		[][]byte{png1x1, png1x1, png1x1, png1x1, png1x1},
		[]string{"1.png", "2.png", "3.png", "4.png", "5.png"})
	if tooMany.Code != http.StatusBadRequest {
		t.Fatalf("5 review files: want 400, got %d %s", tooMany.Code, tooMany.Body.String())
	}

	created := postJSON(t, h, http.MethodPost, "/api/v1/products/"+productID+"/reviews", buyer.AccessToken, map[string]any{
		"orderId": chkBody.Order.ID, "rating": 5, "text": "Получил заказ, всё отлично, рекомендую",
		"photos": upBody.URLs,
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d %s", created.Code, created.Body.String())
	}
	var rev map[string]any
	_ = json.Unmarshal(created.Body.Bytes(), &rev)
	reviewID, _ := rev["id"].(string)
	if reviewID == "" || rev["productId"] != productID || rev["author"] != "Покупатель Отзывов" {
		t.Fatalf("review payload: %s", created.Body.String())
	}

	dup := postJSON(t, h, http.MethodPost, "/api/v1/products/"+productID+"/reviews", buyer.AccessToken, map[string]any{
		"orderId": chkBody.Order.ID, "rating": 4, "text": "Повторный отзыв не должен пройти никогда",
	})
	if dup.Code != http.StatusConflict {
		t.Fatalf("dup: want 409, got %d %s", dup.Code, dup.Body.String())
	}

	guestAfter := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+productID, nil)
	garec := httptest.NewRecorder()
	h.ServeHTTP(garec, guestAfter)
	if garec.Code != 200 {
		t.Fatalf("guest product after: %d %s", garec.Code, garec.Body.String())
	}
	if !strings.Contains(garec.Body.String(), "Получил заказ") {
		t.Fatalf("guest must see new review: %s", garec.Body.String())
	}
	var afterProd struct {
		Rating       float64 `json:"rating"`
		ReviewsCount int     `json:"reviewsCount"`
	}
	if err := json.Unmarshal(garec.Body.Bytes(), &afterProd); err != nil {
		t.Fatal(err)
	}
	if afterProd.ReviewsCount < 1 || afterProd.Rating < 1 {
		t.Fatalf("rating not updated: %+v %s", afterProd, garec.Body.String())
	}

	mine := httptest.NewRequest(http.MethodGet, "/api/v1/cabinet/reviews?limit=100", nil)
	mine.Header.Set("Authorization", "Bearer "+buyer.AccessToken)
	mineRec := httptest.NewRecorder()
	h.ServeHTTP(mineRec, mine)
	if mineRec.Code != 200 || !strings.Contains(mineRec.Body.String(), productID) || !strings.Contains(mineRec.Body.String(), "productTitle") {
		t.Fatalf("cabinet reviews: %d %s", mineRec.Code, mineRec.Body.String())
	}

	pendingAfter := httptest.NewRequest(http.MethodGet, "/api/v1/cabinet/reviews/pending", nil)
	pendingAfter.Header.Set("Authorization", "Bearer "+buyer.AccessToken)
	parec := httptest.NewRecorder()
	h.ServeHTTP(parec, pendingAfter)
	if parec.Code != 200 || strings.Contains(parec.Body.String(), productID) {
		t.Fatalf("pending after review: %d %s", parec.Code, parec.Body.String())
	}

	orders := httptest.NewRequest(http.MethodGet, "/api/v1/cabinet/orders", nil)
	orders.Header.Set("Authorization", "Bearer "+seller.AccessToken)
	orec := httptest.NewRecorder()
	h.ServeHTTP(orec, orders)
	if orec.Code != 200 {
		t.Fatalf("seller cabinet orders: %d %s", orec.Code, orec.Body.String())
	}
	if strings.Contains(orec.Body.String(), chkBody.Order.ID) {
		t.Fatalf("seller cabinet/orders must be purchases, not sales: %s", orec.Body.String())
	}

	srev := httptest.NewRequest(http.MethodGet, "/api/v1/seller/reviews?limit=100", nil)
	srev.Header.Set("Authorization", "Bearer "+seller.AccessToken)
	srec := httptest.NewRecorder()
	h.ServeHTTP(srec, srev)
	if srec.Code != 200 || !strings.Contains(srec.Body.String(), reviewID) {
		t.Fatalf("seller reviews: %d %s", srec.Code, srec.Body.String())
	}

	reply := postJSON(t, h, http.MethodPost, "/api/v1/seller/reviews/"+reviewID+"/reply", seller.AccessToken, map[string]any{
		"text": "Спасибо за отзыв!",
	})
	if reply.Code != 200 || !strings.Contains(reply.Body.String(), "Спасибо за отзыв") {
		t.Fatalf("reply: %d %s", reply.Code, reply.Body.String())
	}

	guestReply := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+productID+"/reviews", nil)
	grrec := httptest.NewRecorder()
	h.ServeHTTP(grrec, guestReply)
	if !strings.Contains(grrec.Body.String(), "Спасибо за отзыв") {
		t.Fatalf("public must show seller reply: %s", grrec.Body.String())
	}

	stranger := postJSON(t, h, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name": "Чужой магазин", "email": fmt.Sprintf("seller-other-%d@woason.ru", suffix),
		"phone": "+79003334455", "password": "123456", "role": "seller", "shopName": "Другой",
		"delivery": []string{"cdek"},
	})
	if stranger.Code != 200 && stranger.Code != 201 {
		t.Fatalf("register other seller: %d %s", stranger.Code, stranger.Body.String())
	}
	other := login(t, h, fmt.Sprintf("seller-other-%d@woason.ru", suffix), "123456")
	stolen := postJSON(t, h, http.MethodPost, "/api/v1/seller/reviews/"+reviewID+"/reply", other.AccessToken, map[string]any{
		"text": "Это не мой товар",
	})
	if stolen.Code != http.StatusForbidden {
		t.Fatalf("foreign reply: want 403, got %d %s", stolen.Code, stolen.Body.String())
	}

	ownCart := postJSON(t, h, http.MethodPost, "/api/v1/cart", seller.AccessToken, map[string]any{"productId": productID, "qty": 1})
	if ownCart.Code >= 300 {
		t.Fatalf("seller cart: %d %s", ownCart.Code, ownCart.Body.String())
	}
	ownChk := postJSON(t, h, http.MethodPost, "/api/v1/checkout", seller.AccessToken, map[string]string{
		"address": "ул. Тверская, 1", "delivery": "cdek", "city": "Москва",
	})
	if ownChk.Code != http.StatusCreated && ownChk.Code != 200 {
		t.Fatalf("seller checkout: %d %s", ownChk.Code, ownChk.Body.String())
	}
	var ownBody struct {
		Order struct {
			ID string `json:"id"`
		} `json:"order"`
	}
	_ = json.Unmarshal(ownChk.Body.Bytes(), &ownBody)
	ownSt := postJSON(t, h, http.MethodPost, "/api/v1/seller/orders/"+ownBody.Order.ID+"/status", seller.AccessToken, map[string]string{
		"status": "delivered",
	})
	if ownSt.Code != 200 {
		t.Fatalf("seller deliver own: %d %s", ownSt.Code, ownSt.Body.String())
	}
	ownRev := postJSON(t, h, http.MethodPost, "/api/v1/products/"+productID+"/reviews", seller.AccessToken, map[string]any{
		"orderId": ownBody.Order.ID, "rating": 5, "text": "Свой товар оценивать нельзя совсем",
	})
	if ownRev.Code != http.StatusForbidden || !strings.Contains(ownRev.Body.String(), "свой товар") {
		t.Fatalf("own product: want 403, got %d %s", ownRev.Code, ownRev.Body.String())
	}
}
