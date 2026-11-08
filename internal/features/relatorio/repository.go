package relatorio

import (
	"context"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/celsolenonsander/visionflow-api/internal/db"
)

type repository struct {
	pool *db.Pool
}

func newRepository(cfg DBIAConfig) (*repository, error) {
	if !cfg.IsConfigured() {
		return nil, fmt.Errorf("feature de IA não configurada")
	}
	poolCfg, err := pgxpool.ParseConfig(buildIAConnectionURL(cfg))
	if err != nil {
		return nil, fmt.Errorf("parse DSN da IA: %w", err)
	}
	poolCfg.MaxConns = 3
	poolCfg.MinConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("criar pool da IA: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping no banco da IA: %w", err)
	}
	return &repository{pool: &db.Pool{Pool: pool}}, nil
}

func buildIAConnectionURL(cfg DBIAConfig) string {
	connectionURL := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Path:   "/" + cfg.Name,
	}
	query := connectionURL.Query()
	query.Set("sslmode", cfg.SSLMode)
	connectionURL.RawQuery = query.Encode()
	return connectionURL.String()
}

func (r *repository) Execute(ctx context.Context, tenantID, empresaID int, sql string) ([]string, [][]any, error) {
	if r == nil || r.pool == nil {
		return nil, nil, fmt.Errorf("feature de IA não configurada")
	}
	session, err := db.NewSession(ctx, r.pool, tenantID, empresaID)
	if err != nil {
		return nil, nil, err
	}
	defer session.Rollback(ctx)

	rows, err := session.Tx.Query(ctx, sql)
	if err != nil {
		return nil, nil, fmt.Errorf("executar SQL gerado: %w", err)
	}
	defer rows.Close()

	fieldNames := make([]string, len(rows.FieldDescriptions()))
	for i, fd := range rows.FieldDescriptions() {
		fieldNames[i] = string(fd.Name)
	}
	result := make([][]any, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, nil, fmt.Errorf("ler resultado do SQL: %w", err)
		}
		result = append(result, values)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if err := session.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("confirmar transação da IA: %w", err)
	}
	return fieldNames, result, nil
}
