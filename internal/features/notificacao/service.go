package notificacao

import (
	"context"

	"github.com/celsolenonsander/visionflow-api/internal/db"
)

type Service struct {
	pool       *db.Pool
	repository *Repository
}

func New(pool *db.Pool) *Service {
	return &Service{pool: pool, repository: NewRepository()}
}

func (s *Service) List(ctx context.Context, tenantID, empresaID, userID, limit, cursor int, read *bool) ([]map[string]any, string, error) {
	session, err := db.NewSession(ctx, s.pool, tenantID, empresaID, userID)
	if err != nil {
		return nil, "", err
	}
	defer session.Rollback(ctx)

	items, err := s.repository.List(ctx, session.Tx, tenantID, empresaID, userID, cursor, limit, read)
	if err != nil {
		return nil, "", err
	}
	items, next := db.CursorPage(items, limit, "id")
	if err := session.Commit(ctx); err != nil {
		return nil, "", err
	}
	return items, next, nil
}

func (s *Service) CountUnread(ctx context.Context, tenantID, empresaID, userID int) (int, error) {
	session, err := db.NewSession(ctx, s.pool, tenantID, empresaID, userID)
	if err != nil {
		return 0, err
	}
	defer session.Rollback(ctx)

	count, err := s.repository.CountUnread(ctx, session.Tx, tenantID, empresaID, userID)
	if err != nil {
		return 0, err
	}
	if err := session.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}
