---
description: "Use when changing notifications, unread counters, user scoping or notification pagination in internal/features/notificacao."
---
# Notificações

- Consulte notificações dentro de `db.NewSession` e restrinja tenant, empresa e `cod_usuario`; não exponha notificações de outra sessão.
- Preserve o filtro opcional `lida`, o cursor e a ordenação decrescente por `data_evento` e `codigo`.
- Preserve o endpoint de contagem de não lidas e o contrato atual somente de leitura. Não marque notificações como lidas sem implementar um contrato explícito.
- Mantenha a serialização de `data_evento`, `origem_tabela` e `origem_id`; cubra paginação, contagem e isolamento com testes.
