---
description: "Use when changing clientes, endereços, contas a receber, soft delete, RLS or ERP mapping in internal/features/cliente."
---
# Clientes

- Faça leituras e gravações dentro de `db.NewSession`, com tenant e empresa da sessão; mantenha os filtros e chaves compostas em todas as consultas. O PostgreSQL aplica RLS pelo contexto transacional `SET LOCAL app.cod_tenant` e `app.cod_empresa`; nunca use `SET` de sessão nem reutilize contexto fora da transação.
- Trate `codigo` como identificador local e `codigo_erp` como chave externa. Preserve os dois ao resolver clientes e endereços importados do ERP e ao montar referências em pedidos.
- Use `deletado_em IS NULL` em leituras e atualizações de clientes e endereços ativos. Exclusões devem marcar `deletado_em`, nunca apagar fisicamente.
- Preserve o bloqueio de atualização, exclusão e edição de endereços quando `situacao_app` for `em_integracao` ou `integrado`; retorne o erro de integração existente.
- Valide duplicidade e referências comerciais (transportadora, condição, forma e tabela de preço) no tenant/empresa. Não aceite IDs de outra empresa.
- Mantenha o mapeamento e as mensagens dos serviços externos de CEP/CNPJ e seus testes com transporte mockado.
- Ao alterar consultas, confira que endereços e contas a receber pertencem ao cliente solicitado e não incluem registros removidos.
