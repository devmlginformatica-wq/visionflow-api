package home

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/celsolenonsander/visionflow-api/internal/db"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) Indicators(ctx context.Context, tx pgx.Tx, tenantID, empresaID, userID int) (map[string]any, error) {
	const query = `
		SELECT
		  COALESCE((SELECT SUM(pi.valor_total_produtos_liquido)
		    FROM pedido p JOIN pedido_item pi ON pi.cod_tenant=p.cod_tenant
		      AND pi.cod_empresa=p.cod_empresa
		      AND (pi.pedido_id=p.codigo OR (pi.pedido_id IS NULL AND pi.codigo_erp_pedido=p.codigo_erp))
		    WHERE p.cod_tenant=$1 AND p.cod_empresa=$2
		      AND COALESCE(p.apagar_dados_pedido,false)=false AND p.e_um_orcamento=false
		      AND p.situacao_app='aprovado'
		      AND p.data_emissao >= date_trunc('month',CURRENT_DATE)::date
		      AND p.data_emissao < (date_trunc('month',CURRENT_DATE)+interval '1 month')::date),0) AS vendas_mes,
		  COALESCE((SELECT SUM(pi.valor_total_produtos_liquido)
		    FROM pedido p JOIN pedido_item pi ON pi.cod_tenant=p.cod_tenant
		      AND pi.cod_empresa=p.cod_empresa
		      AND (pi.pedido_id=p.codigo OR (pi.pedido_id IS NULL AND pi.codigo_erp_pedido=p.codigo_erp))
		    WHERE p.cod_tenant=$1 AND p.cod_empresa=$2
		      AND COALESCE(p.apagar_dados_pedido,false)=false AND p.e_um_orcamento=false
		      AND p.situacao_app='aprovado'
		      AND p.data_emissao=CURRENT_DATE),0) AS vendas_hoje,
		  (SELECT COUNT(*) FROM pedido WHERE cod_tenant=$1 AND cod_empresa=$2
		    AND COALESCE(apagar_dados_pedido,false)=false AND situacao_app='em_digitacao'
		    AND e_um_orcamento=false) AS pedidos_em_digitacao,
		  (SELECT COUNT(*) FROM pedido WHERE cod_tenant=$1 AND cod_empresa=$2
		    AND COALESCE(apagar_dados_pedido,false)=false AND situacao_app='em_digitacao'
		    AND e_um_orcamento=true) AS orcamentos_em_digitacao,
		  (SELECT COUNT(*) FROM cliente
		    WHERE cod_tenant=$1 AND cod_empresa=$2 AND COALESCE(prospect,false)=true) AS clientes_em_digitacao,
		  (SELECT COUNT(*) FROM pedido WHERE cod_tenant=$1 AND cod_empresa=$2
		    AND COALESCE(apagar_dados_pedido,false)=false AND situacao_app='aguardando_aprovacao'
		    AND e_um_orcamento=false) AS pedidos_aguardando_aprovacao,
		  (SELECT COUNT(*) FROM agenda WHERE cod_tenant=$1 AND cod_empresa=$2
		    AND cod_usuario=$3 AND concluida=false) AS agendas_pendentes,
		  (SELECT COUNT(*) FROM tarefa WHERE cod_tenant=$1 AND cod_empresa=$2
		    AND cod_usuario=$3 AND concluida=false) AS tarefas_pendentes,
		  (SELECT COUNT(*) FROM notificacao WHERE cod_tenant=$1 AND cod_empresa=$2
		    AND cod_usuario=$3 AND lida=false) AS notificacoes_nao_lidas`
	rows, err := tx.Query(ctx, query, tenantID, empresaID, userID)
	if err != nil {
		return nil, fmt.Errorf("consultar indicadores: %w", err)
	}
	items, err := db.ReadRows(rows)
	if err != nil {
		return nil, fmt.Errorf("ler indicadores: %w", err)
	}
	if len(items) != 1 {
		return nil, errors.New("consulta de indicadores não retornou uma linha")
	}
	return items[0], nil
}
