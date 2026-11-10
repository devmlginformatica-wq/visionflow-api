package relatorio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"

	"github.com/celsolenonsander/visionflow-api/internal/db"
)

var ErrFeatureNotConfigured = errors.New("feature de IA não configurada")
var ErrLLMUnavailable = errors.New("Serviço de IA indisponível no momento")
var ErrLLMTimeout = errors.New("tempo limite do provedor de IA excedido")
var ErrSchemaUnavailable = errors.New("schema do banco indisponível")
var ErrSQLRejected = errors.New("SQL rejeitado")

const promptSQL = `Você é um assistente que gera SQL PostgreSQL somente leitura.
Use APENAS as tabelas e colunas listadas no schema abaixo para gerar a consulta SQL.
NUNCA retorne colunas de controle (cod_tenant, cod_empresa, codigo, versao_layout, codigo_erp, cod_usuario) a menos que o usuário peça.
Para perguntas sobre faturamento, priorize colunas 'valor', 'total', 'preco' e faça JOIN com pedido e pedido_item.
O banco já filtra os dados por perfil (RLS). Não adicione WHERE de segurança.
Gere apenas uma única instrução SELECT. Não use múltiplos comandos separados por ponto e vírgula (;).
Não inclua explicações, markdown ou blocos de código na resposta.
Retorne apenas a string SQL pura.
Não use comentários, funções perigosas ou comandos DML.`

type schemaProvider func(context.Context, string) (string, error)

type Executor interface {
	Execute(context.Context, int, int, int, string) ([]string, [][]any, error)
}

type ErrorLogger interface {
	Printf(string, ...interface{})
}

type Service struct {
	llm          llms.Model
	llmProvider  string
	repository   Executor
	limitDefault int
	timeout      time.Duration
	schema       schemaProvider
	configErr    error
	logger       ErrorLogger
}

func New(pool *db.Pool, loggers ...ErrorLogger) *Service {
	cfg, err := loadRelatorioConfig()
	service := &Service{limitDefault: cfg.LimitDefault, timeout: cfg.LLM.Timeout}
	if len(loggers) > 0 {
		service.logger = loggers[0]
	}
	if err != nil {
		service.configErr = err
		return service
	}
	service.schema = func(ctx context.Context, question string) (string, error) {
		return loadSchemaFromCache(os.DirFS(schemaCacheDir), question)
	}
	if !cfg.DBIA.IsConfigured() || strings.TrimSpace(cfg.LLM.Provider) == "" {
		service.configErr = ErrFeatureNotConfigured
		return service
	}
	llm, llmErr := newLLM(cfg.LLM)
	if llmErr != nil {
		service.configErr = llmErr
		return service
	}
	repo, repoErr := newRepository(cfg.DBIA)
	if repoErr != nil {
		service.configErr = repoErr
		return service
	}
	service.llm = llm
	service.llmProvider = strings.ToLower(cfg.LLM.Provider)
	service.repository = repo
	return service
}

func NewWithDependencies(model llms.Model, provider string, executor Executor, limitDefault int, timeout time.Duration, logg ErrorLogger, schema schemaProvider) *Service {
	return &Service{
		llm:          model,
		llmProvider:  provider,
		repository:   executor,
		limitDefault: limitDefault,
		timeout:      timeout,
		schema:       schema,
		logger:       logg,
	}
}

func (s *Service) Generate(ctx context.Context, tenantID, empresaID, userID int, pergunta string) (RelatorioResponse, error) {
	if s == nil || s.configErr != nil {
		if s != nil && s.configErr != nil {
			return RelatorioResponse{}, s.configErr
		}
		return RelatorioResponse{}, ErrFeatureNotConfigured
	}
	if strings.TrimSpace(pergunta) == "" {
		return RelatorioResponse{}, fmt.Errorf("pergunta inválida")
	}
	if s.llm == nil || s.repository == nil {
		return RelatorioResponse{}, ErrFeatureNotConfigured
	}
	callCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	if err := callCtx.Err(); err != nil {
		s.logLLMError(err)
		return RelatorioResponse{}, ErrLLMTimeout
	}

	if s.schema == nil {
		return RelatorioResponse{}, ErrFeatureNotConfigured
	}
	schema, err := s.schema(callCtx, pergunta)
	if err != nil {
		s.logSchemaError(err)
		return RelatorioResponse{}, fmt.Errorf("%w: %w", ErrSchemaUnavailable, err)
	}
	if strings.TrimSpace(schema) == "" {
		err := errors.New("schema extraído vazio")
		s.logSchemaError(err)
		return RelatorioResponse{}, fmt.Errorf("%w: %w", ErrSchemaUnavailable, err)
	}

	response, err := s.llm.GenerateContent(callCtx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, promptSQL+"\n\nAqui está o schema do banco de dados:\n\n"+schema),
		llms.TextParts(llms.ChatMessageTypeHuman, "Pergunta: "+pergunta),
	}, llms.WithStopWords([]string{";"}), llms.WithMaxTokens(4096))
	if err == nil {
		err = callCtx.Err()
	}
	if err != nil {
		s.logLLMError(err)
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return RelatorioResponse{}, ErrLLMTimeout
		}
		return RelatorioResponse{}, ErrLLMUnavailable
	}

	sql, err := extractSQLFromResponse(response)
	if err != nil {
		s.logLLMError(err)
		return RelatorioResponse{}, fmt.Errorf("%w: %w", ErrLLMUnavailable, err)
	}
	if err := validateReadOnlySQL(sql); err != nil {
		s.logSQLRejected(err)
		return RelatorioResponse{}, fmt.Errorf("%w: %w", ErrSQLRejected, err)
	}
	finalSQL, limiteAplicado, err := addLimitIfMissing(sql, s.limitDefault)
	if err != nil {
		return RelatorioResponse{}, err
	}
	if err := validateReadOnlySQL(finalSQL); err != nil {
		s.logSQLRejected(err)
		return RelatorioResponse{}, fmt.Errorf("%w: %w", ErrSQLRejected, err)
	}

	start := time.Now()
	cols, rows, err := s.repository.Execute(callCtx, tenantID, empresaID, userID, finalSQL)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("falha ao executar SQL err=%v sql=%q", err, finalSQL)
		}
		return RelatorioResponse{}, err
	}
	elapsed := time.Since(start)
	stats := newEstatisticas(cols, rows)
	graph := recomendarGrafico(cols, rows)
	return RelatorioResponse{
		Colunas:         cols,
		Linhas:          rows,
		SQLGerado:       finalSQL,
		TempoMS:         elapsed.Milliseconds(),
		ProvedorLLM:     s.llmProvider,
		LimiteAplicado:  limiteAplicado,
		Estatisticas:    stats,
		GraficoSugerido: graph,
	}, nil
}

func (s *Service) logLLMError(err error) {
	if s.logger != nil {
		s.logger.Printf("falha na chamada ao LLM err=%v", err)
	}
}

func (s *Service) logSQLRejected(err error) {
	if s.logger != nil {
		s.logger.Printf("SQL rejeitado motivo=%v", err)
	}
}

func (s *Service) logSchemaError(err error) {
	if s.logger != nil {
		s.logger.Printf("falha ao carregar cache de schema; o cache pode estar desatualizado err=%v", err)
	}
}

func extractSQLFromResponse(resp *llms.ContentResponse) (string, error) {
	if resp == nil {
		return "", fmt.Errorf("resposta vazia do provedor de IA")
	}
	message := resp.AssistantMessage()
	var parts []string
	for _, part := range message.Parts {
		if text, ok := part.(llms.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("resposta vazia do provedor de IA")
	}
	sql := normalizeSQL(strings.Join(parts, "\n"))
	if sql == "" {
		return "", fmt.Errorf("resposta vazia do provedor de IA")
	}
	return sql, nil
}
