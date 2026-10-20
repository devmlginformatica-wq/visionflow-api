package cliente

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/celsolenonsander/visionflow-api/internal/db"
	"github.com/celsolenonsander/visionflow-api/internal/models"
	"github.com/celsolenonsander/visionflow-api/internal/platform/apperrors"
)

type Repository struct{}

const clientSearchPredicate = `($4 = '' OR
	public.unaccent_immutable(c.nome_razao_social) ILIKE public.unaccent_immutable('%' || $4 || '%')
	OR public.unaccent_immutable(COALESCE(c.nome_fantasia, '')) ILIKE public.unaccent_immutable('%' || $4 || '%')
	OR public.unaccent_immutable(COALESCE(c.cnpj_cpf, '')) ILIKE public.unaccent_immutable('%' || $4 || '%')
	OR public.unaccent_immutable(c.codigo_erp::text) ILIKE public.unaccent_immutable('%' || $4 || '%'))`

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) List(ctx context.Context, tx pgx.Tx, tenantID, empresaID int, search, status string, limit, cursor int) ([]map[string]any, error) {
	const query = `
		SELECT c.codigo AS id, c.codigo_erp, c.situacao_app AS status_integracao,
		       c.motivo_rejeicao, c.nome_razao_social, c.nome_fantasia, c.cnpj_cpf,
		       CASE c.tipo_pessoa WHEN 'F' THEN 'PF' WHEN 'J' THEN 'PJ' ELSE NULL END AS tipo_pessoa,
		       c.cidade, c.estado, COALESCE(c.ativo, true) AS ativo,
		       COALESCE(c.prospect, false) AS prospect,
		       CASE WHEN NOT COALESCE(c.ativo,true) THEN 'inativo'
		            WHEN (c.data_bloqueio IS NOT NULL AND c.data_bloqueio<=CURRENT_DATE)
		              OR NULLIF(BTRIM(c.motivo_bloqueio),'') IS NOT NULL THEN 'bloqueado'
		            ELSE 'aberto' END AS situacao_cadastro,
		       CASE WHEN NOT COALESCE(c.ativo,true)
		              OR (c.data_bloqueio IS NOT NULL AND c.data_bloqueio<=CURRENT_DATE)
		              OR NULLIF(BTRIM(c.motivo_bloqueio),'') IS NOT NULL THEN 'bloqueado'
		            WHEN last_order.codigo IS NULL THEN 'sem_compra'
		            WHEN last_order.situacao_app='em_digitacao' THEN 'em_digitacao'
		            ELSE 'aberto' END AS situacao_compra,
		       CASE WHEN c.data_inclusao IS NULL THEN NULL ELSE
		         to_char(c.data_inclusao,'YYYY-MM-DD') END AS data_inclusao,
		       NULLIF(BTRIM(CONCAT_WS(', ',
		         NULLIF(BTRIM(CONCAT_WS(' ',c.logradouro,c.numero_endereco)), ''),
		         NULLIF(c.bairro,''))), '') AS endereco_resumo,
		       NULLIF(BTRIM(CONCAT_WS(', ',
		         NULLIF(BTRIM(CONCAT_WS(' ',c.logradouro,c.numero_endereco)), ''),
		         NULLIF(c.complemento,''),NULLIF(c.bairro,''),NULLIF(c.cidade,''),
		         NULLIF(c.estado,''),NULLIF(c.cep,''))), '') AS endereco_completo,
		       CASE WHEN last_order.data_emissao IS NULL THEN NULL ELSE
		         to_char(last_order.data_emissao,'YYYY-MM-DD') END AS ultimo_pedido,
		       CASE WHEN last_attendance.data_agendada IS NULL THEN NULL ELSE
		         to_char(last_attendance.data_agendada,'YYYY-MM-DD') END AS ultimo_atendimento,
		       CASE WHEN c.data_ultima_visita IS NULL THEN NULL ELSE
		         to_char(c.data_ultima_visita,'YYYY-MM-DD') END AS data_ultima_visita,
		       EXISTS (
		         SELECT 1 FROM cliente_conta_receber cr
		         WHERE cr.cod_tenant=c.cod_tenant AND cr.cod_empresa=c.cod_empresa
		           AND cr.codigo_erp_cliente=c.codigo_erp
		           AND cr.data_liquidacao IS NULL AND cr.data_vencimento<CURRENT_DATE
		       ) AS possui_titulo_vencido
		FROM cliente c
		LEFT JOIN LATERAL (
		  SELECT p.codigo,p.data_emissao,p.situacao_app
		  FROM pedido p
		  WHERE p.cod_tenant=c.cod_tenant AND p.cod_empresa=c.cod_empresa
		    AND (p.cliente_id=c.codigo OR
		         (p.cliente_id IS NULL AND p.codigo_erp_cliente=c.codigo_erp))
		    AND NOT COALESCE(p.e_um_orcamento,false)
		    AND NOT COALESCE(p.apagar_dados_pedido,false) AND p.situacao_app<>'cancelado'
		  ORDER BY p.data_emissao DESC NULLS LAST,p.codigo DESC LIMIT 1
		) last_order ON true
		LEFT JOIN LATERAL (
		  SELECT a.data_agendada FROM agenda a
		  WHERE a.cod_tenant=c.cod_tenant AND a.cod_empresa=c.cod_empresa
		    AND a.codigo_erp_cliente=c.codigo_erp AND a.concluida=true
		    AND a.deletado_em IS NULL
		  ORDER BY a.data_agendada DESC,a.codigo DESC LIMIT 1
		) last_attendance ON true
		WHERE c.cod_tenant=$1 AND c.cod_empresa=$2 AND c.deletado_em IS NULL AND c.codigo > $3
		  AND ` + clientSearchPredicate + `
		  AND ($5 = '' OR
		       ($5 = 'ativo' AND COALESCE(c.ativo,true) AND NOT COALESCE(c.prospect,false)
		         AND NOT ((c.data_bloqueio IS NOT NULL AND c.data_bloqueio<=CURRENT_DATE)
		           OR NULLIF(BTRIM(c.motivo_bloqueio),'') IS NOT NULL)) OR
		       ($5 = 'inativo' AND NOT COALESCE(c.ativo,true)) OR
		       ($5 = 'prospect' AND COALESCE(c.prospect,false)) OR
		       ($5 = 'bloqueado' AND COALESCE(c.ativo,true)
		         AND ((c.data_bloqueio IS NOT NULL AND c.data_bloqueio<=CURRENT_DATE)
		           OR NULLIF(BTRIM(c.motivo_bloqueio),'') IS NOT NULL)))
		ORDER BY CASE
		  WHEN $4 = '' THEN 0
		  WHEN public.unaccent_immutable(c.nome_razao_social) ILIKE public.unaccent_immutable($4 || '%') THEN 0
		  WHEN public.unaccent_immutable(COALESCE(c.nome_fantasia, '')) ILIKE public.unaccent_immutable($4 || '%') THEN 1
		  WHEN public.unaccent_immutable(c.codigo_erp::text) ILIKE public.unaccent_immutable($4 || '%') THEN 2
		  ELSE 3 END,
		  c.codigo ASC LIMIT $6`
	rows, err := tx.Query(ctx, query, tenantID, empresaID, cursor, search, status, limit+1)
	if err != nil {
		return nil, fmt.Errorf("listar clientes: %w", err)
	}
	return db.ReadRows(rows)
}

const detailQuery = `
	SELECT c.codigo AS id, c.codigo_erp, c.situacao_app AS status_integracao,
	       c.motivo_rejeicao, c.nome_razao_social, c.nome_fantasia, c.cnpj_cpf,
	       c.inscricao_estadual_rg,
	       CASE c.tipo_pessoa WHEN 'F' THEN 'PF' WHEN 'J' THEN 'PJ' ELSE NULL END AS tipo_pessoa,
	       COALESCE(c.prospect, false) AS prospect, COALESCE(c.ativo, true) AS ativo,
	       COALESCE(c.telefone1, '') AS telefone1, COALESCE(c.telefone2, '') AS telefone2,
	       COALESCE(c.celular, '') AS celular,
	       COALESCE(c.nome_contato_comprador, '') AS nome_contato_comprador,
	       COALESCE(c.email_contato_comprador, '') AS email_contato_comprador,
	       COALESCE(c.nome_contato_faturamento, '') AS nome_contato_faturamento,
	       COALESCE(c.email_contato_faturamento, '') AS email_contato_faturamento,
	       COALESCE(c.limite_credito_total, 0) AS limite_credito_total,
	       c.anotacao_erp, COALESCE(c.anotacao_usuario,'') AS anotacao_usuario,
	       CASE WHEN c.data_ultima_visita IS NULL THEN NULL
	            ELSE to_char(c.data_ultima_visita,'YYYY-MM-DD') END AS data_ultima_visita,
	       CASE WHEN c.data_inclusao IS NULL THEN NULL ELSE
	         to_char(c.data_inclusao::timestamp AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') END AS data_inclusao,
	       CASE WHEN NOT COALESCE(c.ativo,true) THEN 'inativo'
	            WHEN (c.data_bloqueio IS NOT NULL AND c.data_bloqueio<=CURRENT_DATE)
	              OR NULLIF(BTRIM(c.motivo_bloqueio),'') IS NOT NULL THEN 'bloqueado'
	            ELSE 'aberto' END AS situacao_cadastro,
	       CASE WHEN NOT COALESCE(c.ativo,true)
	              OR (c.data_bloqueio IS NOT NULL AND c.data_bloqueio<=CURRENT_DATE)
	              OR NULLIF(BTRIM(c.motivo_bloqueio),'') IS NOT NULL THEN 'bloqueado'
	            WHEN last_order.codigo IS NULL THEN 'sem_compra'
	            WHEN last_order.situacao_app='em_digitacao' THEN 'em_digitacao'
	            ELSE 'aberto' END AS situacao_compra,
	       CASE WHEN last_order.data_emissao IS NULL THEN NULL
	            ELSE to_char(last_order.data_emissao,'YYYY-MM-DD') END AS ultimo_pedido,
	       CASE WHEN last_attendance.data_agendada IS NULL THEN NULL
	            ELSE to_char(last_attendance.data_agendada,'YYYY-MM-DD') END AS ultimo_atendimento,
	       NULLIF(BTRIM(CONCAT_WS(', ',
	         NULLIF(BTRIM(CONCAT_WS(' ',c.logradouro,c.numero_endereco)), ''),
	         NULLIF(c.bairro,''))), '') AS endereco_resumo,
	       NULLIF(BTRIM(CONCAT_WS(', ',
	         NULLIF(BTRIM(CONCAT_WS(' ',c.logradouro,c.numero_endereco)), ''),
	         NULLIF(c.complemento,''),NULLIF(c.bairro,''),NULLIF(c.cidade,''),
	         NULLIF(c.estado,''),NULLIF(c.cep,''))), '') AS endereco_completo,
	       EXISTS (
	         SELECT 1 FROM cliente_conta_receber cr
	         WHERE cr.cod_tenant=c.cod_tenant AND cr.cod_empresa=c.cod_empresa
	           AND cr.codigo_erp_cliente=c.codigo_erp
	           AND cr.data_liquidacao IS NULL AND cr.data_vencimento<CURRENT_DATE
	       ) AS possui_titulo_vencido,
	       tr.codigo AS transportadora_padrao_id,
	       cp.codigo AS condicao_pagamento_padrao_id,
	       fp.codigo AS forma_pagamento_padrao_id,
	       tp.codigo AS tabela_preco_padrao_id,
	       c.cep, c.logradouro, c.numero_endereco, c.complemento, c.bairro, c.cidade, c.estado
	FROM cliente c
	LEFT JOIN transportadora tr ON tr.cod_tenant=c.cod_tenant AND tr.cod_empresa=c.cod_empresa
	  AND tr.codigo_erp=c.codigo_erp_transportadora
	LEFT JOIN condicao_pagamento cp ON cp.cod_tenant=c.cod_tenant AND cp.cod_empresa=c.cod_empresa
	  AND cp.codigo_erp=c.codigo_erp_condicao_pagamento
	LEFT JOIN forma_pagamento fp ON fp.cod_tenant=c.cod_tenant AND fp.cod_empresa=c.cod_empresa
	  AND fp.codigo_erp=c.codigo_erp_forma_pagamento
	LEFT JOIN tabela_preco tp ON tp.cod_tenant=c.cod_tenant AND tp.cod_empresa=c.cod_empresa
	  AND tp.codigo_erp=c.codigo_erp_tabela_preco
	LEFT JOIN LATERAL (
	  SELECT p.codigo,p.data_emissao,p.situacao_app FROM pedido p
	  WHERE p.cod_tenant=c.cod_tenant AND p.cod_empresa=c.cod_empresa
	    AND (p.cliente_id=c.codigo OR
	         (p.cliente_id IS NULL AND p.codigo_erp_cliente=c.codigo_erp))
	    AND NOT COALESCE(p.e_um_orcamento,false)
	    AND NOT COALESCE(p.apagar_dados_pedido,false) AND p.situacao_app<>'cancelado'
	  ORDER BY p.data_emissao DESC NULLS LAST,p.codigo DESC LIMIT 1
	) last_order ON true
	LEFT JOIN LATERAL (
	  SELECT a.data_agendada FROM agenda a
	  WHERE a.cod_tenant=c.cod_tenant AND a.cod_empresa=c.cod_empresa
	    AND a.codigo_erp_cliente=c.codigo_erp AND a.concluida=true
	    AND a.deletado_em IS NULL
	  ORDER BY a.data_agendada DESC,a.codigo DESC LIMIT 1
	) last_attendance ON true
	WHERE c.cod_tenant=$1 AND c.cod_empresa=$2 AND c.codigo=$3 AND c.deletado_em IS NULL`

func (r *Repository) Get(ctx context.Context, tx pgx.Tx, tenantID, empresaID, id int) (map[string]any, error) {
	rows, err := tx.Query(ctx, detailQuery, tenantID, empresaID, id)
	if err != nil {
		return nil, fmt.Errorf("consultar cliente: %w", err)
	}
	items, err := db.ReadRows(rows)
	if err != nil {
		return nil, fmt.Errorf("ler cliente: %w", err)
	}
	if len(items) == 0 {
		return nil, pgx.ErrNoRows
	}
	return items[0], nil
}

func (r *Repository) ListAddresses(ctx context.Context, tx pgx.Tx, tenantID, empresaID, customerID int) ([]map[string]any, error) {
	rows, err := tx.Query(ctx, `
		SELECT codigo AS id, codigo_erp_endereco AS codigo_erp,
		       CASE tipo_endereco WHEN 'C' THEN 'cobranca'
		            WHEN 'E' THEN 'entrega' WHEN 'A' THEN 'ambos' ELSE NULL END AS tipo_endereco,
		       endereco_padrao, cep, logradouro, numero_endereco,
		       complemento_endereco, bairro, cidade, estado, ponto_referencia, restricoes_horarios
		FROM cliente_endereco_adicional
		WHERE cod_tenant=$1 AND cod_empresa=$2 AND deletado_em IS NULL
		  AND (cliente_id=$3 OR (cliente_id IS NULL AND codigo_erp_cliente=(
		    SELECT codigo_erp FROM cliente
		    WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$3 AND deletado_em IS NULL)))
		ORDER BY endereco_padrao DESC, codigo ASC`, tenantID, empresaID, customerID)
	if err != nil {
		return nil, fmt.Errorf("listar endereços do cliente: %w", err)
	}
	return db.ReadRows(rows)
}

func (r *Repository) ListReceivables(ctx context.Context, tx pgx.Tx, tenantID, empresaID, customerID int) ([]map[string]any, error) {
	rows, err := tx.Query(ctx, `
		SELECT codigo AS id, codigo_erp, titulo_numero_documento,
		       titulo_especie_documento, titulo_serie, titulo_numero_parcela,
		       CASE WHEN data_emissao IS NULL THEN NULL ELSE
		         to_char(data_emissao::timestamp AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') END AS data_emissao,
		       CASE WHEN data_vencimento IS NULL THEN NULL ELSE
		         to_char(data_vencimento::timestamp AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') END AS data_vencimento,
		       COALESCE(valor_total_aberto, 0) AS valor_total_aberto,
		       CASE WHEN data_liquidacao IS NOT NULL THEN 'pago'
		            WHEN data_vencimento < CURRENT_DATE THEN 'vencido' ELSE 'aberto' END AS situacao
		FROM cliente_conta_receber
		WHERE cod_tenant=$1 AND cod_empresa=$2
		  AND codigo_erp_cliente = (SELECT codigo_erp FROM cliente WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$3)
		ORDER BY (data_liquidacao IS NULL AND data_vencimento < CURRENT_DATE) DESC,
		         data_vencimento ASC NULLS LAST, codigo ASC`, tenantID, empresaID, customerID)
	if err != nil {
		return nil, fmt.Errorf("listar contas a receber: %w", err)
	}
	return db.ReadRows(rows)
}

func (r *Repository) IsDuplicate(ctx context.Context, tx pgx.Tx, tenantID, empresaID int, document string, excludeID int) (bool, error) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, document)
	if digits == "" {
		return false, nil
	}
	lockKey := fmt.Sprintf("%d:%d:%s", tenantID, empresaID, digits)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); err != nil {
		return false, fmt.Errorf("sincronizar validação de documento duplicado: %w", err)
	}
	var duplicate bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM cliente
		 WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo<>$3
		   AND regexp_replace(COALESCE(cnpj_cpf,''),'[^0-9]','','g')=$4)`,
		tenantID, empresaID, excludeID, digits).Scan(&duplicate)
	if err != nil {
		return false, fmt.Errorf("validar documento duplicado de cliente: %w", err)
	}
	return duplicate, nil
}

type clienteDefaultCodes struct {
	transportadora    *string
	condicaoPagamento *string
	formaPagamento    *string
	tabelaPreco       *string
}

func (r *Repository) resolveDefaultCodes(ctx context.Context, tx pgx.Tx, tenantID, empresaID int, input models.ClienteCreate) (clienteDefaultCodes, error) {
	var codes clienteDefaultCodes
	references := []struct {
		id    *int
		table string
		label string
		code  **string
	}{
		{input.TransportadoraPadraoID, "transportadora", "transportadora_padrao_id", &codes.transportadora},
		{input.CondicaoPagamentoPadraoID, "condicao_pagamento", "condicao_pagamento_padrao_id", &codes.condicaoPagamento},
		{input.FormaPagamentoPadraoID, "forma_pagamento", "forma_pagamento_padrao_id", &codes.formaPagamento},
		{input.TabelaPrecoPadraoID, "tabela_preco", "tabela_preco_padrao_id", &codes.tabelaPreco},
	}
	for _, reference := range references {
		if reference.id == nil {
			continue
		}
		if *reference.id <= 0 {
			return clienteDefaultCodes{}, fmt.Errorf("%s inválido", reference.label)
		}
		var code string
		query := fmt.Sprintf(`SELECT codigo_erp FROM %s
			WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$3 AND COALESCE(ativo,true)`, reference.table)
		if err := tx.QueryRow(ctx, query, tenantID, empresaID, *reference.id).Scan(&code); err != nil {
			if err == pgx.ErrNoRows {
				return clienteDefaultCodes{}, fmt.Errorf("%s inválido: %w", reference.label, err)
			}
			return clienteDefaultCodes{}, fmt.Errorf("validar %s: %w", reference.label, err)
		}
		*reference.code = &code
	}
	return codes, nil
}

func (r *Repository) Create(ctx context.Context, tx pgx.Tx, tenantID, empresaID int, input models.ClienteCreate, codes clienteDefaultCodes) (map[string]any, error) {
	var id int
	if err := tx.QueryRow(ctx, "SELECT nextval('public.cliente_codigo_seq')").Scan(&id); err != nil {
		return nil, fmt.Errorf("gerar identificador de cliente: %w", err)
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO cliente (
			cod_tenant,cod_empresa,codigo,codigo_erp,nome_razao_social,nome_fantasia,cnpj_cpf,
			inscricao_estadual_rg,tipo_pessoa,prospect,telefone1,telefone2,celular,
			nome_contato_comprador,email_contato_comprador,nome_contato_faturamento,
			email_contato_faturamento,limite_credito_total,anotacao_erp,cep,logradouro,
			numero_endereco,complemento,bairro,cidade,estado,codigo_erp_transportadora,
			codigo_erp_condicao_pagamento,codigo_erp_forma_pagamento,codigo_erp_tabela_preco,
			data_ultima_visita,anotacao_usuario
		) VALUES (
			$1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),$9,$10,
			NULLIF($11,''),NULLIF($12,''),NULLIF($13,''),NULLIF($14,''),NULLIF($15,''),
			NULLIF($16,''),NULLIF($17,''),$18,NULLIF($19,''),NULLIF($20,''),NULLIF($21,''),
			NULLIF($22,''),NULLIF($23,''),NULLIF($24,''),NULLIF($25,''),NULLIF($26,''),
			$27,$28,$29,$30,NULLIF($31,'')::date,$32
		)`, tenantID, empresaID, id, nil, input.NomeRazaoSocial,
		input.NomeFantasia, input.CNPJCPF, input.InscricaoEstadualRG, bancoTipoPessoa(input.TipoPessoa),
		input.Prospect, input.Telefone1, input.Telefone2, input.Celular, input.NomeContatoComprador,
		input.EmailContatoComprador, input.NomeContatoFaturamento, input.EmailContatoFaturamento,
		input.LimiteCreditoTotal, input.AnotacaoERP, input.CEP, input.Logradouro, input.NumeroEndereco,
		input.Complemento, input.Bairro, input.Cidade, input.Estado,
		codes.transportadora, codes.condicaoPagamento, codes.formaPagamento, codes.tabelaPreco,
		input.DataUltimaVisita, input.AnotacaoUsuario)
	if err != nil {
		return nil, fmt.Errorf("criar cliente: %w", err)
	}
	return r.Get(ctx, tx, tenantID, empresaID, id)
}

func (r *Repository) Update(ctx context.Context, tx pgx.Tx, tenantID, empresaID, id int, input models.ClienteCreate, codes clienteDefaultCodes) (map[string]any, error) {
	tag, err := tx.Exec(ctx, `
		UPDATE cliente SET nome_razao_social=$3,nome_fantasia=NULLIF($4,''),cnpj_cpf=NULLIF($5,''),
			inscricao_estadual_rg=NULLIF($6,''),tipo_pessoa=$7,prospect=$8,
			telefone1=NULLIF($9,''),telefone2=NULLIF($10,''),celular=NULLIF($11,''),
			nome_contato_comprador=NULLIF($12,''),email_contato_comprador=NULLIF($13,''),
			nome_contato_faturamento=NULLIF($14,''),email_contato_faturamento=NULLIF($15,''),
			limite_credito_total=$16,anotacao_erp=NULLIF($17,''),cep=NULLIF($18,''),
			logradouro=NULLIF($19,''),numero_endereco=NULLIF($20,''),complemento=NULLIF($21,''),
			bairro=NULLIF($22,''),cidade=NULLIF($23,''),estado=NULLIF($24,''),
			codigo_erp_transportadora=$25,codigo_erp_condicao_pagamento=$26,
			codigo_erp_forma_pagamento=$27,codigo_erp_tabela_preco=$28,
			data_ultima_visita=NULLIF($29,'')::date,anotacao_usuario=$30
		WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$31 AND deletado_em IS NULL
		  AND situacao_app NOT IN ('em_integracao','integrado')`,
		tenantID, empresaID, input.NomeRazaoSocial, input.NomeFantasia, input.CNPJCPF,
		input.InscricaoEstadualRG, bancoTipoPessoa(input.TipoPessoa), input.Prospect,
		input.Telefone1, input.Telefone2, input.Celular, input.NomeContatoComprador,
		input.EmailContatoComprador, input.NomeContatoFaturamento, input.EmailContatoFaturamento,
		input.LimiteCreditoTotal, input.AnotacaoERP, input.CEP, input.Logradouro, input.NumeroEndereco,
		input.Complemento, input.Bairro, input.Cidade, input.Estado,
		codes.transportadora, codes.condicaoPagamento, codes.formaPagamento, codes.tabelaPreco,
		input.DataUltimaVisita, input.AnotacaoUsuario, id)
	if err != nil {
		return nil, fmt.Errorf("atualizar cliente: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var status string
		statusErr := tx.QueryRow(ctx, `SELECT situacao_app FROM cliente
			WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$3 AND deletado_em IS NULL`,
			tenantID, empresaID, id).Scan(&status)
		if statusErr == nil && (status == "em_integracao" || status == "integrado") {
			return nil, apperrors.ErrIntegrationLocked
		}
		if statusErr != nil && statusErr != pgx.ErrNoRows {
			return nil, fmt.Errorf("consultar situação do cliente: %w", statusErr)
		}
		return nil, pgx.ErrNoRows
	}
	return r.Get(ctx, tx, tenantID, empresaID, id)
}

func (r *Repository) Deactivate(ctx context.Context, tx pgx.Tx, tenantID, empresaID, id int) error {
	tag, err := tx.Exec(ctx, `UPDATE cliente SET deletado_em=now()
		WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$3 AND deletado_em IS NULL
		  AND situacao_app NOT IN ('em_integracao','integrado')`, tenantID, empresaID, id)
	if err != nil {
		return fmt.Errorf("remover cliente: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var status string
		statusErr := tx.QueryRow(ctx, `SELECT situacao_app FROM cliente
			WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$3 AND deletado_em IS NULL`,
			tenantID, empresaID, id).Scan(&status)
		if statusErr == nil && (status == "em_integracao" || status == "integrado") {
			return apperrors.ErrIntegrationLocked
		}
		if statusErr != nil && statusErr != pgx.ErrNoRows {
			return fmt.Errorf("consultar situação do cliente: %w", statusErr)
		}
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) CreateAddress(ctx context.Context, tx pgx.Tx, tenantID, empresaID, customerID int, input models.EnderecoCreate, dbType string) (map[string]any, error) {
	if err := r.lockCustomer(ctx, tx, tenantID, empresaID, customerID); err != nil {
		return nil, err
	}
	if input.EnderecoPadrao {
		if _, err := tx.Exec(ctx, `
			UPDATE cliente_endereco_adicional SET endereco_padrao=false
			WHERE cod_tenant=$1 AND cod_empresa=$2 AND cliente_id=$3
			  AND deletado_em IS NULL`, tenantID, empresaID, customerID); err != nil {
			return nil, fmt.Errorf("limpar endereço padrão: %w", err)
		}
	}
	var id int
	if err := tx.QueryRow(ctx, "SELECT nextval('public.cliente_endereco_adicional_codigo_seq')").Scan(&id); err != nil {
		return nil, fmt.Errorf("gerar identificador de endereço: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO cliente_endereco_adicional (
			cod_tenant,cod_empresa,codigo,cliente_id,codigo_erp_cliente,codigo_erp_endereco,identificacao_endereco,
			cep,logradouro,numero_endereco,complemento_endereco,bairro,cidade,estado,tipo_endereco,
			endereco_padrao,ponto_referencia,restricoes_horarios
		) VALUES ($1,$2,$3,$4,(SELECT codigo_erp FROM cliente
			WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$4),NULL,NULL,
			$5,$6,$7,NULLIF($8,''),$9,$10,$11,$12,$13,NULLIF($14,''),NULLIF($15,''))`,
		tenantID, empresaID, id, customerID, input.CEP, input.Logradouro,
		input.NumeroEndereco, input.ComplementoEndereco, input.Bairro,
		input.Cidade, input.Estado, dbType, input.EnderecoPadrao, input.PontoReferencia, input.RestricoesHorarios); err != nil {
		return nil, fmt.Errorf("criar endereço: %w", err)
	}
	return r.addressByID(ctx, tx, tenantID, empresaID, customerID, id)
}

func (r *Repository) UpdateAddress(ctx context.Context, tx pgx.Tx, tenantID, empresaID, customerID, addressID int, input models.EnderecoCreate, dbType string) (map[string]any, error) {
	if err := r.lockCustomer(ctx, tx, tenantID, empresaID, customerID); err != nil {
		return nil, err
	}

	tag, err := tx.Exec(ctx, `
		UPDATE cliente_endereco_adicional
		SET cep=$1, logradouro=$2, numero_endereco=$3, complemento_endereco=NULLIF($4,''),
		    bairro=$5, cidade=$6, estado=$7, tipo_endereco=$8, endereco_padrao=$9,
		    ponto_referencia=NULLIF($10,''), restricoes_horarios=NULLIF($11,''), dt_alteracao=now()
		WHERE cod_tenant=$12 AND cod_empresa=$13 AND codigo=$14
		  AND (cliente_id=$15 OR (cliente_id IS NULL AND codigo_erp_cliente=(
		    SELECT codigo_erp FROM cliente
		    WHERE cod_tenant=$12 AND cod_empresa=$13 AND codigo=$15)))
		  AND deletado_em IS NULL`,
		input.CEP, input.Logradouro, input.NumeroEndereco, input.ComplementoEndereco,
		input.Bairro, input.Cidade, input.Estado, dbType, input.EnderecoPadrao,
		input.PontoReferencia, input.RestricoesHorarios, tenantID, empresaID, addressID, customerID)
	if err != nil {
		return nil, fmt.Errorf("atualizar endereço: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}

	if input.EnderecoPadrao {
		if _, err := tx.Exec(ctx, `
			UPDATE cliente_endereco_adicional SET endereco_padrao=false
			WHERE cod_tenant=$1 AND cod_empresa=$2 AND cliente_id=$3
			  AND codigo<>$4 AND deletado_em IS NULL`, tenantID, empresaID, customerID, addressID); err != nil {
			return nil, fmt.Errorf("limpar endereços padrão: %w", err)
		}
	}

	return r.addressByID(ctx, tx, tenantID, empresaID, customerID, addressID)
}

func (r *Repository) lockCustomer(ctx context.Context, tx pgx.Tx, tenantID, empresaID, customerID int) error {
	var status string
	err := tx.QueryRow(ctx, `
		SELECT situacao_app FROM cliente
		WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$3 AND deletado_em IS NULL
		FOR UPDATE`, tenantID, empresaID, customerID).Scan(&status)
	if err != nil {
		return fmt.Errorf("consultar cliente para endereço: %w", err)
	}
	if status == "em_integracao" || status == "integrado" {
		return apperrors.ErrIntegrationLocked
	}
	return nil
}

func (r *Repository) addressByID(ctx context.Context, tx pgx.Tx, tenantID, empresaID, customerID, addressID int) (map[string]any, error) {
	rows, err := tx.Query(ctx, `
		SELECT e.codigo AS id, e.codigo_erp_endereco AS codigo_erp,
		       CASE e.tipo_endereco WHEN 'C' THEN 'cobranca'
		            WHEN 'E' THEN 'entrega' WHEN 'A' THEN 'ambos' ELSE NULL END AS tipo_endereco,
		       e.endereco_padrao, e.cep, e.logradouro, e.numero_endereco,
		       e.complemento_endereco, e.bairro, e.cidade, e.estado,
		       e.ponto_referencia, e.restricoes_horarios
		FROM cliente_endereco_adicional e
		WHERE e.cod_tenant=$1 AND e.cod_empresa=$2 AND e.codigo=$3
		  AND e.deletado_em IS NULL
		  AND (e.cliente_id=$4 OR (e.cliente_id IS NULL AND e.codigo_erp_cliente=(
		    SELECT c.codigo_erp FROM cliente c
		    WHERE c.cod_tenant=$1 AND c.cod_empresa=$2 AND c.codigo=$4 AND c.deletado_em IS NULL)))`,
		tenantID, empresaID, addressID, customerID)
	if err != nil {
		return nil, fmt.Errorf("consultar endereço atualizado: %w", err)
	}
	items, err := db.ReadRows(rows)
	if err != nil {
		return nil, fmt.Errorf("ler endereço atualizado: %w", err)
	}
	if len(items) == 0 {
		return nil, pgx.ErrNoRows
	}
	return items[0], nil
}

func (r *Repository) RemoveAddress(ctx context.Context, tx pgx.Tx, tenantID, empresaID, customerID, addressID int) error {
	if err := r.lockCustomer(ctx, tx, tenantID, empresaID, customerID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE cliente_endereco_adicional SET deletado_em=now()
		WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$3 AND deletado_em IS NULL
		  AND (cliente_id=$4 OR (cliente_id IS NULL AND codigo_erp_cliente=(
		    SELECT codigo_erp FROM cliente
		    WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$4 AND deletado_em IS NULL)))`,
		tenantID, empresaID, addressID, customerID)
	if err != nil {
		return fmt.Errorf("remover endereço: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
