package relatorio

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSchemaTablesForQuestionSelectsRelevantTables(t *testing.T) {
	got := schemaTablesForQuestion("quais os 10 clientes com mais pedidos")
	want := []string{"cliente", "item", "pedido", "pedido_item"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schemaTablesForQuestion() = %v, want %v", got, want)
	}
}

func TestSchemaTablesForQuestionUsesCoreTablesForUnknownQuestion(t *testing.T) {
	got := schemaTablesForQuestion("resumo comercial")
	want := []string{"cliente", "item", "pedido", "pedido_item"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schemaTablesForQuestion() = %v, want %v", got, want)
	}
}

func TestExcludedSchemaTablesOmitInternalTables(t *testing.T) {
	excluded := make(map[string]bool, len(excludedSchemaTables))
	for _, table := range excludedSchemaTables {
		excluded[table] = true
	}
	for _, table := range []string{
		"flyway_schema_history",
		"tenant",
		"usuario",
		"usuario_empresa",
		"log_auditoria",
		"pedido_historico",
		"integracao_fila",
	} {
		if !excluded[table] {
			t.Errorf("tabela interna %q não está excluída", table)
		}
	}
}

func TestLoadSchemaFromCacheConcatenatesTablesForQuestion(t *testing.T) {
	filesystem := fstest.MapFS{
		"cliente.md":         {Data: []byte("### cliente\n- id: integer")},
		"cliente_contato.md": {Data: []byte("### cliente_contato\n- email: varchar")},
		"pedido.md":          {Data: []byte("conteúdo não selecionado")},
	}

	got, err := loadSchemaFromCache(filesystem, "clientes com email")
	if err != nil {
		t.Fatalf("loadSchemaFromCache retornou erro: %v", err)
	}
	want := "### cliente\n- id: integer\n\n### cliente_contato\n- email: varchar"
	if got != want {
		t.Fatalf("schema carregado = %q, want %q", got, want)
	}
}

func TestLoadSchemaFromCacheReportsMissingTable(t *testing.T) {
	filesystem := fstest.MapFS{
		"cliente.md": {Data: []byte("### cliente\n- id: integer")},
	}

	_, err := loadSchemaFromCache(filesystem, "clientes com email")
	if !errors.Is(err, ErrSchemaUnavailable) {
		t.Fatalf("esperava ErrSchemaUnavailable, recebeu %v", err)
	}
	if !strings.Contains(err.Error(), "cliente_contato.md") || !strings.Contains(err.Error(), "desatualizado") {
		t.Fatalf("erro não identificou arquivo ausente e cache possivelmente desatualizado: %v", err)
	}
}

func TestEnsureSchemaCacheRegeneratesWhenFlywayVersionChanges(t *testing.T) {
	cacheDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cacheDir, ".version"), []byte("18\n"), 0o644); err != nil {
		t.Fatalf("preparar versão do cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "_overview.md"), []byte("schema antigo"), 0o644); err != nil {
		t.Fatalf("preparar overview antigo: %v", err)
	}

	generated := false
	err := ensureSchemaCache(context.Background(), cacheDir, "19", func(_ context.Context, outputDir string) error {
		generated = true
		return os.WriteFile(filepath.Join(outputDir, "_overview.md"), []byte("schema novo"), 0o644)
	})
	if err != nil {
		t.Fatalf("ensureSchemaCache retornou erro: %v", err)
	}
	if !generated {
		t.Fatal("cache deveria ser regenerado quando a versão Flyway muda de 18 para 19")
	}

	version, err := os.ReadFile(filepath.Join(cacheDir, ".version"))
	if err != nil {
		t.Fatalf("ler versão atualizada: %v", err)
	}
	if string(version) != "19\n" {
		t.Fatalf("versão persistida = %q, want %q", version, "19\n")
	}
	overview, err := os.ReadFile(filepath.Join(cacheDir, "_overview.md"))
	if err != nil {
		t.Fatalf("ler overview regenerado: %v", err)
	}
	if string(overview) != "schema novo" {
		t.Fatalf("overview = %q, want %q", overview, "schema novo")
	}
}

func TestEnsureSchemaCacheSkipsGenerationWhenFlywayVersionMatches(t *testing.T) {
	cacheDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cacheDir, ".version"), []byte("19\n"), 0o644); err != nil {
		t.Fatalf("preparar versão do cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "_overview.md"), []byte("schema atual"), 0o644); err != nil {
		t.Fatalf("preparar overview atual: %v", err)
	}

	err := ensureSchemaCache(context.Background(), cacheDir, "19", func(context.Context, string) error {
		t.Fatal("geração não deveria ser chamada quando a versão Flyway não mudou")
		return nil
	})
	if err != nil {
		t.Fatalf("ensureSchemaCache retornou erro: %v", err)
	}
}
