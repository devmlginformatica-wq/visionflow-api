---
description: "Use when changing agenda, compromissos, ownership, scheduling or completion endpoints in internal/features/agenda."
---
# Agenda e compromissos

- Use `db.NewSession` com tenant e empresa e restrinja cada registro por `cod_usuario` do usuário autenticado.
- Preserve validação e formatação de data/hora e a associação opcional ao cliente por `codigo_erp`.
- Filtre `deletado_em IS NULL` em listagem, detalhe, edição e conclusão. Exclusão é soft delete via `deletado_em=now()`.
- Preserve os estados e filtros de `concluida`, a paginação por cursor e o contrato de `PATCH /agenda/{id}/concluir`.
- Cubra isolamento entre usuários, registros removidos, validação de cliente e operações CRUD nos testes.
