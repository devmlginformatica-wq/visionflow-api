package relatorio

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tordrt/llmschema"

	"github.com/celsolenonsander/visionflow-api/internal/db"
)

const schemaCacheDir = "docs/db-schema"
const schemaCacheInitializationTimeout = 10 * time.Second

var excludedSchemaTables = []string{
	"flyway_schema_history",
	"revendedor",
	"tenant",
	"usuario",
	"usuario_empresa",
	"log_auditoria",
	"pedido_historico",
	"integracao_fila",
}

func InitializeSchemaCache(ctx context.Context, pool *db.Pool, logger ErrorLogger) error {
	cfg, err := loadRelatorioConfig()
	if err != nil {
		return fmt.Errorf("carregar configuração do schema para relatórios: %w", err)
	}
	if !cfg.DBIA.IsConfigured() {
		if logger != nil {
			logger.Printf("cache de schema não gerado: configure DB_IA_* para habilitar relatórios com IA")
		}
		return nil
	}

	if pool == nil || pool.Pool == nil {
		return fmt.Errorf("consultar versão Flyway: pool do banco indisponível")
	}

	initCtx, cancel := context.WithTimeout(ctx, schemaCacheInitializationTimeout)
	defer cancel()

	var version sql.NullString
	err = pool.QueryRow(
		initCtx,
		`SELECT version
		 FROM flyway_schema_history
		 WHERE success = true
		   AND version IS NOT NULL
		 ORDER BY installed_rank DESC
		 LIMIT 1`,
	).Scan(&version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("consultar versão Flyway: nenhuma migration aplicada com sucesso")
		}
		return fmt.Errorf("consultar versão Flyway: %w", err)
	}
	if !version.Valid || strings.TrimSpace(version.String) == "" {
		return fmt.Errorf("consultar versão Flyway: versão atual vazia")
	}

	generate := func(ctx context.Context, outputDir string) error {
		return llmschema.ExtractAndFormat(
			ctx,
			buildIAConnectionURL(cfg.DBIA),
			&llmschema.Options{
				SchemaName:    "public",
				ExcludeTables: excludedSchemaTables,
			},
			&llmschema.OutputOptions{
				OutputDir:        outputDir,
				OmitDatabaseInfo: true,
			},
		)
	}
	if err := ensureSchemaCache(initCtx, schemaCacheDir, version.String, generate); err != nil {
		return err
	}
	if logger != nil {
		logger.Printf("cache de schema da versão Flyway %s disponível em %s", version.String, schemaCacheDir)
	}
	return nil
}

func ensureSchemaCache(ctx context.Context, cacheDir, databaseVersion string, generate func(context.Context, string) error) error {
	versionPath := filepath.Join(cacheDir, ".version")
	cachedVersion, err := os.ReadFile(versionPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("ler versão do cache de schema %q: %w", versionPath, err)
	}
	if err == nil && strings.TrimSpace(string(cachedVersion)) == databaseVersion {
		if info, statErr := os.Stat(filepath.Join(cacheDir, "_overview.md")); statErr == nil && !info.IsDir() {
			return nil
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("verificar overview do cache de schema: %w", statErr)
		}
	}

	if err := os.RemoveAll(cacheDir); err != nil {
		return fmt.Errorf("remover cache de schema desatualizado %q: %w", cacheDir, err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("criar diretório do cache de schema %q: %w", cacheDir, err)
	}
	if err := generate(ctx, cacheDir); err != nil {
		return fmt.Errorf("gerar cache de schema para versão Flyway %s: %w", databaseVersion, err)
	}
	if err := os.WriteFile(versionPath, []byte(databaseVersion+"\n"), 0o644); err != nil {
		return fmt.Errorf("salvar versão do cache de schema %q: %w", versionPath, err)
	}
	return nil
}

func loadSchemaFromCache(filesystem fs.FS, question string) (string, error) {
	tables := schemaTablesForQuestion(question)
	if len(tables) == 0 {
		content, err := fs.ReadFile(filesystem, "_overview.md")
		if err != nil {
			return "", fmt.Errorf("%w: cache de schema pode estar desatualizado; arquivo _overview.md: %w", ErrSchemaUnavailable, err)
		}
		return string(content), nil
	}

	var schema strings.Builder
	for i, table := range tables {
		filename := table + ".md"
		content, err := fs.ReadFile(filesystem, filename)
		if err != nil {
			return "", fmt.Errorf("%w: cache de schema pode estar desatualizado; arquivo %q: %w", ErrSchemaUnavailable, filename, err)
		}
		if i > 0 {
			schema.WriteString("\n\n")
		}
		schema.Write(content)
	}
	return schema.String(), nil
}

func schemaTablesForQuestion(question string) []string {
	question = strings.ToLower(question)
	tables := make(map[string]struct{})
	add := func(names ...string) {
		for _, name := range names {
			tables[name] = struct{}{}
		}
	}
	matches := func(terms ...string) bool {
		for _, term := range terms {
			if strings.Contains(question, term) {
				return true
			}
		}
		return false
	}

	if matches("cliente", "client", "comprador") {
		add("cliente")
	}
	if matches("pedido", "venda", "orçamento", "orcamento") {
		add("cliente", "pedido", "pedido_item", "item")
	}
	if matches("produto", "produtos", "item", "itens", "estoque", "preço", "preco") {
		add("item", "item_estoque", "tabela_preco_item", "categoria_item", "fabricante", "unidade_medida")
	}
	if matches("conta a receber", "contas a receber", "título", "titulo", "financeiro", "inadimpl") {
		add("cliente", "cliente_conta_receber")
	}
	if matches("endereço", "endereco") {
		add("cliente", "cliente_endereco_adicional")
	}
	if matches("contato", "telefone", "email") {
		add("cliente", "cliente_contato")
	}
	if matches("agenda") {
		add("agenda", "cliente")
	}
	if matches("tarefa", "tarefas") {
		add("tarefa", "cliente")
	}
	if matches("notificação", "notificacao") {
		add("notificacao")
	}
	if matches("transportadora", "transportadoras") {
		add("transportadora")
	}
	if matches("condição de pagamento", "condicao de pagamento") {
		add("condicao_pagamento", "condicao_pagamento_forma", "condicao_pagamento_parcela")
	}
	if matches("forma de pagamento", "formas de pagamento") {
		add("forma_pagamento")
	}
	if matches("empresa", "empresas") {
		add("empresa")
	}

	if len(tables) == 0 {
		add("cliente", "pedido", "pedido_item", "item")
	}

	selected := make([]string, 0, len(tables))
	for name := range tables {
		selected = append(selected, name)
	}
	sort.Strings(selected)
	return selected
}
