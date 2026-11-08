---
description: "Use when changing tarefas, priorities, deadlines, ownership or completion endpoints in internal/features/tarefa."
---
# Tarefas

- Use `db.NewSession` com tenant e empresa e restrinja cada tarefa por `cod_usuario` do usuário autenticado.
- Preserve prazo, prioridade, estado `concluida`, filtros e formatação de data existentes.
- Filtre `deletado_em IS NULL` em listagem, detalhe, edição e conclusão. Exclusão deve atualizar `deletado_em`, sem apagar fisicamente.
- Preserve a paginação por cursor e o contrato de `PATCH /tarefas/{id}/concluir`.
- Cubra isolamento entre usuários, registros removidos, filtros de prioridade/conclusão e operações CRUD nos testes.
