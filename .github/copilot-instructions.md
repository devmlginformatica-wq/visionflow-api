# Instruções de Desenvolvimento — VisionFlow API

## Contexto e stack

- Mantenha a API em Go 1.25+, com `chi` v5 para rotas, `pgx` v5 para PostgreSQL e Flyway 13.7 para migrations.
- Preserve o fluxo em camadas: handler HTTP valida/serializa, service aplica regras de negócio e repository executa consultas. Pedidos e orçamentos ainda usam a fachada em `internal/service`; não os trate como módulos já migrados.
- Use `internal/db.NewSession` para operações de negócio no banco. A sessão abre transação e define o contexto de tenant/empresa para RLS.

## Regras de implementação

- Escreva identificadores e código em inglês; mantenha mensagens de erro visíveis ao usuário em português.
- Não use `panic`. Propague erros com contexto usando `%w` e siga o tratamento HTTP já existente.
- Use o logger estruturado de `internal/logger`; não use `fmt.Println` para diagnóstico.
- Mantenha handlers finos e preserve os contratos JSON e HTTP já publicados.
- Parametrize valores SQL; não concatene entrada do usuário em consultas.
- Não faça exclusão física de registros que seguem soft delete. Preserve o escopo `cod_tenant`/`cod_empresa` e a propriedade do usuário em todas as consultas aplicáveis.
- Ao alterar schema, consulte o estado do Flyway, crie uma nova migration versionada e não edite migrations já aplicadas. Não aplique migrations a outro ambiente sem confirmar o impacto.
- Adicione ou atualize testes para o comportamento alterado e documentação diretamente relacionada. Não versione `.env`, credenciais ou arquivos gerados.
- Mantenha `.github/` versionado para compartilhar estas instruções com a equipe; não o adicione ao `.gitignore` nem às regras de build.

## Testes e validação

- Antes de concluir alterações de código, rode `go test ./...` e `go build ./...` na raiz da API.
- Assuma que `.env` e o banco de desenvolvimento estão preparados. Não exponha nem copie valores secretos do ambiente.
- Para validar fluxos ponta a ponta da API, abra `docs/api-tests.html` no navegador integrado, execute os testes relevantes e confira as respostas. Não substitua a validação de browser por uma suposição de sucesso.
- Se a mudança não puder ser verificada pelo ambiente configurado, registre claramente o que foi e não foi validado.

## Instruções Específicas por Domínio

- Se a tarefa envolver autenticação, siga obrigatoriamente [auth.instructions.md](./instructions/auth.instructions.md).
- Se a tarefa envolver **Clientes**, siga obrigatoriamente [cliente.instructions.md](./instructions/cliente.instructions.md).
- Se a tarefa envolver catálogo, produtos ou preços, siga obrigatoriamente [catalogo.instructions.md](./instructions/catalogo.instructions.md).
- Se a tarefa envolver **Pedidos ou Orçamentos**, siga obrigatoriamente [pedido.instructions.md](./instructions/pedido.instructions.md).
- Se a tarefa envolver agenda ou compromissos, siga obrigatoriamente [agenda.instructions.md](./instructions/agenda.instructions.md).
- Se a tarefa envolver tarefas, siga obrigatoriamente [tarefa.instructions.md](./instructions/tarefa.instructions.md).
- Se a tarefa envolver **Relatórios com IA**, siga obrigatoriamente [relatorio.instructions.md](./instructions/relatorio.instructions.md).
- Se a tarefa envolver notificações, siga obrigatoriamente [notificacao.instructions.md](./instructions/notificacao.instructions.md).
- Se a tarefa envolver indicadores da Home, siga obrigatoriamente [home.instructions.md](./instructions/home.instructions.md).
- Se a tarefa envolver dados de sessão da interface (Shell), siga obrigatoriamente [shell.instructions.md](./instructions/shell.instructions.md).
