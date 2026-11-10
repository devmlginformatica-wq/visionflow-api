package tarefa

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/celsolenonsander/visionflow-api/internal/authcontext"
	"github.com/celsolenonsander/visionflow-api/internal/db"
	"github.com/celsolenonsander/visionflow-api/internal/models"
	"github.com/celsolenonsander/visionflow-api/internal/platform/apperrors"
)

type Service struct {
	pool       *db.Pool
	repository *Repository
}

func New(pool *db.Pool) *Service {
	return &Service{pool: pool, repository: NewRepository()}
}

func (s *Service) List(ctx context.Context, tenantID, empresaID, userID, limit, cursor int, completed *bool, priority string) ([]map[string]any, string, error) {
	var items []map[string]any
	var next string
	err := s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		var err error
		items, err = s.repository.List(ctx, tx, tenantID, empresaID, userID, limit, cursor, completed, priority)
		if err != nil {
			return err
		}
		items, next = db.CursorPage(items, limit, "id")
		return nil
	})
	return items, next, err
}

func (s *Service) Get(ctx context.Context, tenantID, empresaID, userID, id int) (map[string]any, error) {
	var result map[string]any
	err := s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		var err error
		result, err = s.repository.Get(ctx, tx, tenantID, empresaID, userID, id)
		return err
	})
	return result, err
}

func (s *Service) Create(ctx context.Context, tenantID, empresaID, userID int, input models.TarefaCreate) (map[string]any, error) {
	due, err := validateInput(&input)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		var err error
		result, err = s.repository.Create(ctx, tx, tenantID, empresaID, userID, input, due)
		return err
	})
	return result, err
}

func (s *Service) Update(ctx context.Context, tenantID, empresaID, userID, id int, input models.TarefaCreate) (map[string]any, error) {
	due, err := validateInput(&input)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		var err error
		result, err = s.repository.Update(ctx, tx, tenantID, empresaID, userID, id, input, due)
		return err
	})
	return result, err
}

func (s *Service) Complete(ctx context.Context, tenantID, empresaID, userID, id int) (map[string]any, error) {
	var result map[string]any
	err := s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		var err error
		result, err = s.repository.Complete(ctx, tx, tenantID, empresaID, userID, id)
		return err
	})
	return result, err
}

func (s *Service) Delete(ctx context.Context, tenantID, empresaID, userID, id int) error {
	return s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		return s.repository.Delete(ctx, tx, tenantID, empresaID, userID, id)
	})
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

func validateInput(input *models.TarefaCreate) (*time.Time, error) {
	if strings.TrimSpace(input.Titulo) == "" {
		return nil, apperrors.InvalidInput("titulo é obrigatório")
	}
	if len([]rune(input.Titulo)) > 200 {
		return nil, apperrors.InvalidInput("titulo excede 200 caracteres")
	}
	if input.Prioridade == "" {
		input.Prioridade = "media"
	}
	if input.Prioridade != "baixa" && input.Prioridade != "media" && input.Prioridade != "alta" {
		return nil, apperrors.InvalidInput("prioridade deve ser baixa, media ou alta")
	}
	if input.Prazo == nil {
		return nil, nil
	}
	due, err := time.Parse(time.RFC3339, *input.Prazo)
	if err != nil {
		return nil, apperrors.InvalidInput("data deve estar no formato ISO 8601 com timezone")
	}
	due = due.UTC()
	return &due, nil
}
