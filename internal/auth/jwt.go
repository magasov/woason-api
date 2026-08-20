package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"woason-api/internal/models"
)

type Tokens struct {
	secret []byte
}

func NewTokens(secret string) *Tokens {
	return &Tokens{secret: []byte(secret)}
}

type claims struct {
	Role  string `json:"role"`
	Email string `json:"email"`
	jwt.RegisteredClaims
}

func (t *Tokens) IssueAccess(u *models.User) (string, error) {
	now := time.Now()
	c := claims{
		Role:  u.Role,
		Email: u.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return tok.SignedString(t.secret)
}

func (t *Tokens) ParseAccess(raw string) (*models.Principal, error) {
	tok, err := jwt.ParseWithClaims(raw, &claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("алгоритм")
		}
		return t.secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, fmt.Errorf("недействительный токен")
	}
	c, ok := tok.Claims.(*claims)
	if !ok || c.Subject == "" {
		return nil, fmt.Errorf("недействительный токен")
	}
	return &models.Principal{ID: c.Subject, Role: c.Role, Email: c.Email}, nil
}

func NewRefreshToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(sum[:]), nil
}

func HashRefresh(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func RefreshTTL() time.Duration {
	return 7 * 24 * time.Hour
}
