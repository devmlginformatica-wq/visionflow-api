package home

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/celsolenonsander/visionflow-api/internal/authcontext"
	"github.com/celsolenonsander/visionflow-api/internal/db"
)

type Service struct {
	pool       *db.Pool
	repository *Repository
}

func New(pool *db.Pool) *Service {
	return &Service{pool: pool, repository: NewRepository()}
}

func (s *Service) Indicators(ctx context.Context, tenantID, empresaID, userID int) (map[string]any, error) {
	var result map[string]any
	err := s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		var err error
		result, err = s.repository.Indicators(ctx, tx, tenantID, empresaID, userID)
		return err
	})
	return result, err
}

func (s *Service) withSession(ctx context.Context, tenantID, empresaID int, fn func(pgx.Tx) error) error {
	session, err := db.NewSession(ctx, s.pool, tenantID, empresaID, authcontext.UserID(ctx))
	if err != nil {
		return err
	}
	defer session.Rollback(ctx)
	if err := fn(session.Tx); err != nil {
		return err
	}
	return session.Commit(ctx)
}
