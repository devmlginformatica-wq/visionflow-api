package relatorio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubReportService struct {
	response RelatorioResponse
	err      error
}

func (s stubReportService) Generate(context.Context, int, int, string) (RelatorioResponse, error) {
	return s.response, s.err
}

type panickingReportService struct{}

func (panickingReportService) Generate(context.Context, int, int, string) (RelatorioResponse, error) {
	panic("unexpected failure")
}

type reportTestLogger struct {
	lines []string
}

func (l *reportTestLogger) Printf(format string, args ...interface{}) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func TestHandlerSuccess(t *testing.T) {
	handler := NewHandler(stubReportService{
		response: RelatorioResponse{
			Colunas:        []string{"nome"},
			Linhas:         [][]any{{"Ana"}},
			SQLGerado:      "SELECT nome FROM cliente",
			ProvedorLLM:    "fake",
			LimiteAplicado: 1000,
		},
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/relatorios/ia", strings.NewReader(`{"pergunta":"clientes"}`))

	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body)
	}
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Colunas []string `json:"colunas"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("resposta JSON inválida: %v", err)
	}
	if !response.Success || len(response.Data.Colunas) != 1 || response.Data.Colunas[0] != "nome" {
		t.Fatalf("envelope de sucesso inesperado: %+v", response)
	}
}

func TestHandlerBadRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
		err  error
	}{
		{name: "pergunta vazia", body: `{"pergunta":""}`},
		{name: "SQL inválido", body: `{"pergunta":"clientes"}`, err: ErrSQLRejected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(stubReportService{err: tt.err})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/relatorios/ia", strings.NewReader(tt.body))

			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body)
			}
		})
	}
}

func TestHandlerFeatureNotConfigured(t *testing.T) {
	handler := NewHandler(stubReportService{err: ErrFeatureNotConfigured})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/relatorios/ia", strings.NewReader(`{"pergunta":"clientes"}`))

	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body)
	}
}

func TestHandlerSchemaUnavailable(t *testing.T) {
	handler := NewHandler(stubReportService{err: ErrSchemaUnavailable})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/relatorios/ia", strings.NewReader(`{"pergunta":"clientes"}`))

	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body)
	}
}

func TestHandlerUnexpectedErrorIsInternalServerError(t *testing.T) {
	handler := NewHandler(stubReportService{err: errors.New("db down")})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/relatorios/ia", strings.NewReader(`{"pergunta":"clientes"}`))

	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body)
	}
}

func TestHandlerRecoversPanicsAndLogsStackTrace(t *testing.T) {
	logger := &reportTestLogger{}
	handler := NewHandler(panickingReportService{}, logger)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/relatorios/ia", strings.NewReader(`{"pergunta":"clientes"}`))

	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body)
	}
	if !strings.Contains(recorder.Body.String(), "Erro interno ao processar relatório.") {
		t.Fatalf("resposta não contém mensagem de erro interna: %s", recorder.Body)
	}
	if len(logger.lines) != 1 || !strings.Contains(logger.lines[0], "unexpected failure") ||
		!strings.Contains(logger.lines[0], "stack:") || !strings.Contains(logger.lines[0], "ServeHTTP") {
		t.Fatalf("panic e stack trace não foram registrados: %+v", logger.lines)
	}
}

func TestHandlerLogsUnexpectedServiceErrors(t *testing.T) {
	logger := &reportTestLogger{}
	handler := NewHandler(stubReportService{err: errors.New("unexpected database failure")}, logger)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/relatorios/ia", strings.NewReader(`{"pergunta":"clientes"}`))

	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body)
	}
	if len(logger.lines) != 1 || !strings.Contains(logger.lines[0], "unexpected database failure") {
		t.Fatalf("erro inesperado do serviço não foi registrado: %+v", logger.lines)
	}
}
