package httpapi

import (
	"context"
	"net/http"

	"github.com/celsolenonsander/visionflow-api/internal/authcontext"
	"github.com/celsolenonsander/visionflow-api/internal/db"
)

type ctxKey string

const (
	ctxKeyUserID    ctxKey = "user_id"
	ctxKeyTenantID  ctxKey = "tenant_id"
	ctxKeyEmpresaID ctxKey = "empresa_id"
)

// WithUserContext injeta os IDs resolvidos pelo middleware no contexto.
func WithUserContext(ctx context.Context, userID, tenantID, empresaID int) context.Context {
	return WithAuthenticatedUserContext(ctx, userID, tenantID, empresaID, authcontext.Profile(ctx))
}

func WithAuthenticatedUserContext(ctx context.Context, userID, tenantID, empresaID int, profile string) context.Context {
	ctx = authcontext.WithIdentity(ctx, userID, profile)
	ctx = context.WithValue(ctx, ctxKeyUserID, userID)
	ctx = context.WithValue(ctx, ctxKeyTenantID, tenantID)
	ctx = context.WithValue(ctx, ctxKeyEmpresaID, empresaID)
	return ctx
}

// UserID retorna o cod_usuario injetado pelo middleware.
func UserID(ctx context.Context) (int, bool) {
	v, ok := ctx.Value(ctxKeyUserID).(int)
	return v, ok
}

// TenantID retorna o cod_tenant injetado pelo middleware.
func TenantID(ctx context.Context) (int, bool) {
	v, ok := ctx.Value(ctxKeyTenantID).(int)
	return v, ok
}

// EmpresaID retorna o cod_empresa injetado pelo middleware.
func EmpresaID(ctx context.Context) (int, bool) {
	v, ok := ctx.Value(ctxKeyEmpresaID).(int)
	return v, ok
}

func Perfil(r *http.Request) string {
	if r == nil {
		return ""
	}
	return authcontext.Profile(r.Context())
}

// Session abre uma sessão de banco com o tenant/empresa do contexto.
func (h *Handler) Session(ctx context.Context) (*db.Session, error) {
	tenantID, _ := TenantID(ctx)
	empresaID, _ := EmpresaID(ctx)
	userID, _ := UserID(ctx)
	return db.NewSession(ctx, h.Pool, tenantID, empresaID, userID)
}
