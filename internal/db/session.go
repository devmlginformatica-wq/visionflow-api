package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Session representa uma transação de banco com contexto de tenant/empresa/
// usuário já injetado via SET LOCAL.
//
// O RLS filtra automaticamente todas as queries executadas nessa transação.
//
// Padrão de uso:
//
//	sess, err := db.NewSession(ctx, pool, tenantID, empresaID, userID)
//	if err != nil { ... }
//	defer sess.Rollback(ctx) // no-op se Commit já foi chamado
//
//	// ... usa sess.Tx como se fosse pool ...
//
//	if err := sess.Commit(ctx); err != nil { ... }
type Session struct {
	Tx        pgx.Tx
	TenantID  int
	EmpresaID int
}

// NewSession abre uma transação e injeta o contexto de RLS.
func NewSession(ctx context.Context, pool *Pool, tenantID, empresaID, userID int) (*Session, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("iniciar transação: %w", err)
	}

	// SET LOCAL vale só até o COMMIT/ROLLBACK desta transação.
	// Não vaza entre conexões do pool.
	if _, err := tx.Exec(ctx,
		"SELECT set_config('app.cod_tenant', $1, true)",
		fmt.Sprintf("%d", tenantID),
	); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("set app.cod_tenant: %w", err)
	}

	if _, err := tx.Exec(ctx,
		"SELECT set_config('app.cod_empresa', $1, true)",
		fmt.Sprintf("%d", empresaID),
	); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("set app.cod_empresa: %w", err)
	}

	userIDSetting := ""
	if userID > 0 {
		userIDSetting = fmt.Sprintf("%d", userID)
	}
	if _, err := tx.Exec(ctx,
		"SELECT set_config('app.cod_usuario', $1, true)",
		userIDSetting,
	); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("set app.cod_usuario: %w", err)
	}

	return &Session{
		Tx:        tx,
		TenantID:  tenantID,
		EmpresaID: empresaID,
	}, nil
}

// Commit finaliza a transação.
func (s *Session) Commit(ctx context.Context) error {
	return s.Tx.Commit(ctx)
}

// Rollback é no-op se a transação já foi commitada.
// Use sempre com defer pra garantir cleanup.
func (s *Session) Rollback(ctx context.Context) {
	_ = s.Tx.Rollback(ctx)
}
