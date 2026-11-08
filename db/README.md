# Banco de Dados — VisionFlow API

Documentação técnica da estrutura de banco, migrations e boas práticas.

---

## 📋 Sumário

- [Visão geral](#visão-geral)
- [Estrutura de pastas](#estrutura-de-pastas)
- [Como usar o Flyway](#como-usar-o-flyway)
- [Como criar uma nova migration](#como-criar-uma-nova-migration)
- [Como resetar o banco do zero](#como-resetar-o-banco-do-zero)
- [Backup](#backup)
- [Convenções do schema](#convenções-do-schema)
- [Troubleshooting](#troubleshooting)

---

## Visão geral

O projeto usa **PostgreSQL 16** com:

- **Multi-tenant** — isolamento por `cod_tenant` e `cod_empresa`
- **Row-Level Security (RLS)** — filtragem automática de dados por tenant
- **White label** — suporta revendedores com marca própria (`revendedor`)
- **Flyway** — controle de versão do schema

**Migrations no repositório:** V001 a V019. A V019 cria o papel PostgreSQL
`visionflow_ia` para consultas da feature de relatórios com IA, sem privilégios de
superusuário nem `BYPASSRLS`. Consulte `flyway info` e valide o ambiente antes de
aplicar migrations; não execute migrations em produção sem a janela de mudança aprovada.

---

## Estrutura de pastas

```
db/
├── README.md                # Este arquivo
├── flyway.conf              # Config do Flyway (NÃO commitar — tem senha)
├── migrations/              # Migrations versionadas (ordem importa)
│   ├── V001__nucleo_saas.sql
│   ├── V002__cadastros_base.sql
│   ├── V003__cadastros_operacionais.sql
│   ├── V004__tabelas_filho.sql
│   ├── V005__rls.sql
│   ├── V006__indices_unicos_seeds.sql
│   ├── V007__pedido_situacao_app.sql
│   ├── V008__notificacao.sql
│   ├── V009__agenda_tarefa.sql
│   ├── V010__empresa_regional.sql
│   ├── V011__pgcrypto_password_validation.sql
│   ├── V012__fix_demo_admin_password_hash.sql
│   ├── V013__soft_delete.sql
│   ├── V014__fix_saajoao_seed_passwords.sql
│   ├── V015__pedido_data_entrega.sql
│   ├── V016__api_editor_campos_complementares.sql
│   ├── V017__indices_busca_textual.sql
│   ├── V018__integracao_erp.sql
│   └── V019__visionflow_ia_role.sql
├── seeds/                   # Dados de teste (não vão para produção)
├── scripts/                 # Automação
│   ├── reset_db.sh
│   └── backup.sh
└── backups/                 # Backups gerados (no .gitignore)
```

### Configurar a senha da role de relatórios com IA

A V019 cria `visionflow_ia` sem senha para não gravar credenciais em migrations.
Depois de aplicar a migration, conecte ao banco como administrador e defina a
senha no prompt interativo do `psql`:

```sql
\password visionflow_ia
```

Configure a mesma senha localmente em `DB_IA_PASSWORD`. Não coloque a senha em
migrations, no `.env.example` ou em commits. A role recebe `pg_read_all_data`,
mas não tem `BYPASSRLS`; as políticas RLS continuam sendo aplicadas às tabelas
protegidas.

---

## Como usar o Flyway

### Pré-requisitos

O Flyway está em `tools/flyway-13.7.0/` e linkado globalmente em
`/usr/local/bin/flyway`.

**Se não estiver instalado em outra máquina:**

```bash
cd tools
wget -qO- "https://download.red-gate.com/maven/release/com/redgate/flyway/flyway-commandline/13.7.0/flyway-commandline-13.7.0-linux-x64.tar.gz" | tar -xvz
sudo ln -sf "$(pwd)/flyway-13.7.0/flyway" /usr/local/bin/flyway
```

### Comandos principais

**Ver status (sem aplicar nada):**

```bash
flyway -configFiles=./db/flyway.conf info
```

**Aplicar migrations pendentes:**

```bash
flyway -configFiles=./db/flyway.conf migrate
```

**Validar (só verifica integridade, não aplica):**

```bash
flyway -configFiles=./db/flyway.conf validate
```

**Marcar o banco em uma versão (usado uma vez):**

```bash
flyway -configFiles=./db/flyway.conf baseline -baselineVersion=5
```

---

## Como criar uma nova migration

### 1. Nomear corretamente

O padrão é: `V<versão>__<descrição>.sql`

**Regras:**

- Prefixo `V` (maiúsculo)
- **DOIS underscores** (`__`) entre a versão e a descrição
- Descrição em minúsculas com `_` como separador
- Extensão `.sql`

**✅ Exemplos válidos:**

```
V006__adiciona_campo_observacao_cliente.sql
V007__cria_tabela_pedido_comissao.sql
V008__indice_cliente_por_cidade.sql
```

**❌ Exemplos INVÁLIDOS:**

```
V006_adiciona_campo.sql        ← 1 underscore só (não funciona)
v006__adiciona_campo.sql       ← v minúsculo
006__adiciona_campo.sql        ← falta o V
V006__Adiciona Campo.sql       ← tem espaço
```

### 2. Escrever a migration

Exemplo:

```sql
-- ============================================================
-- V006 — Adiciona campo observacao na tabela cliente
-- ============================================================

ALTER TABLE cliente ADD COLUMN observacao TEXT;
COMMENT ON COLUMN cliente.observacao IS 'Observação livre do vendedor';
```

### 3. Aplicar

```bash
flyway -configFiles=./db/flyway.conf migrate
```

### 4. Confirmar

```bash
flyway -configFiles=./db/flyway.conf info
```

A nova migration aparece como **Success** (não mais Pending).

---

## Como resetar o banco do zero

Se precisar recriar tudo (nova máquina, ambiente de teste):

```bash
./db/scripts/reset_db.sh
```

Esse script:

1. Dropa o banco (se existir)
2. Cria novamente
3. Ajusta permissões do schema
4. Aplica todas as migrations via Flyway

> ⚠️ **ATENÇÃO:** apaga **todos os dados**. Use só em desenvolvimento.

---

## Backup

### Backup manual

```bash
./db/scripts/backup.sh
```

Gera um arquivo `.sql.gz` em `db/backups/` e retém os últimos 30 dias.

### Backup automático

Já configurado no `crontab` para rodar **todo dia às 3h**.

Para conferir:

```bash
crontab -l | grep backup
```

### Configuração `.pgpass` (para backup não pedir senha)

O arquivo `~/.pgpass` guarda as credenciais de forma segura:

```
localhost:5432:epare_forca_de_vendas:epare:<senha>
```

Permissão obrigatória (senão o PostgreSQL ignora):

```bash
chmod 600 ~/.pgpass
```

---

## Convenções do schema

### Nomenclatura de tabelas

- **snake_case** (minúsculas com `_`)
- **singular** (`cliente`, `pedido`, não `clientes`, `pedidos`)
- Tabelas filhas: `<pai>_<filho>` (ex: `cliente_representante`)
- Tabelas de junção N:N: `<tabela1>_<tabela2>`

### Colunas

| Coluna | Tipo | Propósito |
|--------|------|-----------|
| `cod_tenant` | `INTEGER NOT NULL` | Isolamento por cliente SaaS |
| `cod_empresa` | `INTEGER NOT NULL` | Isolamento por CNPJ |
| `codigo` | `SERIAL` | ID interno (nunca muda) |
| `codigo_erp` | `VARCHAR(20) NOT NULL` | Código no ERP do cliente |
| `ativo` | `BOOLEAN DEFAULT TRUE` | `NULL` = usa padrão |
| `dt_inclusao` | `TIMESTAMPTZ` | Quando foi criado |
| `dt_alteracao` | `TIMESTAMPTZ` | Quando foi alterado (trigger) |

### Chaves primárias

Sempre **compostas**: `(cod_tenant, cod_empresa, codigo)`.

### Chaves únicas

Sempre **compostas com o código ERP**: `(cod_tenant, cod_empresa, codigo_erp)`.

### Foreign keys

Apontam para `empresa (cod_tenant, cod_empresa)` ou para a tabela pai.

### Triggers

Toda tabela tem trigger `tg_<tabela>_dt_alteracao` que atualiza
`dt_alteracao` automaticamente em UPDATEs.

### RLS

Todas as tabelas operacionais têm RLS ativo com policy
`tenant_isolation`. Apenas `revendedor`, `tenant`, `empresa`, `usuario`,
`usuario_empresa` e `log_auditoria` ficam fora (para autenticação e
controle).

---

## Troubleshooting

### Erro: "Found more than one migration with version X"

**Causa:** dois arquivos têm a mesma versão (ex: `V001__a.sql` e `V001__b.sql`).

**Solução:** renomeie um deles para a próxima versão disponível.

### Erro: "Detected resolved migration not applied to database"

**Causa:** você editou uma migration que já foi aplicada.

**Solução:** as opções são:

1. Reverter a edição (não mexa em migration já aplicada)
2. Criar uma **nova migration** (`V006__...`) que faça a correção
3. Em desenvolvimento: apagar o banco e recriar (`reset_db.sh`)

### Erro: "no pg_hba.conf entry for host"

**Causa:** o PostgreSQL está bloqueando conexão via TCP.

**Solução:** edite `/etc/postgresql/16/main/pg_hba.conf` e adicione:

```
host    all    all    127.0.0.1/32    scram-sha-256
```

Depois: `sudo systemctl restart postgresql`.

### Erro: "connection refused"

**Causa:** PostgreSQL parado.

**Solução:**

```bash
sudo systemctl status postgresql
sudo systemctl start postgresql
```

### RLS não filtra

**Causa:** você está conectado como superuser (`postgres`), que ignora RLS.

**Solução:** conecte com o usuário da API (`epare`).

### "You are not signed in to Flyway"

**Causa:** mensagem da Community Edition. Não é erro.

**Solução:** ignorar. O Flyway funciona sem login.

---

## Referências

- [Documentação oficial do Flyway](https://documentation.red-gate.com/flyway)
- [PostgreSQL RLS](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)
- [Manual de infraestrutura completo](../docs/infraestrutura/visionflow-api-postgresql-infraestrutura.html)


---

## Seeds — Dados de Demonstração

Dados de teste organizados por **cenário de uso**. Cada cenário fica isolado
em seu próprio **tenant**, então convivem na mesma base sem interferência
(graças ao RLS).

### Cenários disponíveis

| Cenário | Tenant | Nicho | Objetivo |
|---------|--------|-------|----------|
| **Demo Comercial** | Distribuidora São João | Distribuidora de bebidas | Demonstração comercial, desenvolvimento inicial |

### Estrutura

```
db/seeds/
├── README.md                    # Este documento (visão geral)
└── demo_comercial/
    ├── apply_all.sh             # Roda todos os seeds em ordem
    ├── 001_base.sql             # Revendedor + Tenant + Empresas + Usuários
    ├── 002_cadastros.sql        # 18 tabelas de cadastros base
    ├── 003_clientes.sql         # 30 clientes realistas
    ├── 004_produtos.sql         # 50 produtos + estoque + 150 preços
    └── 005_pedidos.sql          # 40 pedidos + 141 itens + 28 históricos
```

---

## 🎬 Cenário: Demo Comercial

**Tenant:** Distribuidora São João
**Nicho:** Distribuidora de bebidas
**Empresas:** 2 (Matriz SP + Filial Campinas)

### O que é criado

| Entidade | Quantidade | Detalhes |
|----------|-----------|----------|
| Revendedor | 1 | VisionFlow Sistemas |
| Tenant | 1 | Distribuidora São João LTDA |
| Empresas | 2 | Matriz SP (Av. Paulista) + Filial Campinas |
| Usuários | 4 | 1 admin + 3 vendedores |
| Vínculos usuário↔empresa | 5 | Admin vê tudo, vendedores veem a empresa deles |
| Clientes | 30 | 10 premium + 11 médios + 9 pequenos |
| Produtos | 50 | 35 bebidas + 15 alimentos |
| Estoque | 50 | Registros de estoque por unidade de negócio |
| Preços | 150 | 50 produtos × 3 tabelas (Varejo/Atacado/Promocional) |
| Pedidos | 40 | 9 abertos + 10 faturados + 6 entregues + 6 cancelados |
| Itens de pedido | 141 | Média de 3,5 itens/pedido |
| Históricos | 28 | Faturamento, entrega, cancelamento |

### Usuários para login

| Email | Senha | Tipo | Empresas |
|-------|-------|------|----------|
| `admin@saajoao.com.br` | `visionflow123` | admin | Matriz + Campinas |
| `joao.silva@saajoao.com.br` | `visionflow123` | vendedor | Matriz |
| `ana.costa@saajoao.com.br` | `visionflow123` | vendedor | Matriz |
| `carlos.oliveira@saajoao.com.br` | `visionflow123` | vendedor | Campinas |

> ⚠️ **Senha apenas para desenvolvimento.** Em produção, trocar via
> `ALTER USER ... WITH PASSWORD` ou reset via API.

### Como aplicar

**Opção 1 — Script único (recomendado):**

```bash
cd ~/Documents/VisionFlow_1/visionflow-api
./db/seeds/demo_comercial/apply_all.sh
```

**Opção 2 — Arquivo por arquivo (para debug):**

```bash
cd ~/Documents/VisionFlow_1/visionflow-api

for f in 001_base 002_cadastros 003_clientes 004_produtos 005_pedidos; do
    echo "Aplicando $f..."
    psql -h localhost -U epare -d epare_forca_de_vendas \
        -f "db/seeds/demo_comercial/${f}.sql"
done
```

### Como consultar (respeitando RLS)

**No psql:**

```sql
BEGIN;
SET LOCAL app.cod_tenant = '4';      -- Distribuidora São João
SET LOCAL app.cod_empresa = '1';     -- Matriz SP

SELECT COUNT(*) FROM cliente;         -- 30
SELECT COUNT(*) FROM item;            -- 50
SELECT COUNT(*) FROM pedido;          -- 40

COMMIT;
```

**No pgAdmin:**

1. Conecte como `epare`
2. Abra o **Query Tool**
3. Cole o bloco acima
4. Execute (F5)

> 💡 **Descobrir o `cod_tenant`:** se você já rodou os seeds mas não
> lembra o ID:
> ```sql
> SELECT cod_tenant, slug, nome FROM tenant WHERE slug = 'distribuidora-sao-joao';
> ```

### Como remover o cenário

Se quiser **apagar tudo** do demo (mantendo outros tenants):

```sql
BEGIN;
SET LOCAL app.cod_tenant = '4';

-- Isso apaga em cascata: empresa, cliente, item, pedido, tudo
DELETE FROM tenant WHERE slug = 'distribuidora-sao-joao';

COMMIT;
```

Confirma que foi removido:

```sql
SELECT cod_tenant, slug, nome FROM tenant ORDER BY cod_tenant;
```

---

## 🧠 Boas práticas para criar novos seeds

### 1. Sempre defina o contexto RLS antes dos INSERTs

**Sem isso, os INSERTs falham com:**

```
ERROR: new row violates row-level security policy for table "X"
```

**Solução:**

```sql
DO $$
DECLARE v_tenant INTEGER;
BEGIN
    SELECT cod_tenant INTO v_tenant FROM tenant WHERE slug = 'meu-tenant';

    -- ⚠️ Define contexto RLS
    PERFORM set_config('app.cod_tenant', v_tenant::TEXT, true);
    PERFORM set_config('app.cod_empresa', '1', true);

    -- Agora pode inserir
    INSERT INTO ...
END $$;
```

### 2. Sempre defina o contexto RLS antes dos SELECTs de verificação

O bloco de verificação **também** precisa do contexto. Se não tiver,
o `SELECT COUNT(*)` retorna **0** mesmo com dados presentes.

### 3. Use `WHERE NOT EXISTS` em vez de `ON CONFLICT`

`ON CONFLICT (coluna)` só funciona com **índice único total**. Nossos
índices únicos são **parciais** (`WHERE coluna IS NOT NULL`), então o
Postgres não consegue casar. Use:

```sql
INSERT INTO tabela (...)
SELECT ...
WHERE NOT EXISTS (
    SELECT 1 FROM tabela WHERE ... = ...
);
```

### 4. Prefira prefixos de código ERP reconhecíveis

| Padrão | Uso |
|--------|-----|
| `000001` a `000030` | Clientes |
| `BEB001`, `PET001` | Produtos (bebidas, petiscos) |
| `PED00001` | Pedidos |
| `F001` a `F006` | Fabricantes |
| `G1`, `S1` | Categorias (grupo, subgrupo) |
| `R01`, `R02` | Ramos de atividade |

Isso facilita identificar dados de teste com um simples `WHERE codigo_erp LIKE 'BEB%'`.

### 5. Mantenha um `apply_all.sh` por cenário

Assim, aplicar tudo é um comando só:

```bash
./db/seeds/demo_comercial/apply_all.sh
```

### 6. Documente o cenário no topo de cada arquivo

Exemplo de header:

```sql
-- ============================================================
-- SEED — Demo Comercial — Clientes
-- ============================================================
-- Popula: 30 clientes (bares, restaurantes, mercados)
-- Idempotente: WHERE NOT EXISTS
-- Tenant: distribuidora-sao-joao
-- ============================================================
```

---

## 🔮 Próximos cenários sugeridos

| Cenário | Tenant | Objetivo |
|---------|--------|----------|
| **Dev** | dev-teste | Casos-limite: campos NULL, strings longas, caracteres especiais |
| **Performance** | perf-teste | Volume grande: 1000+ clientes, 10000+ pedidos |
| **Multi-tenant** | tenant-a, tenant-b | Testar isolamento entre tenants |

Cada novo cenário segue o mesmo padrão:

```
db/seeds/
├── dev/
│   ├── apply_all.sh
│   └── 001_*.sql
├── performance/
│   ├── apply_all.sh
│   └── 001_*.sql
└── multi_tenant/
    ├── apply_all.sh
    └── 001_*.sql
```

---

## ⚠️ Nunca aplique seeds em produção

Estes dados são para **desenvolvimento e demonstração**. Em produção:

- ❌ Não rode `apply_all.sh`
- ❌ Não commite dados de demo
- ✅ Mantenha apenas as migrations (`db/migrations/`)
- ✅ Se precisar de dados mínimos em produção (ex: unidades de medida padrão), crie uma migration separada

---

## Referências

- [Estrutura das migrations](../migrations/)
- [Script de backup](../scripts/backup.sh)
- [Script de reset](../scripts/reset_db.sh)