package catalogo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/celsolenonsander/visionflow-api/internal/db"
)

type Repository struct{}

var ErrInvalidFilter = errors.New("filtro de preço do produto inválido")

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) ValidateProductReferences(ctx context.Context, tx pgx.Tx, tenantID, empresaID int, customerID, priceTableID *int) error {
	if customerID != nil {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM cliente WHERE cod_tenant=$1 AND cod_empresa=$2
			  AND codigo=$3 AND deletado_em IS NULL)`, tenantID, empresaID, *customerID).Scan(&exists); err != nil {
			return fmt.Errorf("validar cliente para preço: %w", err)
		}
		if !exists {
			return fmt.Errorf("%w: cliente_id", ErrInvalidFilter)
		}
	}
	if priceTableID != nil {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM tabela_preco WHERE cod_tenant=$1 AND cod_empresa=$2
			  AND codigo=$3 AND COALESCE(ativo,true))`, tenantID, empresaID, *priceTableID).Scan(&exists); err != nil {
			return fmt.Errorf("validar tabela de preço do produto: %w", err)
		}
		if !exists {
			return fmt.Errorf("%w: tabela_preco_id", ErrInvalidFilter)
		}
	}
	return nil
}

func (r *Repository) List(ctx context.Context, tx pgx.Tx, tenantID, empresaID, userID, cursor, limit int, resource, search string) ([]map[string]any, error) {
	return r.ListWithPricing(ctx, tx, tenantID, empresaID, userID, cursor, limit, resource, search, nil, nil)
}

func (r *Repository) ListWithPricing(ctx context.Context, tx pgx.Tx, tenantID, empresaID, userID, cursor, limit int, resource, search string, customerID, priceTableID *int) ([]map[string]any, error) {
	query, ok := queryFor(resource)
	if !ok {
		return nil, fmt.Errorf("catálogo desconhecido: %s", resource)
	}

	var rows pgx.Rows
	var err error
	switch resource {
	case "produtos":
		rows, err = tx.Query(ctx, query, cursor, search, limit+1, tenantID, empresaID, priceTableID, customerID)
	case "empresas":
		rows, err = tx.Query(ctx, query, cursor, limit+1, tenantID, userID)
	default:
		rows, err = tx.Query(ctx, query, cursor, limit+1, tenantID, empresaID)
	}
	if err != nil {
		return nil, fmt.Errorf("consultar catálogo %s: %w", resource, err)
	}
	items, err := db.ReadRows(rows)
	if err != nil {
		return nil, fmt.Errorf("ler catálogo %s: %w", resource, err)
	}
	return items, nil
}

func queryFor(resource string) (string, bool) {
	query, ok := catalogQueries[resource]
	return query, ok
}

const productSearchPredicate = `($2='' OR
	public.unaccent_immutable(i.codigo_erp::text) ILIKE public.unaccent_immutable('%'||$2||'%')
	OR public.unaccent_immutable(i.descricao) ILIKE public.unaccent_immutable('%'||$2||'%'))`

var catalogQueries = map[string]string{
	"empresas": `SELECT e.cod_empresa AS id, COALESCE(e.nome_fantasia, e.razao_social) AS nome
		FROM empresa e JOIN usuario_empresa ue ON ue.cod_tenant=e.cod_tenant
		  AND ue.cod_empresa=e.cod_empresa AND ue.cod_usuario=$4
		WHERE e.cod_tenant=$3 AND e.cod_empresa > $1 AND COALESCE(e.ativo, true)
		ORDER BY e.cod_empresa LIMIT $2`,
	"condicoes-pagamento": `SELECT codigo AS id, codigo_erp, descricao, quantidade_parcelas, prazo_medio
		FROM condicao_pagamento WHERE codigo > $1 AND COALESCE(ativo, true)
		  AND cod_tenant=$3 AND cod_empresa=$4
		ORDER BY codigo LIMIT $2`,
	"formas-pagamento": `SELECT codigo AS id, codigo_erp, descricao
		FROM forma_pagamento WHERE codigo > $1 AND COALESCE(ativo, true)
		  AND cod_tenant=$3 AND cod_empresa=$4
		ORDER BY codigo LIMIT $2`,
	"transportadoras": `SELECT codigo AS id, codigo_erp, nome_razao, cnpj_cpf
		FROM transportadora WHERE codigo > $1 AND COALESCE(ativo, true)
		  AND cod_tenant=$3 AND cod_empresa=$4
		ORDER BY codigo LIMIT $2`,
	"tabelas-preco": `SELECT t.codigo AS id, t.codigo_erp, t.descricao,
		  CASE WHEN MAX(i.validade_inicial) FILTER (WHERE
		    (i.validade_inicial IS NULL OR i.validade_inicial<=CURRENT_DATE)
		    AND (i.validade_final IS NULL OR i.validade_final>=CURRENT_DATE)) IS NULL THEN NULL ELSE
		    to_char(MAX(i.validade_inicial) FILTER (WHERE
		      (i.validade_inicial IS NULL OR i.validade_inicial<=CURRENT_DATE)
		      AND (i.validade_final IS NULL OR i.validade_final>=CURRENT_DATE))::timestamp AT TIME ZONE 'UTC',
		      'YYYY-MM-DD"T"HH24:MI:SS"Z"') END AS vigencia
		FROM tabela_preco t LEFT JOIN tabela_preco_item i
		  ON i.cod_tenant=t.cod_tenant AND i.cod_empresa=t.cod_empresa
		  AND i.codigo_erp_tabela_preco=t.codigo_erp
		WHERE t.codigo > $1 AND COALESCE(t.ativo, true)
		  AND t.cod_tenant=$3 AND t.cod_empresa=$4
		GROUP BY t.codigo,t.codigo_erp,t.descricao
		ORDER BY t.codigo LIMIT $2`,
	"produtos": `SELECT i.codigo AS id, i.codigo_erp, i.descricao,
		  COALESCE(i.descricao_categoria_nivel1,i.descricao_categoria_nivel2,i.descricao_categoria_nivel3) AS familia,
		  i.apelido_fantasia_fabricante AS marca, u.sigla AS unidade,
		  CASE WHEN selected_price.table_id IS NULL THEN i.preco_venda
		       ELSE selected_price.preco_venda END AS preco_venda,
		  selected_price.table_id AS tabela_preco_padrao_id,
		  CASE WHEN selected_price.validade_inicial IS NULL THEN NULL ELSE
		    to_char(selected_price.validade_inicial::timestamp AT TIME ZONE 'UTC',
		      'YYYY-MM-DD"T"HH24:MI:SS"Z"') END AS tabela_preco_vigencia,
		  COALESCE((SELECT SUM(ie.quantidade) FROM item_estoque ie
		    WHERE ie.cod_tenant=i.cod_tenant AND ie.cod_empresa=i.cod_empresa
		      AND ie.codigo_erp_item=i.codigo_erp),0) AS stock,
		  COALESCE(i.peso_liquido,0) AS peso_liquido,
		  COALESCE(i.peso_bruto,0) AS peso_bruto,
		  COALESCE(i.cubagem_embalagem,0) AS cubagem,
		  COALESCE(i.comprimento_embalagem,0) AS comprimento,
		  COALESCE(i.largura_embalagem,0) AS largura,
		  COALESCE(i.altura_embalagem,0) AS altura_espessura,
		  COALESCE(i.qtde_volumes_embalagem,0) AS qtde_embalagem,
		  COALESCE(i.permite_venda_sem_estoque,false) AS permite_venda_sem_estoque
		FROM item i LEFT JOIN unidade_medida u
		  ON u.cod_tenant=i.cod_tenant AND u.cod_empresa=i.cod_empresa
		  AND u.codigo_erp=i.codigo_erp_unidade_medida
		LEFT JOIN LATERAL (
		  SELECT tp.codigo AS table_id,tpi.preco_venda,tpi.validade_inicial
		  FROM tabela_preco tp
		  LEFT JOIN LATERAL (
		    SELECT tpi.preco_venda,tpi.validade_inicial
		    FROM tabela_preco_item tpi
		    WHERE tpi.cod_tenant=i.cod_tenant AND tpi.cod_empresa=i.cod_empresa
		      AND tpi.codigo_erp_item=i.codigo_erp
		      AND tpi.codigo_erp_tabela_preco=tp.codigo_erp
		      AND (tpi.validade_inicial IS NULL OR tpi.validade_inicial<=CURRENT_DATE)
		      AND (tpi.validade_final IS NULL OR tpi.validade_final>=CURRENT_DATE)
		    ORDER BY tpi.validade_inicial DESC NULLS LAST,tpi.codigo DESC LIMIT 1
		  ) tpi ON true
		  WHERE tp.cod_tenant=i.cod_tenant AND tp.cod_empresa=i.cod_empresa
		    AND COALESCE($6::integer,
		      (SELECT client_table.codigo FROM tabela_preco client_table
		       JOIN cliente c ON c.cod_tenant=client_table.cod_tenant
		         AND c.cod_empresa=client_table.cod_empresa AND c.codigo=$7
		         AND c.deletado_em IS NULL
		       WHERE client_table.cod_tenant=i.cod_tenant
		         AND client_table.cod_empresa=i.cod_empresa
		         AND client_table.codigo_erp=c.codigo_erp_tabela_preco
		         AND COALESCE(client_table.ativo,true) LIMIT 1),
		      (SELECT default_table.codigo FROM tabela_preco default_table
		       WHERE default_table.cod_tenant=i.cod_tenant
		         AND default_table.cod_empresa=i.cod_empresa
		         AND default_table.codigo_erp=i.codigo_integracao_tabela_preco
		         AND COALESCE(default_table.ativo,true) LIMIT 1)
		    )=tp.codigo AND COALESCE(tp.ativo,true)
		  LIMIT 1
		) selected_price ON true
		WHERE i.codigo > $1 AND COALESCE(i.ativo, true)
		  AND ` + productSearchPredicate + `
		  AND i.cod_tenant=$4 AND i.cod_empresa=$5
		ORDER BY CASE
		  WHEN $2='' THEN 0
		  WHEN public.unaccent_immutable(i.descricao) ILIKE public.unaccent_immutable($2||'%') THEN 0
		  WHEN public.unaccent_immutable(i.codigo_erp::text) ILIKE public.unaccent_immutable($2||'%') THEN 1
		  ELSE 2 END,
		  i.codigo LIMIT $3`,
}
