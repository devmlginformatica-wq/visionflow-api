package relatorio

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/fake"
)

type promptCapturingLLM struct {
	messages []llms.MessageContent
	options  llms.CallOptions
}

type emptyResponseLLM struct{}

func (emptyResponseLLM) GenerateContent(context.Context, []llms.MessageContent, ...llms.CallOption) (*llms.ContentResponse, error) {
	return &llms.ContentResponse{}, nil
}

func (emptyResponseLLM) Call(context.Context, string, ...llms.CallOption) (string, error) {
	return "", nil
}

func (l *promptCapturingLLM) GenerateContent(_ context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	l.messages = messages
	for _, option := range options {
		option(&l.options)
	}
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: "SELECT id FROM cliente"}},
	}, nil
}

func (l *promptCapturingLLM) Call(context.Context, string, ...llms.CallOption) (string, error) {
	return "", nil
}

type fakeExecutor struct {
	called  bool
	sql     string
	columns []string
	rows    [][]any
	err     error
}

func testSchema(context.Context, string) (string, error) {
	return "### cliente\n- codigo_erp: varchar\n- nome_razao_social: varchar", nil
}

func (f *fakeExecutor) Execute(_ context.Context, _, _ int, sql string) ([]string, [][]any, error) {
	f.called = true
	f.sql = sql
	return f.columns, f.rows, f.err
}

type capturedLogger struct {
	lines []string
}

func (l *capturedLogger) Printf(format string, args ...interface{}) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func TestServiceGenerateSuccess(t *testing.T) {
	executor := &fakeExecutor{
		columns: []string{"nome", "total"},
		rows:    [][]any{{"Mercadinho X", 42}, {"Mercadinho Y", 31}},
	}
	service := NewWithDependencies(
		fake.NewFakeLLM([]string{"SELECT nome, total FROM cliente"}),
		"fake",
		executor,
		1000,
		time.Second,
		&capturedLogger{},
		testSchema,
	)

	got, err := service.Generate(context.Background(), 1, 2, "clientes com mais pedidos")
	if err != nil {
		t.Fatalf("Generate retornou erro: %v", err)
	}
	if !executor.called || executor.sql != "SELECT nome, total FROM cliente LIMIT 1000" {
		t.Fatalf("SQL recebido pelo repository: called=%v sql=%q", executor.called, executor.sql)
	}
	if got.Estatisticas.TotalRegistros != 2 || got.Estatisticas.ColunasNumericas["total"].Soma != 73 {
		t.Fatalf("estatísticas não foram calculadas: %+v", got.Estatisticas)
	}
	if got.GraficoSugerido == nil || got.GraficoSugerido.Tipo != "bar" {
		t.Fatalf("gráfico sugerido inesperado: %+v", got.GraficoSugerido)
	}
	if got.ProvedorLLM != "fake" || got.LimiteAplicado != 1000 {
		t.Fatalf("provedor/limite inesperados: provider=%q limit=%d", got.ProvedorLLM, got.LimiteAplicado)
	}
}

func TestServiceSendsSQLInstructionsAsSystemPrompt(t *testing.T) {
	model := &promptCapturingLLM{}
	service := NewWithDependencies(model, "fake", &fakeExecutor{}, 1000, time.Second, nil, testSchema)

	if _, err := service.Generate(context.Background(), 1, 2, "clientes ativos"); err != nil {
		t.Fatalf("Generate retornou erro: %v", err)
	}
	if len(model.messages) != 2 {
		t.Fatalf("esperava mensagem de sistema e pergunta, recebeu %d mensagens", len(model.messages))
	}
	if model.messages[0].Role != llms.ChatMessageTypeSystem {
		t.Fatalf("primeira mensagem deveria ser system, recebeu %q", model.messages[0].Role)
	}
	systemPrompt, ok := model.messages[0].Parts[0].(llms.TextContent)
	if !ok {
		t.Fatalf("conteúdo do system prompt tem tipo inesperado: %T", model.messages[0].Parts[0])
	}
	for _, instruction := range []string{
		"Use APENAS as tabelas e colunas listadas no schema abaixo",
		"Gere apenas uma única instrução SELECT. Não use múltiplos comandos separados por ponto e vírgula (;).",
		"Não inclua explicações, markdown ou blocos de código na resposta.",
		"Retorne apenas a string SQL pura.",
		"Aqui está o schema do banco de dados:",
		"### cliente\n- codigo_erp: varchar\n- nome_razao_social: varchar",
	} {
		if !strings.Contains(systemPrompt.Text, instruction) {
			t.Errorf("system prompt não contém a instrução %q", instruction)
		}
	}
	if model.messages[1].Role != llms.ChatMessageTypeHuman {
		t.Fatalf("pergunta deveria ser mensagem human, recebeu %q", model.messages[1].Role)
	}
	if len(model.options.StopWords) != 1 || model.options.StopWords[0] != ";" {
		t.Fatalf("geração deveria parar no primeiro ponto e vírgula: %v", model.options.StopWords)
	}
	if model.options.MaxTokens != 4096 {
		t.Fatalf("limite de tokens deveria ser 4096, recebeu %d", model.options.MaxTokens)
	}
	question, ok := model.messages[1].Parts[0].(llms.TextContent)
	if !ok || question.Text != "Pergunta: clientes ativos" {
		t.Fatalf("pergunta enviada incorretamente: %#v", model.messages[1].Parts)
	}
}

func TestServiceRejectsInvalidSQLBeforeRepository(t *testing.T) {
	executor := &fakeExecutor{}
	service := NewWithDependencies(
		fake.NewFakeLLM([]string{"DELETE FROM cliente"}),
		"fake",
		executor,
		1000,
		time.Second,
		&capturedLogger{},
		testSchema,
	)

	_, err := service.Generate(context.Background(), 1, 2, "apague clientes")
	if !errors.Is(err, ErrSQLRejected) {
		t.Fatalf("esperava ErrSQLRejected, recebeu %v", err)
	}
	if executor.called {
		t.Fatal("repository não deveria executar SQL rejeitado")
	}
}

func TestServiceLLMTimeout(t *testing.T) {
	executor := &fakeExecutor{}
	logg := &capturedLogger{}
	service := NewWithDependencies(
		fake.NewFakeLLM([]string{"SELECT nome FROM cliente"}),
		"fake",
		executor,
		1000,
		time.Second,
		logg,
		testSchema,
	)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := service.Generate(ctx, 1, 2, "clientes")
	if !errors.Is(err, ErrLLMTimeout) {
		t.Fatalf("esperava timeout do LLM, recebeu %v", err)
	}
	if len(logg.lines) != 1 || !strings.Contains(logg.lines[0], "falha na chamada ao LLM") ||
		!strings.Contains(logg.lines[0], "context deadline exceeded") {
		t.Fatalf("erro original do LLM não foi logado: %+v", logg.lines)
	}
	if executor.called {
		t.Fatal("repository não deveria ser chamado após timeout")
	}
}

func TestServiceLimitCanBeDisabled(t *testing.T) {
	executor := &fakeExecutor{columns: []string{"nome"}, rows: [][]any{{"Ana"}}}
	service := NewWithDependencies(
		fake.NewFakeLLM([]string{"SELECT nome FROM cliente"}),
		"fake",
		executor,
		0,
		time.Second,
		&capturedLogger{},
		testSchema,
	)

	got, err := service.Generate(context.Background(), 1, 2, "clientes")
	if err != nil {
		t.Fatalf("Generate retornou erro: %v", err)
	}
	if executor.sql != "SELECT nome FROM cliente" || got.LimiteAplicado != 0 {
		t.Fatalf("LIMIT deveria estar desativado; sql=%q limit=%d", executor.sql, got.LimiteAplicado)
	}
}

func TestServiceLogsSQLExecutionFailure(t *testing.T) {
	repositoryErr := errors.New("db indisponível")
	executor := &fakeExecutor{err: repositoryErr}
	logg := &capturedLogger{}
	service := NewWithDependencies(
		fake.NewFakeLLM([]string{"SELECT nome FROM cliente"}),
		"fake",
		executor,
		1000,
		time.Second,
		logg,
		testSchema,
	)

	_, err := service.Generate(context.Background(), 1, 2, "clientes")
	if !errors.Is(err, repositoryErr) {
		t.Fatalf("esperava erro original do repository, recebeu %v", err)
	}
	if len(logg.lines) != 1 || !strings.Contains(logg.lines[0], "falha ao executar SQL") ||
		!strings.Contains(logg.lines[0], executor.sql) || !strings.Contains(logg.lines[0], repositoryErr.Error()) {
		t.Fatalf("falha SQL não foi logada com erro e query: %+v", logg.lines)
	}
}

func TestServiceLogsRejectedSQL(t *testing.T) {
	executor := &fakeExecutor{}
	logg := &capturedLogger{}
	service := NewWithDependencies(
		fake.NewFakeLLM([]string{"SELECT pg_sleep(1)"}),
		"fake",
		executor,
		1000,
		time.Second,
		logg,
		testSchema,
	)

	_, err := service.Generate(context.Background(), 1, 2, "aguarde")
	if !errors.Is(err, ErrSQLRejected) {
		t.Fatalf("esperava SQL rejeitado, recebeu %v", err)
	}
	if len(logg.lines) != 1 || !strings.Contains(logg.lines[0], "SQL rejeitado") ||
		!strings.Contains(logg.lines[0], "função perigosa") {
		t.Fatalf("motivo da rejeição não foi logado: %+v", logg.lines)
	}
}

func TestServiceSchemaReadDoesNotUseExtractionTimeout(t *testing.T) {
	model := &promptCapturingLLM{}
	schema := func(ctx context.Context, _ string) (string, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("contexto da chamada deveria manter o deadline do LLM")
		}
		if remaining := time.Until(deadline); remaining <= schemaCacheInitializationTimeout {
			t.Fatalf("leitura local não deveria usar timeout curto de extração: restante=%s", remaining)
		}
		return testSchema(ctx, "")
	}
	service := NewWithDependencies(model, "fake", &fakeExecutor{}, 1000, 30*time.Second, nil, schema)

	if _, err := service.Generate(context.Background(), 1, 2, "clientes ativos"); err != nil {
		t.Fatalf("Generate retornou erro: %v", err)
	}
}

func TestServiceStopsWhenSchemaExtractionFails(t *testing.T) {
	extractionErr := errors.New("banco indisponível")
	model := &promptCapturingLLM{}
	executor := &fakeExecutor{}
	logg := &capturedLogger{}
	service := NewWithDependencies(
		model,
		"fake",
		executor,
		1000,
		time.Second,
		logg,
		func(context.Context, string) (string, error) {
			return "", extractionErr
		},
	)

	_, err := service.Generate(context.Background(), 1, 2, "clientes ativos")
	if !errors.Is(err, ErrSchemaUnavailable) || !errors.Is(err, extractionErr) {
		t.Fatalf("esperava erro de schema preservando a causa original, recebeu %v", err)
	}
	if len(model.messages) != 0 || executor.called {
		t.Fatal("LLM e repository não deveriam ser chamados quando a extração falha")
	}
	if len(logg.lines) != 1 || !strings.Contains(logg.lines[0], extractionErr.Error()) {
		t.Fatalf("falha de extração não foi logada: %+v", logg.lines)
	}
}

func TestServiceClassifiesEmptyLLMResponseAsUnavailable(t *testing.T) {
	logger := &capturedLogger{}
	service := NewWithDependencies(
		emptyResponseLLM{},
		"fake",
		&fakeExecutor{},
		1000,
		time.Second,
		logger,
		testSchema,
	)

	_, err := service.Generate(context.Background(), 1, 2, "clientes ativos")
	if !errors.Is(err, ErrLLMUnavailable) || !strings.Contains(err.Error(), "resposta vazia do provedor de IA") {
		t.Fatalf("esperava falha de LLM preservando a causa original, recebeu %v", err)
	}
	if len(logger.lines) != 1 || !strings.Contains(logger.lines[0], "resposta vazia do provedor de IA") {
		t.Fatalf("resposta vazia do LLM não foi logada: %+v", logger.lines)
	}
}
