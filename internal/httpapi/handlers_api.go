package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/celsolenonsander/visionflow-api/internal/models"
	"github.com/celsolenonsander/visionflow-api/internal/service"
)

func cursorValue(cursor string) any {
	if cursor == "" {
		return nil
	}
	return cursor
}

func (h *Handler) CriarCliente(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	var input models.ClienteCreate
	if !decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.NomeRazaoSocial) == "" ||
		(input.TipoPessoa != "PF" && input.TipoPessoa != "PJ") {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "nome_razao_social e tipo_pessoa (PF ou PJ) são obrigatórios.")
		return
	}
	result, err := h.Service.CriarCliente(r.Context(), tenantID, empresaID, input)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.Is(err, service.ErrDuplicateResource) ||
			(errors.As(err, &pgErr) && pgErr.Code == "23505") {
			bad(w, http.StatusConflict, "CLIENTE_JA_EXISTE", "Já existe um cliente com este CNPJ/CPF.")
			return
		}
		writeServiceError(w, err, "Não foi possível criar o cliente.")
		return
	}
	writeJSON(w, http.StatusCreated, result, nil)
}

func (h *Handler) AtualizarCliente(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do cliente inválido.")
		return
	}
	var input models.ClienteCreate
	if !decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.NomeRazaoSocial) == "" ||
		(input.TipoPessoa != "PF" && input.TipoPessoa != "PJ") {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "nome_razao_social e tipo_pessoa (PF ou PJ) são obrigatórios.")
		return
	}
	result, err := h.Service.AtualizarCliente(r.Context(), tenantID, empresaID, id, input)
	if err != nil {
		writeServiceError(w, err, "Não foi possível atualizar o cliente.")
		return
	}
	writeJSON(w, http.StatusOK, result, nil)
}

func (h *Handler) RemoverCliente(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do cliente inválido.")
		return
	}
	if err := h.Service.RemoverCliente(r.Context(), tenantID, empresaID, id); err != nil {
		writeServiceError(w, err, "Não foi possível remover o cliente.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true}, nil)
}

func (h *Handler) EnderecosCliente(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do cliente inválido.")
		return
	}
	items, err := h.Service.EnderecosCliente(r.Context(), tenantID, empresaID, id)
	if err != nil {
		writeServiceError(w, err, "Não foi possível listar os endereços.")
		return
	}
	writeJSON(w, http.StatusOK, items, nil)
}

func (h *Handler) CriarEndereco(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	clienteID, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do cliente inválido.")
		return
	}
	var input models.EnderecoCreate
	if !decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.TipoEndereco) == "" || strings.TrimSpace(input.CEP) == "" ||
		strings.TrimSpace(input.Logradouro) == "" || strings.TrimSpace(input.NumeroEndereco) == "" ||
		strings.TrimSpace(input.Bairro) == "" || strings.TrimSpace(input.Cidade) == "" ||
		len(input.Estado) != 2 {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "Os campos obrigatórios do endereço devem ser preenchidos.")
		return
	}
	result, err := h.Service.CriarEndereco(r.Context(), tenantID, empresaID, clienteID, input)
	if err != nil {
		writeServiceError(w, err, "Não foi possível criar o endereço.")
		return
	}
	writeJSON(w, http.StatusCreated, result, nil)
}

func (h *Handler) RemoverEndereco(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	clienteID, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do cliente inválido.")
		return
	}
	enderecoID, err := requestID(chi.URLParam(r, "enderecoId"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do endereço inválido.")
		return
	}
	if err := h.Service.RemoverEndereco(r.Context(), tenantID, empresaID, clienteID, enderecoID); err != nil {
		writeServiceError(w, err, "Não foi possível remover o endereço.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true}, nil)
}

func (h *Handler) AtualizarEndereco(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	clienteID, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do cliente inválido.")
		return
	}
	enderecoID, err := requestID(chi.URLParam(r, "enderecoId"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do endereço inválido.")
		return
	}
	var input models.EnderecoCreate
	if !decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.TipoEndereco) == "" || strings.TrimSpace(input.CEP) == "" ||
		strings.TrimSpace(input.Logradouro) == "" || strings.TrimSpace(input.NumeroEndereco) == "" ||
		strings.TrimSpace(input.Bairro) == "" || strings.TrimSpace(input.Cidade) == "" ||
		len(input.Estado) != 2 {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "Os campos obrigatórios do endereço devem ser preenchidos.")
		return
	}
	result, err := h.Service.AtualizarEndereco(r.Context(), tenantID, empresaID, clienteID, enderecoID, input)
	if err != nil {
		writeServiceError(w, err, "Não foi possível atualizar o endereço.")
		return
	}
	writeJSON(w, http.StatusOK, result, nil)
}

func (h *Handler) ContasReceberCliente(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do cliente inválido.")
		return
	}
	items, err := h.Service.ContasReceberCliente(r.Context(), tenantID, empresaID, id)
	if err != nil {
		writeServiceError(w, err, "Não foi possível consultar as contas a receber.")
		return
	}
	writeJSON(w, http.StatusOK, items, nil)
}

func (h *Handler) ConsultarCEP(w http.ResponseWriter, r *http.Request) {
	result, err := h.Service.ConsultarCEP(r.Context(), chi.URLParam(r, "cep"))
	if err != nil {
		writeServiceError(w, err, "Não foi possível consultar o CEP.")
		return
	}
	writeJSON(w, http.StatusOK, result, nil)
}

func (h *Handler) Catalogo(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	limit, cursor, err := validPage(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", err.Error())
		return
	}
	resource := strings.TrimPrefix(r.URL.Path, "/")
	var clienteID, tabelaPrecoID *int
	if resource == "produtos" {
		for _, filter := range []struct {
			name string
			dest **int
		}{{"cliente_id", &clienteID}, {"tabela_preco_id", &tabelaPrecoID}} {
			if value := r.URL.Query().Get(filter.name); value != "" {
				id, err := requestID(value)
				if err != nil {
					bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", filter.name+" deve ser um ID positivo.")
					return
				}
				*filter.dest = &id
			}
		}
	}
	items, next, err := h.Service.CatalogoComPreco(r.Context(), tenantID, empresaID,
		userID, resource, r.URL.Query().Get("busca"), limit, cursor, clienteID, tabelaPrecoID)
	if err != nil {
		writeServiceError(w, err, "Não foi possível consultar o catálogo.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "nextCursor": cursorValue(next)}, nil)
}

func (h *Handler) HomeIndicadores(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	result, err := h.Service.IndicadoresAPI(r.Context(), tenantID, empresaID, userID)
	if err != nil {
		writeServiceError(w, err, "Não foi possível consultar os indicadores.")
		return
	}
	writeJSON(w, http.StatusOK, result, nil)
}

func (h *Handler) PedidosAPI(w http.ResponseWriter, r *http.Request) {
	h.listarPedidos(w, r, false)
}

func (h *Handler) OrcamentosAPI(w http.ResponseWriter, r *http.Request) {
	h.listarPedidos(w, r, true)
}

func (h *Handler) listarPedidos(w http.ResponseWriter, r *http.Request, orcamento bool) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	limit, cursor, err := validPage(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", err.Error())
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" && !validPedidoStatus(status) {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "status de pedido inválido.")
		return
	}
	var clienteID *int
	if value := r.URL.Query().Get("cliente_id"); value != "" {
		parsed, err := requestID(value)
		if err != nil {
			bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "cliente_id inválido.")
			return
		}
		clienteID = &parsed
	}
	includeCanceled, err := boolQuery(r, "incluir_cancelados")
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", err.Error())
		return
	}
	items, next, err := h.Service.PedidosListarAPI(
		r.Context(), tenantID, empresaID, status, clienteID, limit, cursor, orcamento,
		includeCanceled != nil && *includeCanceled,
	)
	if err != nil {
		writeServiceError(w, err, "Não foi possível listar os pedidos.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "nextCursor": cursorValue(next)}, nil)
}

func (h *Handler) PedidoAPI(w http.ResponseWriter, r *http.Request) {
	h.obterPedido(w, r, false)
}

func (h *Handler) OrcamentoAPI(w http.ResponseWriter, r *http.Request) {
	h.obterPedido(w, r, true)
}

func (h *Handler) obterPedido(w http.ResponseWriter, r *http.Request, orcamento bool) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do pedido inválido.")
		return
	}
	result, err := h.Service.PedidoObterAPI(r.Context(), tenantID, empresaID, id)
	if err != nil {
		writeServiceError(w, err, "Não foi possível consultar o pedido.")
		return
	}
	if (result["tipo"] == "orcamento") != orcamento {
		bad(w, http.StatusNotFound, "RECURSO_NAO_ENCONTRADO", "Recurso não encontrado.")
		return
	}
	writeJSON(w, http.StatusOK, result, nil)
}

func (h *Handler) SalvarPedidoAPI(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	var input models.PedidoCreate
	if !decode(w, r, &input) {
		return
	}
	if len(input.Itens) == 0 {
		bad(w, http.StatusBadRequest, "ITENS_OBRIGATORIOS", "O pedido deve conter ao menos um item.")
		return
	}
	if input.ClienteID == nil || *input.ClienteID <= 0 {
		bad(w, http.StatusBadRequest, "CLIENTE_INVALIDO", "cliente_id é obrigatório e deve ser válido.")
		return
	}
	result, err := h.Service.CriarPedido(r.Context(), tenantID, empresaID, input)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCustomer) {
			bad(w, http.StatusBadRequest, "CLIENTE_INVALIDO", "O cliente informado não existe ou foi removido.")
			return
		}
		writeServiceError(w, err, "Não foi possível salvar o pedido.")
		return
	}
	writeJSON(w, http.StatusCreated, result, nil)
}

func (h *Handler) AtualizarPedidoAPI(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do pedido inválido.")
		return
	}
	var input models.PedidoCreate
	if !decode(w, r, &input) {
		return
	}
	if len(input.Itens) == 0 {
		bad(w, http.StatusBadRequest, "ITENS_OBRIGATORIOS", "O pedido deve conter ao menos um item.")
		return
	}
	if input.ClienteID == nil || *input.ClienteID <= 0 {
		bad(w, http.StatusBadRequest, "CLIENTE_INVALIDO", "cliente_id é obrigatório e deve ser válido.")
		return
	}
	result, err := h.Service.AtualizarPedido(r.Context(), tenantID, empresaID, id, input)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCustomer) {
			bad(w, http.StatusBadRequest, "CLIENTE_INVALIDO", "O cliente informado não existe ou foi removido.")
			return
		}
		writeServiceError(w, err, "Não foi possível atualizar o pedido.")
		return
	}
	writeJSON(w, http.StatusOK, result, nil)
}

func (h *Handler) RemoverPedido(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do pedido inválido.")
		return
	}
	if err := h.Service.RemoverPedido(r.Context(), tenantID, empresaID, id); err != nil {
		writeServiceError(w, err, "Não foi possível remover o pedido.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true}, nil)
}

func validPedidoStatus(status string) bool {
	switch status {
	case "em_digitacao", "aguardando_aprovacao", "aprovado", "cancelado":
		return true
	default:
		return false
	}
}

func (h *Handler) ShellEmpresaAPI(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	result, err := h.Service.ShellEmpresaAPI(r.Context(), tenantID, empresaID, userID)
	if err != nil {
		writeServiceError(w, err, "Não foi possível consultar os dados da empresa.")
		return
	}
	writeJSON(w, http.StatusOK, result, nil)
}

func (h *Handler) NotificacoesAPI(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	limit, cursor, err := validPage(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", err.Error())
		return
	}
	var read *bool
	if value := r.URL.Query().Get("lida"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "lida deve ser true ou false.")
			return
		}
		read = &parsed
	}
	items, next, err := h.Service.NotificacoesAPI(r.Context(), tenantID, empresaID, userID, limit, cursor, read)
	if err != nil {
		writeServiceError(w, err, "Não foi possível listar notificações.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "nextCursor": cursorValue(next)}, nil)
}

func (h *Handler) ContagemNotificacoesAPI(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	count, err := h.Service.ContagemNotificacoes(r.Context(), tenantID, empresaID, userID)
	if err != nil {
		writeServiceError(w, err, "Não foi possível contar notificações.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"nao_lidas": count}, nil)
}

func (h *Handler) HomeShellNotificacoes(w http.ResponseWriter, r *http.Request) {
	h.ContagemNotificacoesAPI(w, r)
}

func (h *Handler) AgendaListar(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	limit, cursor, err := validPage(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", err.Error())
		return
	}
	concluida, err := boolQuery(r, "concluida")
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", err.Error())
		return
	}
	items, next, err := h.Service.AgendaListar(r.Context(), tenantID, empresaID, userID, limit, cursor, concluida)
	if err != nil {
		writeServiceError(w, err, "Não foi possível listar a agenda.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "nextCursor": cursorValue(next)}, nil)
}

func (h *Handler) AgendaCriar(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	var input models.AgendaCreate
	if !decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Titulo) == "" || strings.TrimSpace(input.DataAgendada) == "" {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "titulo e data_agendada são obrigatórios.")
		return
	}
	result, err := h.Service.CriarAgenda(r.Context(), tenantID, empresaID, userID, input)
	if err != nil {
		writeServiceError(w, err, "Não foi possível criar o compromisso.")
		return
	}
	writeJSON(w, http.StatusCreated, result, nil)
}

func (h *Handler) AgendaObter(w http.ResponseWriter, r *http.Request) {
	h.pessoalResource(w, r, "agenda")
}

func (h *Handler) AgendaAtualizar(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID inválido.")
		return
	}
	var input models.AgendaCreate
	if !decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Titulo) == "" || strings.TrimSpace(input.DataAgendada) == "" {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "titulo e data_agendada são obrigatórios.")
		return
	}
	result, err := h.Service.AtualizarAgenda(r.Context(), tenantID, empresaID, userID, id, input)
	if err != nil {
		writeServiceError(w, err, "Não foi possível atualizar o compromisso.")
		return
	}
	writeJSON(w, http.StatusOK, result, nil)
}

func (h *Handler) AgendaConcluir(w http.ResponseWriter, r *http.Request) {
	h.concluirPessoal(w, r, "agenda")
}

func (h *Handler) AgendaRemover(w http.ResponseWriter, r *http.Request) {
	h.removerPessoal(w, r, "agenda")
}

func (h *Handler) TarefasListarAPI(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	limit, cursor, err := validPage(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", err.Error())
		return
	}
	concluida, err := boolQuery(r, "concluida")
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", err.Error())
		return
	}
	prioridade := r.URL.Query().Get("prioridade")
	if prioridade != "" && prioridade != "baixa" && prioridade != "media" && prioridade != "alta" {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "prioridade deve ser baixa, media ou alta.")
		return
	}
	items, next, err := h.Service.TarefasListar(r.Context(), tenantID, empresaID, userID, limit, cursor, concluida, prioridade)
	if err != nil {
		writeServiceError(w, err, "Não foi possível listar as tarefas.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "nextCursor": cursorValue(next)}, nil)
}

func (h *Handler) TarefaCriar(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	var input models.TarefaCreate
	if !decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Titulo) == "" {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "titulo é obrigatório.")
		return
	}
	result, err := h.Service.CriarTarefa(r.Context(), tenantID, empresaID, userID, input)
	if err != nil {
		writeServiceError(w, err, "Não foi possível criar a tarefa.")
		return
	}
	writeJSON(w, http.StatusCreated, result, nil)
}

func (h *Handler) TarefaObter(w http.ResponseWriter, r *http.Request) {
	h.pessoalResource(w, r, "tarefa")
}

func (h *Handler) TarefaAtualizar(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID inválido.")
		return
	}
	var input models.TarefaCreate
	if !decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Titulo) == "" {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "titulo é obrigatório.")
		return
	}
	result, err := h.Service.AtualizarTarefa(r.Context(), tenantID, empresaID, userID, id, input)
	if err != nil {
		writeServiceError(w, err, "Não foi possível atualizar a tarefa.")
		return
	}
	writeJSON(w, http.StatusOK, result, nil)
}

func (h *Handler) TarefaConcluir(w http.ResponseWriter, r *http.Request) {
	h.concluirPessoal(w, r, "tarefa")
}

func (h *Handler) TarefaRemover(w http.ResponseWriter, r *http.Request) {
	h.removerPessoal(w, r, "tarefa")
}

func (h *Handler) pessoalResource(w http.ResponseWriter, r *http.Request, resource string) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID inválido.")
		return
	}
	var result map[string]any
	switch resource {
	case "agenda":
		result, err = h.Service.ObterAgenda(r.Context(), tenantID, empresaID, userID, id)
	case "tarefa":
		result, err = h.Service.ObterTarefa(r.Context(), tenantID, empresaID, userID, id)
	default:
		err = errors.New("recurso inválido")
	}
	if err != nil {
		writeServiceError(w, err, "Não foi possível consultar o recurso.")
		return
	}
	writeJSON(w, http.StatusOK, result, nil)
}

func (h *Handler) concluirPessoal(w http.ResponseWriter, r *http.Request, resource string) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID inválido.")
		return
	}
	var result map[string]any
	if resource == "agenda" {
		result, err = h.Service.ConcluirAgenda(r.Context(), tenantID, empresaID, userID, id)
	} else {
		result, err = h.Service.ConcluirTarefa(r.Context(), tenantID, empresaID, userID, id)
	}
	if err != nil {
		writeServiceError(w, err, "Não foi possível concluir o recurso.")
		return
	}
	writeJSON(w, http.StatusOK, result, nil)
}

func (h *Handler) removerPessoal(w http.ResponseWriter, r *http.Request, resource string) {
	tenantID, empresaID, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID inválido.")
		return
	}
	switch resource {
	case "agenda":
		err = h.Service.RemoverAgenda(r.Context(), tenantID, empresaID, userID, id)
	case "tarefa":
		err = h.Service.RemoverTarefa(r.Context(), tenantID, empresaID, userID, id)
	default:
		err = errors.New("recurso inválido")
	}
	if err != nil {
		writeServiceError(w, err, "Não foi possível remover o recurso.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true}, nil)
}

func boolQuery(r *http.Request, name string) (*bool, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, errors.New(name + " deve ser true ou false.")
	}
	return &parsed, nil
}
