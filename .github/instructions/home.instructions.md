---
description: "Use when changing Home indicators, sales totals, pending work or unread counts in internal/features/home."
---
# Indicadores da Home

- Preserve a agregação em `internal/features/home` e execute consultas na sessão RLS correta para tenant, empresa e usuário.
- Mantenha a definição atual de vendas: pedidos não orçamentos, aprovados, não marcados para apagar e com itens, dentro do período solicitado.
- Preserve contagens separadas de pedidos/orçamentos em digitação, clientes prospect, pedidos aguardando aprovação, agenda/tarefas pendentes e notificações não lidas.
- Ao alterar qualquer indicador, atualize testes e confirme que totais e filtros não duplicam valores ao juntar `pedido` e `pedido_item`.
