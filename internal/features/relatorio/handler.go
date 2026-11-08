package relatorio

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/celsolenonsander/visionflow-api/internal/models"
)

type reportService interface {
	Generate(context.Context, int, int, string) (RelatorioResponse, error)
}

type Handler struct {
	service reportService
	logger  ErrorLogger
}

func NewHandler(service reportService, loggers ...ErrorLogger) *Handler {
	handler := &Handler{service: service}
	if len(loggers) > 0 {
		handler.logger = loggers[0]
	}
	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if recovered := recover(); recovered != nil {
			method, path := "", ""
			if r != nil && r.URL != nil {
				method, path = r.Method, r.URL.Path
			}
			if h != nil && h.logger != nil {
				h.logger.Printf("panic ao processar relatório method=%s path=%s panic=%v stack:\n%s", method, path, recovered, debug.Stack())
			}
			writeHandlerError(w, http.StatusInternalServerError, "ERRO_INTERNO", "Erro interno ao processar relatório.")
		}
	}()

	if h == nil || h.service == nil {
		writeHandlerError(w, http.StatusServiceUnavailable, "FEATURE_NAO_CONFIGURADA", ErrFeatureNotConfigured.Error())
		return
	}
	var req struct {
		Pergunta string `json:"pergunta"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeHandlerError(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "JSON inválido")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeHandlerError(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "A requisição deve conter apenas um objeto JSON")
		return
	}
	if strings.TrimSpace(req.Pergunta) == "" {
		writeHandlerError(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "pergunta é obrigatória")
		return
	}
	resp, err := h.service.Generate(r.Context(), 0, 0, req.Pergunta)
	if err != nil {
		if errors.Is(err, ErrSQLRejected) {
			writeHandlerError(w, http.StatusBadRequest, "SQL_INVALIDO", err.Error())
			return
		}
		if errors.Is(err, ErrLLMTimeout) {
			writeHandlerError(w, http.StatusGatewayTimeout, "TIMEOUT_IA", ErrLLMTimeout.Error())
			return
		}
		if errors.Is(err, ErrFeatureNotConfigured) {
			writeHandlerError(w, http.StatusServiceUnavailable, "FEATURE_NAO_CONFIGURADA", err.Error())
			return
		}
		if errors.Is(err, ErrSchemaUnavailable) {
			writeHandlerError(w, http.StatusServiceUnavailable, "SCHEMA_INDISPONIVEL", "Não foi possível carregar o schema do banco para gerar o relatório")
			return
		}
		if errors.Is(err, ErrLLMUnavailable) {
			writeHandlerError(w, http.StatusServiceUnavailable, "IA_INDISPONIVEL", err.Error())
			return
		}
		if h.logger != nil {
			h.logger.Printf("falha inesperada ao gerar relatório method=%s path=%s err=%v", r.Method, r.URL.Path, err)
		}
		writeHandlerError(w, http.StatusInternalServerError, "ERRO_INTERNO", "Não foi possível gerar o relatório")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(models.APIResponse{Success: true, Data: resp, Error: nil})
}

func writeHandlerError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(models.APIResponse{
		Success: false,
		Data:    nil,
		Error:   &models.APIError{Code: code, Message: message},
	})
}
