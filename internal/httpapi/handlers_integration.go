package httpapi

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/celsolenonsander/visionflow-api/internal/service"
)

func (h *Handler) FinalizarClienteIntegracao(w http.ResponseWriter, r *http.Request) {
	h.finalizarIntegracao(w, r, "cliente")
}

func (h *Handler) FinalizarPedidoIntegracao(w http.ResponseWriter, r *http.Request) {
	h.finalizarIntegracao(w, r, "pedido")
}

func (h *Handler) FinalizarOrcamentoIntegracao(w http.ResponseWriter, r *http.Request) {
	h.finalizarIntegracao(w, r, "orcamento")
}

func (h *Handler) finalizarIntegracao(w http.ResponseWriter, r *http.Request, entity string) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do cadastro inválido.")
		return
	}
	result, err := h.Service.FinalizarIntegracao(r.Context(), tenantID, empresaID, id, entity)
	if err != nil {
		writeServiceError(w, err, "Não foi possível enviar o cadastro para integração.")
		return
	}
	writeJSON(w, http.StatusAccepted, result, nil)
}

func (h *Handler) IntegracaoPendentes(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	limit := 100
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > 100 {
			bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "limit deve estar entre 1 e 100.")
			return
		}
		limit = parsed
	}
	items, err := h.Service.IntegracaoPendentes(
		r.Context(), tenantID, empresaID, chi.URLParam(r, "entidade"), limit,
	)
	if err != nil {
		writeServiceError(w, err, "Não foi possível consultar a fila de integração.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items}, nil)
}

func (h *Handler) IntegracaoRetorno(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "ID do cadastro inválido.")
		return
	}
	var result service.IntegrationResult
	if !decode(w, r, &result) {
		return
	}
	response, err := h.Service.ProcessarRetornoIntegracao(
		r.Context(), tenantID, empresaID, id, chi.URLParam(r, "entidade"), result,
	)
	if err != nil {
		writeServiceError(w, err, "Não foi possível processar o retorno do ERP.")
		return
	}
	writeJSON(w, http.StatusOK, response, nil)
}
