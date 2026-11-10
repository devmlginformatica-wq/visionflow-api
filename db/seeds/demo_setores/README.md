# Demos setoriais

Este seed cria três tenants fictícios, cada um com uma empresa:

- **Atacado e distribuição:** Serra Norte Atacado
- **Tecnologia/SaaS:** Nuvem Pioneira Sistemas
- **Saúde e dispositivos médicos:** Horizonte Clínico Equipamentos

Cada tenant recebe um admin, um gerente, dois vendedores, três representantes,
quatro clientes, três produtos e três pedidos com itens. O gerente possui um
representante próprio e os dois representantes vendedores na equipe. Cada
vendedor é vinculado ao seu próprio representante. As quatro contas de usuário
(admin, gerente e vendedores) são criadas ou atualizadas pelo seed.

## Pré-requisitos e execução

As migrations até V021 devem estar aplicadas, e o banco precisa conter o
revendedor com slug `visionflow`. Para executar no banco de desenvolvimento:

```bash
psql -v ON_ERROR_STOP=1 -h localhost -U epare -d epare_forca_de_vendas \
  -f db/seeds/demo_setores/001_empresas_demo.sql
```

O arquivo é idempotente e preserva configurações existentes. A chave
`vendedor_ve_dados_equipe` é criada com valor `false` quando ainda não existe.
Assim, cada vendedor vê inicialmente apenas os dados do próprio representante;
o gerente vê os próprios dados e os da equipe. Ao executar novamente, o seed
redefine os nomes, perfis, status e senhas das quatro contas demo. A execução é
transacional; se houver erro, a transação não deve ser confirmada.

## Contas de demonstração

Todas as contas de usuário (admin, gerente e vendedores) usam a senha
`123456`. O seed armazena somente o hash bcrypt dessa senha e redefine esse hash
a cada execução. Use essas contas apenas em desenvolvimento e demonstração;
troque as senhas antes de qualquer uso fora desse contexto.

| Tenant | Admin | Gerente | Vendedor 1 | Vendedor 2 |
|---|---|---|---|---|
| Atacado | `admin@demo-atacado-serranorte.example.test` | `gerente@demo-atacado-serranorte.example.test` | `vendedor1@demo-atacado-serranorte.example.test` | `vendedor2@demo-atacado-serranorte.example.test` |
| SaaS | `admin@demo-saas-nuvempioneira.example.test` | `gerente@demo-saas-nuvempioneira.example.test` | `vendedor1@demo-saas-nuvempioneira.example.test` | `vendedor2@demo-saas-nuvempioneira.example.test` |
| Saúde | `admin@demo-saude-horizonteclinico.example.test` | `gerente@demo-saude-horizonteclinico.example.test` | `vendedor1@demo-saude-horizonteclinico.example.test` | `vendedor2@demo-saude-horizonteclinico.example.test` |

Os dados são exclusivamente fictícios e destinam-se a desenvolvimento e
demonstração; não use essas contas ou dados em produção.
