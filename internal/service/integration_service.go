package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type IntegrationResult struct {
	IntegrationID   int64   `json:"integracao_id"`
	Success         bool    `json:"sucesso"`
	ERPCode         *string `json:"codigo_erp"`
	RejectionReason *string `json:"motivo_rejeicao"`
}

type pendingIntegration struct {
	id       int64
	entityID int
	entity   string
	attempt  int
	sentAt   time.Time
}

func validIntegrationEntity(entity string) bool {
	return entity == "cliente" || entity == "pedido" || entity == "orcamento"
}

func (s *Service) FinalizarIntegracao(ctx context.Context, tenantID, empresaID, entityID int, entity string) (map[string]any, error) {
	if !validIntegrationEntity(entity) {
		return nil, InvalidInput("entidade deve ser cliente, pedido ou orcamento")
	}

	var result map[string]any
	err := s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		var status string
		switch entity {
		case "cliente":
			err := tx.QueryRow(ctx, `
				SELECT situacao_app FROM cliente
				WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$3
				  AND deletado_em IS NULL
				FOR UPDATE`, tenantID, empresaID, entityID).Scan(&status)
			if err != nil {
				return err
			}
		case "pedido", "orcamento":
			var isBudget bool
			err := tx.QueryRow(ctx, `
				SELECT situacao_app,COALESCE(e_um_orcamento,false) FROM pedido
				WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$3
				  AND COALESCE(apagar_dados_pedido,false)=false
				FOR UPDATE`, tenantID, empresaID, entityID).Scan(&status, &isBudget)
			if err != nil {
				return err
			}
			if isBudget != (entity == "orcamento") {
				return pgx.ErrNoRows
			}
		}
		if status != "em_digitacao" && status != "erro_integracao" {
			return ErrIntegrationLocked
		}

		var attempt int
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(MAX(tentativa),0)+1
			FROM integracao_fila
			WHERE cod_tenant=$1 AND cod_empresa=$2 AND entidade=$3 AND entidade_id=$4`,
			tenantID, empresaID, entity, entityID).Scan(&attempt); err != nil {
			return fmt.Errorf("calcular tentativa de integração: %w", err)
		}
		var integrationID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO integracao_fila
				(cod_tenant,cod_empresa,entidade,entidade_id,status,tentativa)
			VALUES ($1,$2,$3,$4,'pendente',$5)
			RETURNING id`, tenantID, empresaID, entity, entityID, attempt).Scan(&integrationID); err != nil {
			return fmt.Errorf("enfileirar cadastro para integração: %w", err)
		}

		table := "pedido"
		if entity == "cliente" {
			table = "cliente"
		}
		query := fmt.Sprintf(`
			UPDATE %s SET situacao_app='em_integracao',motivo_rejeicao=NULL
			WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$3`, table)
		if _, err := tx.Exec(ctx, query, tenantID, empresaID, entityID); err != nil {
			return fmt.Errorf("atualizar situação da integração: %w", err)
		}
		result = map[string]any{
			"entidade": entity, "entidade_id": entityID,
			"integracao_id": integrationID, "status": "em_integracao",
		}
		return nil
	})
	return result, err
}

func (s *Service) IntegracaoPendentes(ctx context.Context, tenantID, empresaID int, entity string, limit int) ([]map[string]any, error) {
	if !validIntegrationEntity(entity) {
		return nil, InvalidInput("entidade deve ser cliente, pedido ou orcamento")
	}
	if limit < 1 || limit > 100 {
		return nil, InvalidInput("limit deve estar entre 1 e 100")
	}

	var pending []pendingIntegration
	err := s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id,entidade_id,entidade,tentativa,data_envio
			FROM integracao_fila
			WHERE cod_tenant=$1 AND cod_empresa=$2 AND entidade=$3 AND status='pendente'
			ORDER BY data_envio,id
			LIMIT $4`, tenantID, empresaID, entity, limit)
		if err != nil {
			return fmt.Errorf("consultar fila de integração: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item pendingIntegration
			if err := rows.Scan(&item.id, &item.entityID, &item.entity, &item.attempt, &item.sentAt); err != nil {
				return fmt.Errorf("ler fila de integração: %w", err)
			}
			pending = append(pending, item)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(pending))
	for _, queued := range pending {
		var data map[string]any
		switch entity {
		case "cliente":
			data, err = s.ClienteAPI(ctx, tenantID, empresaID, queued.entityID)
		case "pedido", "orcamento":
			data, err = s.PedidoObterAPI(ctx, tenantID, empresaID, queued.entityID)
			if err == nil && (data["tipo"] != entity) {
				return nil, fmt.Errorf("a entidade da fila não corresponde ao pedido %d", queued.entityID)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("carregar dados da fila de integração %d: %w", queued.id, err)
		}
		items = append(items, map[string]any{
			"integracao_id": queued.id,
			"entidade":      queued.entity,
			"entidade_id":   queued.entityID,
			"tentativa":     queued.attempt,
			"data_envio":    queued.sentAt,
			"dados":         data,
		})
	}
	return items, nil
}

func (s *Service) ProcessarRetornoIntegracao(ctx context.Context, tenantID, empresaID, entityID int, entity string, result IntegrationResult) (map[string]any, error) {
	if !validIntegrationEntity(entity) {
		return nil, InvalidInput("entidade deve ser cliente, pedido ou orcamento")
	}
	if result.IntegrationID <= 0 {
		return nil, InvalidInput("integracao_id é obrigatório")
	}
	if result.Success {
		if result.ERPCode == nil || strings.TrimSpace(*result.ERPCode) == "" {
			return nil, InvalidInput("codigo_erp é obrigatório em retornos com sucesso")
		}
		normalizedCode := strings.TrimSpace(*result.ERPCode)
		if len([]rune(normalizedCode)) > 20 {
			return nil, InvalidInput("codigo_erp deve ter no máximo 20 caracteres")
		}
		result.ERPCode = &normalizedCode
		result.RejectionReason = nil
	} else {
		if result.RejectionReason == nil || strings.TrimSpace(*result.RejectionReason) == "" {
			return nil, InvalidInput("motivo_rejeicao é obrigatório em retornos com erro")
		}
		normalizedReason := strings.TrimSpace(*result.RejectionReason)
		result.RejectionReason = &normalizedReason
		result.ERPCode = nil
	}

	var state string
	err := s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		var queueStatus string
		var queuedCode, queuedReason *string
		if err := tx.QueryRow(ctx, `
			SELECT status,codigo_erp_retorno,motivo_rejeicao
			FROM integracao_fila
			WHERE id=$1 AND cod_tenant=$2 AND cod_empresa=$3
			  AND entidade=$4 AND entidade_id=$5
			FOR UPDATE`, result.IntegrationID, tenantID, empresaID, entity, entityID).
			Scan(&queueStatus, &queuedCode, &queuedReason); err != nil {
			return err
		}

		queueState := "erro"
		if result.Success {
			queueState = "sucesso"
		}
		if queueStatus != "pendente" {
			if queueStatus == queueState &&
				equalOptionalString(queuedCode, result.ERPCode) &&
				equalOptionalString(queuedReason, result.RejectionReason) {
				state = queueState
				return nil
			}
			return ErrIntegrationLocked
		}

		table := "pedido"
		if entity == "cliente" {
			table = "cliente"
		}
		entityState := "erro_integracao"
		if result.Success {
			entityState = "integrado"
		}
		updateEntity := fmt.Sprintf(`
			UPDATE %s SET situacao_app=$4,motivo_rejeicao=$5,codigo_erp=$6
			WHERE cod_tenant=$1 AND cod_empresa=$2 AND codigo=$3
			  AND situacao_app='em_integracao'`, table)
		var code any
		if result.Success {
			code = strings.TrimSpace(*result.ERPCode)
		}
		var reason any
		if !result.Success {
			reason = strings.TrimSpace(*result.RejectionReason)
		}
		tag, err := tx.Exec(ctx, updateEntity, tenantID, empresaID, entityID, entityState, reason, code)
		if err != nil {
			return fmt.Errorf("atualizar cadastro com retorno ERP: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return ErrIntegrationLocked
		}
		if _, err := tx.Exec(ctx, `
			UPDATE integracao_fila
			SET status=$2,codigo_erp_retorno=$3,motivo_rejeicao=$4,data_retorno=NOW()
			WHERE id=$1`, result.IntegrationID, queueState, code, reason); err != nil {
			return fmt.Errorf("registrar retorno ERP: %w", err)
		}
		state = queueState
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"entidade": entity, "entidade_id": entityID, "integracao_id": result.IntegrationID,
		"status": state,
	}, nil
}

func equalOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
