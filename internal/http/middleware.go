package api

import (
	"context"
	"net/http"
	"strings"

	"woason-api/internal/auth"
	"woason-api/internal/httpx"
	"woason-api/internal/models"
)

func CORS(frontendURL string) func(http.Handler) http.Handler {
	allow := map[string]struct{}{
		frontendURL:             {},
		"http://localhost:3000": {},
		"http://127.0.0.1:3000": {},
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := allow[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func OptionalAuth(tokens *auth.Tokens) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if p := bearerPrincipal(tokens, r); p != nil {
				r = r.WithContext(httpx.WithPrincipal(r.Context(), p))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAuth(tokens *auth.Tokens, lookup func(ctx context.Context, id string) (*models.User, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := bearerPrincipal(tokens, r)
			if p == nil {
				httpx.Error(w, http.StatusUnauthorized, "нужна авторизация")
				return
			}
			u, err := lookup(r.Context(), p.ID)
			if err != nil || u == nil {
				httpx.Error(w, http.StatusUnauthorized, "сессия недействительна")
				return
			}
			if u.BannedAt != nil {
				httpx.Error(w, http.StatusForbidden, "аккаунт заблокирован")
				return
			}
			p.Role = u.Role
			p.Email = u.Email
			next.ServeHTTP(w, r.WithContext(httpx.WithPrincipal(r.Context(), p)))
		})
	}
}

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := httpx.PrincipalFrom(r)
			if p == nil || p.Role != role {
				httpx.Error(w, http.StatusForbidden, "недостаточно прав")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireSeller() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := httpx.PrincipalFrom(r)
			if p == nil || (p.Role != models.RoleSeller && p.Role != models.RoleAdmin) {
				httpx.Error(w, http.StatusForbidden, "доступ только для продавца")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearerPrincipal(tokens *auth.Tokens, r *http.Request) *models.Principal {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return nil
	}
	raw := strings.TrimSpace(h[7:])
	p, err := tokens.ParseAccess(raw)
	if err != nil {
		return nil
	}
	return p
}
