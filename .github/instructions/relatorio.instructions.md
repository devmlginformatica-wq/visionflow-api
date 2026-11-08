---
description: "Use when changing Relatórios com IA, schema cache, Flyway invalidation, SQL guard or POST /relatorios/ia in internal/features/relatorio."
---
# Relatórios com IA

- Mantenha o fluxo em `internal/features/relatorio`: carregar schema local, solicitar SQL ao provedor, validar a consulta, executar no banco de leitura e montar a resposta.
- O schema fica em `docs/db-schema/`, é gerado no boot quando `DB_IA_*` está configurado e é ignorado pelo Git. Compare a última versão Flyway aplicada com sucesso em `flyway_schema_history` com `docs/db-schema/.version`; regenere se a versão diferir, o cache faltar ou `_overview.md` estiver ausente.
- Use `schemaTablesForQuestion` para selecionar somente os arquivos de tabelas relevantes. Atualize os mapeamentos e testes quando novas entidades/perguntas forem suportadas; use `_overview.md` apenas quando nenhuma palavra-chave corresponder.
- Trate `sqlguard.go` como barreira obrigatória: rejeite SQL vazio, múltiplas instruções (ponto e vírgula), qualquer instrução que não seja `SELECT` e funções perigosas. Valide também o SQL final após adicionar `LIMIT`; nunca execute SQL antes da validação.
- Preserve timeout e mensagens/status HTTP específicos para configuração, indisponibilidade do schema/IA, timeout e SQL rejeitado. Não transforme erros em sucesso vazio.
- Para testar `POST /relatorios/ia`, envie uma pergunta no JSON e valide HTTP 200 com resposta que inclua `colunas`, `linhas` e `sql_gerado`. Confirme também as rotas relevantes em `docs/api-tests.html` pelo navegador quando possível.
- Cubra seleção do schema, invalidação de cache, bloqueios do SQL, limite, respostas e erros com testes; não use LLM real em testes unitários.
