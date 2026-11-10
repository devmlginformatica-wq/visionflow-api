package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"

	"github.com/celsolenonsander/visionflow-api/internal/logger"
	"github.com/celsolenonsander/visionflow-api/internal/models"
)

// Auth valida o JWT e injeta userID, tenantID e empresaID no contexto.
func (h *Handler) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractBearerToken(r)
		if tokenStr == "" {
			unauthorized(w, "NAO_AUTENTICADO", "Token ausente.")
			return
		}

		claims, err := h.parseToken(tokenStr)
		if err != nil {
			unauthorized(w, "TOKEN_INVALIDO", "Token inválido ou expirado.")
			return
		}

		userID, err := extractUserID(claims)
		if err != nil {
			unauthorized(w, "TOKEN_INVALIDO", "Token sem sub válido.")
			return
		}

		tenantID, empresaID, err := h.resolveTenantEmpresa(r.Context(), userID)
		if err != nil {
			unauthorized(w, "SEM_EMPRESA", "Usuário sem empresa vinculada.")
			return
		}

		profile, err := extractProfile(claims)
		if err != nil {
			unauthorized(w, "TOKEN_INVALIDO", "Token sem perfil válido.")
			return
		}

		ctx := WithAuthenticatedUserContext(r.Context(), userID, tenantID, empresaID, profile)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggingMiddleware registra todas as requisições HTTP.
func LoggingMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			tenantID, _ := TenantID(r.Context())
			tenant := "-"
			if tenantID > 0 {
				tenant = strconv.Itoa(tenantID)
			}

			log.Printf("%s %s %d %s tenant=%s",
				r.Method,
				r.URL.Path,
				ww.Status(),
				time.Since(start),
				tenant,
			)
		})
	}
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func (h *Handler) parseToken(tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

func extractUserID(claims jwt.MapClaims) (int, error) {
	sub, ok := claims["sub"]
	if !ok {
		return 0, jwt.ErrTokenInvalidClaims
	}
	switch v := sub.(type) {
	case float64:
		if v <= 0 || v != float64(int(v)) {
			return 0, jwt.ErrTokenInvalidClaims
		}
		return int(v), nil
	case string:
		id, err := strconv.Atoi(v)
		if err != nil || id <= 0 {
			return 0, jwt.ErrTokenInvalidClaims
		}
		return id, nil
	default:
		return 0, jwt.ErrTokenInvalidClaims
	}
}

func extractProfile(claims jwt.MapClaims) (string, error) {
	profile, ok := claims["perfil"].(string)
	if !ok {
		return "", jwt.ErrTokenInvalidClaims
	}
	switch profile {
	case "admin", "gerente", "vendedor", "supervisor":
		return profile, nil
	default:
		return "", jwt.ErrTokenInvalidClaims
	}
}

// resolveTenantEmpresa consulta usuario_empresa e retorna a empresa padrão
// do usuário. Roda FORA do RLS (usa Pool direto).
func (h *Handler) resolveTenantEmpresa(ctx context.Context, userID int) (int, int, error) {
	const q = `
		SELECT ue.cod_tenant, ue.cod_empresa
		FROM usuario_empresa ue
		JOIN usuario u ON u.cod_usuario=ue.cod_usuario
		JOIN tenant t ON t.cod_tenant=ue.cod_tenant
		JOIN empresa e ON e.cod_tenant=ue.cod_tenant AND e.cod_empresa=ue.cod_empresa
		WHERE ue.cod_usuario = $1
		  AND ue.cod_tenant=u.cod_tenant
		  AND u.ativo=true AND t.ativo=true AND e.ativo=true
		ORDER BY ue.empresa_padrao DESC, ue.cod_empresa ASC
		LIMIT 1
	`
	var tenantID, empresaID int
	err := h.Pool.QueryRow(ctx, q, userID).Scan(&tenantID, &empresaID)
	if err != nil {
		return 0, 0, err
	}
	return tenantID, empresaID, nil
}

func unauthorized(w http.ResponseWriter, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(models.APIResponse{
		Success: false,
		Data:    nil,
		Error:   &models.APIError{Code: code, Message: msg},
	})
}
