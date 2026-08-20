package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	MigrationsPath  string
	JWTSecret       string
	FrontendURL     string
	YKassaShopID    string
	YKassaSecretKey string
	YKassaAPIURL    string
	YKassaReturnURL string
	AdminEmail      string
	AdminPassword   string
	PaymentsMock    bool
	UploadDir       string
	PublicBaseURL   string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		HTTPAddr:        env("HTTP_ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		MigrationsPath:  env("MIGRATIONS_PATH", "migrations"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		FrontendURL:     env("FRONTEND_URL", "http://localhost:3000"),
		YKassaShopID:    os.Getenv("YKASSA_SHOP_ID"),
		YKassaSecretKey: os.Getenv("YKASSA_SECRET_KEY"),
		YKassaAPIURL:    env("YKASSA_API_URL", "https://api.yookassa.ru/v3"),
		YKassaReturnURL: env("YKASSA_RETURN_URL", "http://localhost:3000/orders"),
		AdminEmail:      env("ADMIN_EMAIL", "admin@woason.ru"),
		AdminPassword:   env("ADMIN_PASSWORD", "123456"),
		PaymentsMock:    parseBool(os.Getenv("PAYMENTS_MOCK")),
		UploadDir:       env("UPLOAD_DIR", "uploads"),
		PublicBaseURL:   strings.TrimRight(env("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("не задан DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("не задан JWT_SECRET")
	}
	if !cfg.PaymentsMock && (cfg.YKassaShopID == "" || cfg.YKassaSecretKey == "") {
		return Config{}, fmt.Errorf("не заданы YKASSA_SHOP_ID / YKASSA_SECRET_KEY (или включите PAYMENTS_MOCK)")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func parseBool(v string) bool {
	b, _ := strconv.ParseBool(strings.TrimSpace(v))
	return b
}
