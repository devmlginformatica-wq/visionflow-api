---
description: "Use when changing pedidos, orçamentos, pedido_item, pricing, cancellation or order endpoints in internal/service."
---
# Pedidos e orçamentos

- Pedidos e orçamentos continuam implementados na fachada `internal/service/api_service.go`; preserve os contratos existentes e as transações com tenant/empresa.
- Resolva IDs locais para `codigo_erp` somente dentro do escopo atual. Valide cliente, produto, tabela de preço, condição de pagamento, forma de pagamento, transportadora e endereços antes de gravar.
- Calcule o preço oficial pela tabela e faixa de quantidade vigentes na data de emissão; use o preço-base do item somente quando não houver tabela aplicável. Não confie no preço recebido: `effectivePedidoSalePrice` limita o preço informado ao preço oficial e os totais/descontos devem continuar consistentes.
- Preserve `pedido_item` como coleção filha do pedido, incluindo sequência única, produto/tabela ERP, preço original sugerido, preço praticado, quantidade, desconto e total líquido. Atualizar o pedido substitui os itens na mesma transação.
- Para excluir, defina `situacao_app='cancelado'`; não apague pedido nem itens. Pedidos cancelados ficam ocultos por padrão e só aparecem com `incluir_cancelados=true`.
- Não permita cancelar pedidos `em_integracao` ou `integrado`. Preserve as regras de edição e os estados atuais sem assumir propagação automática ao ERP.
- Mantenha a distinção pedido/orçamento (`e_um_orcamento`), estados do app e status do ERP. Cubra preço, itens, atualização, cancelamento e paginação com testes.
