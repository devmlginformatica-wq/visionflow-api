# Relatório de campos sem cobertura completa da API

**Data:** 2026-10-19  
**Objetivo:** identificar campos que ainda recebem valores sintéticos, não são persistidos ou não podem ser recarregados nas telas ativas, e especificar as mudanças necessárias no contrato/backend.

## Resumo executivo

A inspeção do roteador e dos ViewModels indica que as telas ativas de autenticação, shell, home, clientes e pedidos selecionam repositórios de API. Não foi encontrada seleção de `Mock*Repository` nesses fluxos. Os repositórios mock e os JSONs continuam no código para uso de desenvolvimento/testes; a cópia não roteada `src/features/pedido (copy)` ainda contém seleção por `VITE_USE_API` e não representa o fluxo ativo.

As lacunas que afetam dados reais concentram-se em:

1. **Cliente:** a lista/documento de cliente não entrega todos os dados exigidos pelos cards e filtros. O editor também não consegue persistir padrões comerciais, última visita e observação do usuário.
2. **Produto em pedido:** ao adicionar um produto, o frontend preenche peso, cubagem, dimensões, embalagem, estoque e permissão de venda sem estoque com valores artificiais. O contrato de catálogo não contém a maioria desses campos.
3. **Pedido:** há campos de item editáveis que não são enviados ao salvar; o detalhe documentado não define IDs de endereço e propriedades dos catálogos com a precisão que o frontend precisa para recarregar o editor.
4. **Consistência de contrato:** o inventário fornecido descreve criação de pedido via `POST`, enquanto o OpenAPI define `PUT /pedidos/{id}` e o frontend o utiliza para edição. A implementação real precisa ser confirmada.

Este relatório é uma análise estática do frontend, do [OpenAPI](./openapi.yaml) e do inventário fornecido. Não foi feita chamada ao backend nem validação contra banco real; portanto, os pontos abaixo descrevem lacunas comprováveis no contrato/código e itens que precisam de confirmação operacional.

## Cobertura atual por tela

| Área | Situação observada |
|---|---|
| Login | Usa `POST /auth/login`; o mock de autenticação não é selecionado pelo fluxo ativo. |
| Shell | Empresa/usuário e contador de notificações vêm de `/shell/empresa` e `/notificacoes/nao-lidas/count`. |
| Home | Indicadores e avisos vêm de `/home/indicadores` e `/notificacoes`. |
| Cliente | Lista, cadastro, endereços, títulos, consulta CNPJ/CEP e catálogos usam endpoints. As lacunas de campos estão detalhadas abaixo. |
| Pedido/orçamento | Lista, detalhe, gravação e catálogos usam endpoints. Há valores de produto artificiais e dados que não fazem round-trip no editor. |
| Agenda, tarefas e notificações | As rotas atuais do frontend exibem páginas placeholder, não dados mockados. O OpenAPI já contém endpoints de agenda/tarefa e consulta de notificações; trata-se de integração de frontend ainda não feita, não de endpoint ausente. |
| Dados mock mantidos | `MockAuthRepository`, `MockShellRepository`, `MockHomeRepository`, `MockClienteRepository`, `MockPedidoRepository` e `src/mocks/**/*.json` permanecem no repositório. São código de mock não selecionado pelas telas ativas. A pasta `src/features/pedido (copy)` é uma cópia antiga não importada pelo roteador ativo. |

## Especificações necessárias para o backend

### 1. Completar o modelo de cliente — prioridade alta

As abas “Padrões” e “Outros” expõem valores que hoje não são enviados no payload de cliente e não são devolvidos pelo detalhe:

| Campo da tela | Estado atual | Campo sugerido no contrato |
|---|---|---|
| Transportadora padrão | Catálogo carrega; valor do cliente é sempre `null` | `transportadora_padrao_id: integer \| null` |
| Condição de pagamento padrão | Catálogo carrega; valor do cliente é sempre `null` | `condicao_pagamento_padrao_id: integer \| null` |
| Forma de pagamento padrão | Catálogo carrega; valor do cliente é sempre `null` | `forma_pagamento_padrao_id: integer \| null` |
| Tabela de preço padrão | Catálogo carrega; valor do cliente é sempre `null` | `tabela_preco_padrao_id: integer \| null` |
| Data da última visita | Campo inicia vazio e não é persistido | `data_ultima_visita: string \| null` (ISO 8601) |
| Observações (usuário) | Campo inicia vazio e não é persistido; não confundir com anotação ERP | `anotacao_usuario: string` |

**Implementação pedida:** incluir esses campos na entrada de `POST /clientes` e `PUT /clientes/{id}`, persistir em colunas próprias (ou relação equivalente), e devolvê-los em `GET /clientes/{id}` e nas respostas de criação/atualização. `anotacao_erp` continua separado e não deve ser sobrescrito por `anotacao_usuario`.

O campo `codigo_erp`/ID de integração já existe na resposta e é somente leitura no frontend. O inventário informa que o código criado pela API pode ser provisório (`API-*`); definir se e quando ele será reconciliado com o código ERP real.

### 2. Completar listagem, filtros e status de cliente — prioridade alta

O schema `Cliente` usado por `GET /clientes` documenta apenas ID, código ERP, nome/fantasia, CPF/CNPJ, tipo de pessoa, cidade, estado e `ativo`. Os cards e filtros do frontend também precisam dos campos abaixo, que existem no mock antigo ou são esperados pelo mapper atual:

- `prospect: boolean`;
- `situacao_cadastro` e `situacao_compra`;
- data de inclusão/abertura, endereço resumido e endereço completo;
- último pedido e data/último atendimento;
- situação de bloqueio, distinta de `ativo`;
- indicador de título vencido (ou filtro de vencidos calculado no servidor).

**Contrato recomendado:**

1. Ampliar o item da página de `GET /clientes` para entregar esses dados de forma documentada. Para datas, padronizar `data_inclusao` (já usado em `ClienteDetalhe`) em vez de `data_abertura`, ou acordar um único nome e atualizar o frontend.
2. Definir formalmente os enums e semânticas de `situacao_cadastro`, `situacao_compra` e bloqueio. `ativo: boolean` sozinho não distingue cliente bloqueado; `prospect` deve permanecer um atributo separado do estado ativo/inativo.
3. Documentar e retornar `ultimo_pedido` e `ultimo_atendimento` com tipos e semântica definidos (número/data e evento considerado). O filtro “Sem Followup (+30d)” precisa permitir identificar atendimento anterior a 30 dias; apenas `null` não cobre essa regra.
4. Para “Vencidos”, fornecer `possui_titulo_vencido: boolean` na listagem ou suportar filtro de servidor. A tela atualmente aplica o filtro localmente a `titles`, mas os itens da listagem recebem `titles: []`; o endpoint de títulos existe apenas por cliente e não é carregado para cada registro da lista.
5. Ajustar os valores aceitos por `GET /clientes?status=...` para corresponder aos estados definidos, incluindo bloqueado se esse estado existir no ERP. Hoje o OpenAPI enumera `ativo`, `inativo`, `prospect`, sem `bloqueado`.

`endereco_resumo`/`endereco_completo` podem ser campos derivados do endereço principal, sem exigir cópia redundante no banco, desde que retornem conteúdo útil para os cards. Hoje o frontend só consegue derivar cidade/UF dos campos documentados; o endereço completo depende de uma propriedade não documentada.

### 3. Expor os dados completos de produto usados no pedido — prioridade alta

O catálogo `/produtos` atualmente documenta dados básicos e `stock`. A tela de edição do item também apresenta dados logísticos e usa esses valores para calcular totais. Ao inserir um produto novo, o frontend atualmente preenche:

- pesos líquido/bruto = `0`;
- cubagem e dimensões = `0`;
- quantidade por embalagem = `1`;
- permite venda sem estoque = `true`;
- estoque = `0`.

Esses valores são defaults sintéticos em `createOrderItem`, não dados reais. Incluir no schema de `GET /produtos` e no produto embutido em `GET /pedidos/{id}`:

```text
peso_liquido: number
peso_bruto: number
cubagem: number
comprimento: number
largura: number
altura_espessura: number
qtde_embalagem: number
permite_venda_sem_estoque: boolean
stock: number
```

Definir claramente unidade de medida para dimensões/cubagem, origem do estoque (por empresa/depósito) e regra de permissão para venda sem estoque. O campo `stock` já aparece no schema de `/produtos`, mas o frontend atual não o lê; isso exige também ajuste do frontend, não uma nova coluna/endpoint no backend.

**Preço e tabela:** `Produto.tabela_preco_padrao_id` está marcado como TODO no OpenAPI, e `TabelaPreco.vigencia` também tem derivação pendente. Definir a origem do preço e da tabela efetiva para o cliente/empresa, e retornar tabela, vigência e preço coerentes. Se o preço depender do cliente ou da tabela escolhida, documentar os parâmetros necessários (por exemplo, `cliente_id` e/ou `tabela_preco_id`) e validar o preço no servidor. O frontend não deve usar ID `0` ou preço genérico como substituto de dados ausentes.

### 4. Garantir round-trip dos itens e endereços de pedido — prioridade alta

O editor permite alterar tabela de preço e código do produto no cliente, mas o payload atual enviado a `POST/PUT /pedidos` leva somente produto, quantidade, preço e desconto. Os campos `priceTableId` e `customerCode` se perdem ao salvar; `PedidoItemCreate` não documenta o ID da tabela, e `ItemPedido` não documenta `codigo_cliente`.

**Alterar o contrato do item de pedido:**

- aceitar `price_table_id` no item de entrada e persistir a associação selecionada;
- aceitar `codigo_cliente` se o ERP suportar código do produto específico por cliente;
- retornar ambos no detalhe; retornar também nome/vigência da tabela resolvidos pelo servidor;
- validar produto, tabela, cliente/empresa e preço em uma mesma transação.

Além disso, `PedidoCreate` já aceita `endereco_faturamento_id`, `endereco_entrega_id` e `endereco_cobranca_id`, mas `PedidoDetalhe` não declara esses campos no schema. O frontend espera os três IDs para reabrir o pedido. Declarar e retornar os IDs como inteiros anuláveis no detalhe, preservando-os em `PUT`.

O schema também tipa `condicao_pagamento`, `forma_pagamento` e `transportadora` apenas como objeto anulável sem propriedades. O frontend precisa de `id` e descrição/nome para recuperar as seleções. Definir schemas explícitos:

```text
condicao_pagamento: { id, descricao } | null
forma_pagamento: { id, descricao } | null
transportadora: { id, nome_razao } | null
```

**Atualização:** o OpenAPI documenta `PUT /pedidos/{id}`, e o repositório ativo do frontend o utiliza ao editar. Já o inventário fornecido descreve `POST /pedidos` como operação de criação que sempre insere e não atualiza. Confirmar que `PUT` está implementado no backend com substituição transacional de itens e que o detalhe retornado mantém todos os campos acima; se não estiver, implementar e testar essa operação ou alinhar o frontend/contrato para não apresentar edição persistente.

## Pontos que são correções do frontend, não novas entregas de backend

- `GET /produtos` já documenta `stock`; o repositório do frontend não o mapeia.
- O frontend envia `data_entrega`, frete, pagamentos, transportadora, endereços, observações e itens, mas não envia atualmente `empresa_id`, `status` e `data_emissao`, embora o contrato de criação os aceite. Definir o comportamento desejado e ajustar o payload frontend; não criar campos duplicados no backend.
- O mapper de cliente espera `data_abertura`, enquanto o OpenAPI do detalhe declara `data_inclusao`; alinhar nome no contrato ou corrigir o mapper para usar o campo existente.
- O frontend converte `ativo` em `ativo/inativo`, mas os filtros locais comparam com `aberto` e `bloqueado`. Corrigir o mapeamento para os enums definidos pelo backend.
- Depois da disponibilização dos campos logísticos e de preço, remover os defaults artificiais de `createOrderItem` e carregar o produto/tabela selecionados com os valores efetivos.
- Os endpoints de agenda, tarefa e notificações já constam na documentação; as páginas correspondentes ainda são placeholders e precisam ser conectadas pelo frontend.

## Critérios de aceite sugeridos para o backend

1. Criar e atualizar cliente com os quatro padrões comerciais, última visita e observação do usuário; recarregar via `GET /clientes/{id}` e confirmar igualdade dos valores, sem mistura entre observação ERP e observação do usuário.
2. Listar clientes com os campos de card/status; validar filtros de prospect, bloqueado, follow-up com mais de 30 dias e títulos vencidos em paginação.
3. Listar produto com todos os dados logísticos e preço/tabela efetivos; conferir mesma informação dentro do detalhe do pedido.
4. Criar pedido com tabela de preço e código do produto no cliente; atualizar o mesmo pedido; reler o detalhe e confirmar persistência de itens, seleções e IDs de endereço sem duplicar o pedido.
5. Cobrir esses fluxos com testes de integração handler → service → PostgreSQL, incluindo tenant/empresa/usuário do token. Os testes de contrato do frontend não substituem essa validação de persistência.
