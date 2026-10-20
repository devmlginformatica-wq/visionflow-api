-- Estado de integração para clientes, pedidos e orçamentos.
-- Orçamentos continuam armazenados em pedido, diferenciados por e_um_orcamento.

ALTER TABLE public.pedido
    ALTER COLUMN codigo_erp DROP NOT NULL,
    ALTER COLUMN codigo_erp_cliente DROP NOT NULL,
    ADD COLUMN cliente_id INTEGER,
    ADD COLUMN motivo_rejeicao TEXT,
    ADD COLUMN endereco_faturamento_local_id INTEGER,
    ADD COLUMN endereco_entrega_local_id INTEGER,
    ADD COLUMN endereco_cobranca_local_id INTEGER;

ALTER TABLE public.pedido
    DROP CONSTRAINT ck_pedido_situacao_app,
    ADD CONSTRAINT ck_pedido_situacao_app CHECK (situacao_app IN (
        'em_digitacao',
        'em_integracao',
        'integrado',
        'erro_integracao',
        'aguardando_aprovacao',
        'aprovado',
        'cancelado'
    ));

ALTER TABLE public.cliente
    ALTER COLUMN codigo_erp DROP NOT NULL,
    ADD COLUMN situacao_app VARCHAR(20) NOT NULL DEFAULT 'em_digitacao',
    ADD COLUMN motivo_rejeicao TEXT;

ALTER TABLE public.cliente
    ADD CONSTRAINT ck_cliente_situacao_app CHECK (situacao_app IN (
        'em_digitacao',
        'em_integracao',
        'integrado',
        'erro_integracao'
    ));

-- Cadastros já carregados do ERP permanecem identificados como integrados.
UPDATE public.cliente
SET situacao_app = 'integrado'
WHERE codigo_erp IS NOT NULL
  AND codigo_erp !~ '^API-[0-9]+$';

-- Preserva relações locais antes de retirar os códigos provisórios gerados pela API.
UPDATE public.pedido p
SET cliente_id = c.codigo
FROM public.cliente c
WHERE p.cod_tenant = c.cod_tenant
  AND p.cod_empresa = c.cod_empresa
  AND p.codigo_erp_cliente = c.codigo_erp;

ALTER TABLE public.cliente_endereco_adicional
    ALTER COLUMN codigo_erp_cliente DROP NOT NULL,
    ALTER COLUMN codigo_erp_endereco DROP NOT NULL,
    ADD COLUMN cliente_id INTEGER;

UPDATE public.cliente_endereco_adicional e
SET cliente_id = c.codigo
FROM public.cliente c
WHERE e.cod_tenant = c.cod_tenant
  AND e.cod_empresa = c.cod_empresa
  AND e.codigo_erp_cliente = c.codigo_erp;

UPDATE public.pedido p
SET endereco_faturamento_local_id = e.codigo
FROM public.cliente_endereco_adicional e
WHERE p.cod_tenant=e.cod_tenant AND p.cod_empresa=e.cod_empresa
  AND p.cliente_id=e.cliente_id AND p.endereco_ident_faturamento IS NOT NULL
  AND p.endereco_ident_faturamento IN (e.identificacao_endereco,e.codigo_erp_endereco);

UPDATE public.pedido p
SET endereco_entrega_local_id = e.codigo
FROM public.cliente_endereco_adicional e
WHERE p.cod_tenant=e.cod_tenant AND p.cod_empresa=e.cod_empresa
  AND p.cliente_id=e.cliente_id AND p.endereco_ident_entrega IS NOT NULL
  AND p.endereco_ident_entrega IN (e.identificacao_endereco,e.codigo_erp_endereco);

UPDATE public.pedido p
SET endereco_cobranca_local_id = e.codigo
FROM public.cliente_endereco_adicional e
WHERE p.cod_tenant=e.cod_tenant AND p.cod_empresa=e.cod_empresa
  AND p.cliente_id=e.cliente_id AND p.endereco_ident_cobranca IS NOT NULL
  AND p.endereco_ident_cobranca IN (e.identificacao_endereco,e.codigo_erp_endereco);

ALTER TABLE public.pedido
    ADD CONSTRAINT fk_pedido_cliente_local
    FOREIGN KEY (cod_tenant, cod_empresa, cliente_id)
    REFERENCES public.cliente (cod_tenant, cod_empresa, codigo);

ALTER TABLE public.cliente_endereco_adicional
    ADD CONSTRAINT fk_cli_end_cliente_local
    FOREIGN KEY (cod_tenant, cod_empresa, cliente_id)
    REFERENCES public.cliente (cod_tenant, cod_empresa, codigo);

ALTER TABLE public.pedido
    ADD CONSTRAINT fk_pedido_endereco_faturamento_local
        FOREIGN KEY (cod_tenant, cod_empresa, endereco_faturamento_local_id)
        REFERENCES public.cliente_endereco_adicional (cod_tenant, cod_empresa, codigo),
    ADD CONSTRAINT fk_pedido_endereco_entrega_local
        FOREIGN KEY (cod_tenant, cod_empresa, endereco_entrega_local_id)
        REFERENCES public.cliente_endereco_adicional (cod_tenant, cod_empresa, codigo),
    ADD CONSTRAINT fk_pedido_endereco_cobranca_local
        FOREIGN KEY (cod_tenant, cod_empresa, endereco_cobranca_local_id)
        REFERENCES public.cliente_endereco_adicional (cod_tenant, cod_empresa, codigo);

CREATE INDEX idx_pedido_cliente_local
    ON public.pedido (cod_tenant, cod_empresa, cliente_id);

CREATE INDEX idx_cli_end_cliente_local
    ON public.cliente_endereco_adicional (cod_tenant, cod_empresa, cliente_id)
    WHERE deletado_em IS NULL;

UPDATE public.cliente_endereco_adicional
SET codigo_erp_cliente = NULL,
    codigo_erp_endereco = NULL,
    identificacao_endereco = NULL
WHERE codigo_erp_cliente ~ '^API-[0-9]+$'
   OR codigo_erp_endereco ~ '^API-[0-9]+$';

ALTER TABLE public.pedido_item
    ALTER COLUMN codigo_erp_pedido DROP NOT NULL,
    ALTER COLUMN codigo_erp_item_pedido DROP NOT NULL,
    ADD COLUMN pedido_id INTEGER;

UPDATE public.pedido_item pi
SET pedido_id = p.codigo
FROM public.pedido p
WHERE pi.cod_tenant = p.cod_tenant
  AND pi.cod_empresa = p.cod_empresa
  AND pi.codigo_erp_pedido = p.codigo_erp;

ALTER TABLE public.pedido_item
    ADD CONSTRAINT fk_pedido_item_pedido_local
    FOREIGN KEY (cod_tenant, cod_empresa, pedido_id)
    REFERENCES public.pedido (cod_tenant, cod_empresa, codigo);

CREATE INDEX idx_pedido_item_pedido_local
    ON public.pedido_item (cod_tenant, cod_empresa, pedido_id);

UPDATE public.pedido_item
SET nosso_codigo_item_pedido = COALESCE(nosso_codigo_item_pedido, codigo_erp_item_pedido),
    codigo_erp_pedido = NULL,
    codigo_erp_item_pedido = NULL
WHERE codigo_erp_pedido ~ '^API-[0-9]+$';

UPDATE public.pedido
SET codigo_erp_cliente = NULL
WHERE codigo_erp_cliente ~ '^API-[0-9]+$';

UPDATE public.cliente
SET codigo_erp = NULL,
    situacao_app = 'em_digitacao'
WHERE codigo_erp ~ '^API-[0-9]+$';

UPDATE public.pedido
SET codigo_erp = NULL,
    situacao_app = 'em_digitacao'
WHERE codigo_erp ~ '^API-[0-9]+$';

CREATE TABLE public.integracao_fila (
    id BIGSERIAL PRIMARY KEY,
    cod_tenant INTEGER NOT NULL,
    cod_empresa INTEGER NOT NULL,
    entidade VARCHAR(20) NOT NULL CHECK (entidade IN ('pedido', 'orcamento', 'cliente')),
    entidade_id INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pendente', 'sucesso', 'erro')),
    tentativa INTEGER NOT NULL CHECK (tentativa > 0),
    codigo_erp_retorno VARCHAR(20),
    motivo_rejeicao TEXT,
    data_envio TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    data_retorno TIMESTAMPTZ,
    CONSTRAINT fk_integracao_fila_empresa
        FOREIGN KEY (cod_tenant, cod_empresa)
        REFERENCES public.empresa (cod_tenant, cod_empresa) ON DELETE CASCADE
);

CREATE INDEX idx_integracao_fila_pendentes
    ON public.integracao_fila (cod_tenant, cod_empresa, entidade, data_envio, id)
    WHERE status = 'pendente';

CREATE UNIQUE INDEX uk_integracao_fila_ativa
    ON public.integracao_fila (cod_tenant, cod_empresa, entidade, entidade_id)
    WHERE status = 'pendente';

CREATE INDEX idx_integracao_fila_entidade
    ON public.integracao_fila (cod_tenant, cod_empresa, entidade, entidade_id, id DESC);

ALTER TABLE public.integracao_fila ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.integracao_fila FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON public.integracao_fila
    FOR ALL TO PUBLIC
    USING (
        cod_tenant = fn_current_tenant()
        AND (
            fn_current_empresa() IS NULL
            OR cod_empresa = fn_current_empresa()
        )
    )
    WITH CHECK (
        cod_tenant = fn_current_tenant()
        AND (
            fn_current_empresa() IS NULL
            OR cod_empresa = fn_current_empresa()
        )
    );

COMMENT ON TABLE public.integracao_fila IS
    'Outbox e histórico de tentativas de integração ERP para pedido, orçamento e cliente.';
