package auth

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/celsolenonsander/visionflow-api/internal/db"
	"github.com/celsolenonsander/visionflow-api/internal/models"
)

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Repository struct {
	db rowQuerier
}

func NewRepository(pool *db.Pool) *Repository {
	return &Repository{db: pool}
}

func (r *Repository) Authenticate(ctx context.Context, email, password string) (models.Usuario, error) {
	const query = `
		SELECT u.cod_usuario, u.nome, u.email, u.tipo
		FROM usuario u
		JOIN usuario_empresa ue ON ue.cod_usuario=u.cod_usuario
		JOIN empresa e ON e.cod_tenant=ue.cod_tenant AND e.cod_empresa=ue.cod_empresa
		JOIN tenant t ON t.cod_tenant=u.cod_tenant
		WHERE lower(u.email)=lower($1)
		  AND u.ativo=true AND e.ativo=true AND t.ativo=true
		  AND ue.cod_tenant=u.cod_tenant
		  AND crypt($2,u.senha_hash)=u.senha_hash
		ORDER BY ue.empresa_padrao DESC, ue.cod_empresa ASC
		LIMIT 1`

	var user models.Usuario
	err := r.db.QueryRow(ctx, query, email, password).Scan(&user.ID, &user.Nome, &user.Email, &user.Tipo)
	if err != nil {
		return models.Usuario{}, err
	}
	return user, nil
}
