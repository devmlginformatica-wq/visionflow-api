---
description: "Use when changing catálogos, produtos, estoque, tabelas de preço ou resolução de preço efetivo em internal/features/catalogo."
---
# Catálogos e preços

- Mantenha catálogos como consultas de leitura, com paginação por cursor e filtro de tenant/empresa. Para empresas, preserve também a associação do usuário.
- Ao listar produtos, resolva a tabela de preço nesta ordem: tabela explicitamente solicitada, tabela padrão do cliente, tabela de integração do item. Use somente tabelas ativas e linhas vigentes na data atual; se não houver preço de tabela selecionado, preserve o fallback existente para `item.preco_venda`.
- Não trate qualquer tabela como válida: valide `cliente_id` e `tabela_preco_id` no escopo da empresa antes da consulta.
- Preserve a seleção do preço mais recente aplicável, os limites de validade e o vínculo por `codigo_erp` entre item e linha da tabela.
- Preserve os campos de estoque, unidade, dimensões e disponibilidade retornados pela API; não introduza gravações neste domínio de consulta.
- Cubra precedência, validade, busca, paginação e filtros inválidos com testes de repository/service existentes.
