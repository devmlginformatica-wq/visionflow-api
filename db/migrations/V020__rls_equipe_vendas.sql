-- Contexto de usuário e controle de acesso por representante/equipe.
-- A aplicação deve definir app.cod_usuario na mesma transação que executa as
-- consultas protegidas por RLS.

BEGIN;

-- Um usuário pode atuar como um representante por empresa. Gerentes que também
-- vendem recebem seu representante nesta tabela e sua equipe na tabela abaixo.
CREATE TABLE public.usuario_representante (
    cod_tenant                  INTEGER     NOT NULL,
    cod_empresa                 INTEGER     NOT NULL,
    cod_usuario                 INTEGER     NOT NULL,
    codigo_erp_representante    VARCHAR(20) NOT NULL,
    dt_inclusao                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_usuario_representante
        PRIMARY KEY (cod_tenant, cod_empresa, cod_usuario),
    CONSTRAINT fk_usuario_representante_usuario_empresa
        FOREIGN KEY (cod_usuario, cod_tenant, cod_empresa)
        REFERENCES public.usuario_empresa (cod_usuario, cod_tenant, cod_empresa)
        ON DELETE CASCADE,
    CONSTRAINT fk_usuario_representante_representante
        FOREIGN KEY (cod_tenant, cod_empresa, codigo_erp_representante)
        REFERENCES public.representante (cod_tenant, cod_empresa, codigo_erp)
        ON DELETE CASCADE
);

CREATE INDEX idx_usuario_representante_representante
    ON public.usuario_representante
       (cod_tenant, cod_empresa, codigo_erp_representante);

COMMENT ON TABLE public.usuario_representante IS
    'Representante ERP associado ao usuário do VisionFlow em cada empresa.';

CREATE TABLE public.equipe_venda_gerente (
    cod_tenant                  INTEGER     NOT NULL,
    cod_empresa                 INTEGER     NOT NULL,
    cod_usuario_gerente         INTEGER     NOT NULL,
    codigo_erp_representante    VARCHAR(20) NOT NULL,
    dt_inclusao                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_equipe_venda_gerente
        PRIMARY KEY
        (cod_tenant, cod_empresa, cod_usuario_gerente, codigo_erp_representante),
    CONSTRAINT fk_equipe_venda_gerente_usuario_empresa
        FOREIGN KEY (cod_usuario_gerente, cod_tenant, cod_empresa)
        REFERENCES public.usuario_empresa (cod_usuario, cod_tenant, cod_empresa)
        ON DELETE CASCADE,
    CONSTRAINT fk_equipe_venda_gerente_representante
        FOREIGN KEY (cod_tenant, cod_empresa, codigo_erp_representante)
        REFERENCES public.representante (cod_tenant, cod_empresa, codigo_erp)
        ON DELETE CASCADE
);

CREATE INDEX idx_equipe_venda_gerente_representante
    ON public.equipe_venda_gerente
       (cod_tenant, cod_empresa, codigo_erp_representante);

COMMENT ON TABLE public.equipe_venda_gerente IS
    'Representantes ERP que compõem a equipe de cada gerente por empresa.';

-- A V018 permite clientes locais sem codigo_erp. Preserva os vínculos antigos
-- pelo código ERP e também permite associar clientes locais pelo ID da tabela.
ALTER TABLE public.cliente_representante
    ADD COLUMN cliente_id INTEGER;

UPDATE public.cliente_representante cr
SET cliente_id = c.codigo
FROM public.cliente c
WHERE cr.cod_tenant = c.cod_tenant
  AND cr.cod_empresa = c.cod_empresa
  AND cr.codigo_erp_cliente = c.codigo_erp
  AND cr.cliente_id IS NULL;

ALTER TABLE public.cliente_representante
    ALTER COLUMN codigo_erp_cliente DROP NOT NULL,
    ADD CONSTRAINT fk_cliente_representante_cliente_local
        FOREIGN KEY (cod_tenant, cod_empresa, cliente_id)
        REFERENCES public.cliente (cod_tenant, cod_empresa, codigo)
        ON DELETE CASCADE,
    ADD CONSTRAINT ck_cliente_representante_cliente_ref
        CHECK (cliente_id IS NOT NULL OR codigo_erp_cliente IS NOT NULL);

CREATE INDEX idx_cliente_representante_cliente_local
    ON public.cliente_representante (cod_tenant, cod_empresa, cliente_id);

CREATE INDEX idx_cliente_representante_cliente_representante
    ON public.cliente_representante
       (cod_tenant, cod_empresa, codigo_erp_cliente, codigo_erp_representante);

-- Estas tabelas de vínculo também ficam limitadas ao contexto tenant/empresa.
ALTER TABLE public.usuario_representante ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.usuario_representante FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON public.usuario_representante;
CREATE POLICY tenant_isolation ON public.usuario_representante
    FOR ALL
    TO PUBLIC
    USING (
        cod_tenant = public.fn_current_tenant()
        AND (
            public.fn_current_empresa() IS NULL
            OR cod_empresa = public.fn_current_empresa()
        )
    )
    WITH CHECK (
        cod_tenant = public.fn_current_tenant()
        AND (
            public.fn_current_empresa() IS NULL
            OR cod_empresa = public.fn_current_empresa()
        )
    );

ALTER TABLE public.equipe_venda_gerente ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.equipe_venda_gerente FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON public.equipe_venda_gerente;
CREATE POLICY tenant_isolation ON public.equipe_venda_gerente
    FOR ALL
    TO PUBLIC
    USING (
        cod_tenant = public.fn_current_tenant()
        AND (
            public.fn_current_empresa() IS NULL
            OR cod_empresa = public.fn_current_empresa()
        )
    )
    WITH CHECK (
        cod_tenant = public.fn_current_tenant()
        AND (
            public.fn_current_empresa() IS NULL
            OR cod_empresa = public.fn_current_empresa()
        )
    );

CREATE OR REPLACE FUNCTION public.fn_current_usuario()
RETURNS INTEGER
LANGUAGE plpgsql
STABLE
AS $$
BEGIN
    RETURN NULLIF(current_setting('app.cod_usuario', true), '')::INTEGER;
EXCEPTION WHEN OTHERS THEN
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION public.fn_current_perfil()
RETURNS VARCHAR
LANGUAGE sql
STABLE
AS $$
    SELECT u.tipo
    FROM public.usuario u
    WHERE u.cod_usuario = public.fn_current_usuario()
      AND u.cod_tenant = public.fn_current_tenant()
      AND u.ativo = TRUE
    LIMIT 1
$$;

CREATE OR REPLACE FUNCTION public.fn_current_representante()
RETURNS VARCHAR
LANGUAGE sql
STABLE
AS $$
    SELECT ur.codigo_erp_representante
    FROM public.usuario_representante ur
    WHERE ur.cod_usuario = public.fn_current_usuario()
      AND ur.cod_tenant = public.fn_current_tenant()
      AND ur.cod_empresa = public.fn_current_empresa()
    LIMIT 1
$$;

CREATE OR REPLACE FUNCTION public.fn_representantes_da_equipe()
RETURNS VARCHAR[]
LANGUAGE sql
STABLE
AS $$
    SELECT COALESCE(
        array_agg(DISTINCT evg.codigo_erp_representante),
        ARRAY[]::VARCHAR[]
    )
    FROM public.equipe_venda_gerente evg
    WHERE public.fn_current_perfil() = 'gerente'
      AND evg.cod_usuario_gerente = public.fn_current_usuario()
      AND evg.cod_tenant = public.fn_current_tenant()
      AND evg.cod_empresa = public.fn_current_empresa()
$$;

COMMENT ON FUNCTION public.fn_current_usuario() IS
    'Retorna cod_usuario do contexto app.cod_usuario.';
COMMENT ON FUNCTION public.fn_current_perfil() IS
    'Retorna o perfil ativo do usuário autenticado no tenant atual.';
COMMENT ON FUNCTION public.fn_current_representante() IS
    'Retorna o representante ERP do usuário autenticado na empresa atual.';
COMMENT ON FUNCTION public.fn_representantes_da_equipe() IS
    'Retorna os representantes da equipe do gerente autenticado na empresa atual.';

-- Substitui as policies tenant/empresa das duas tabelas por políticas
-- hierárquicas. Sem usuário/perfil ou vínculo correspondente, o acesso falha
-- fechado para vendedor e gerente.
DROP POLICY IF EXISTS tenant_isolation ON public.pedido;
CREATE POLICY tenant_isolation ON public.pedido
    FOR ALL
    TO PUBLIC
    USING (
        cod_tenant = public.fn_current_tenant()
        AND (
            public.fn_current_empresa() IS NULL
            OR cod_empresa = public.fn_current_empresa()
        )
        AND CASE public.fn_current_perfil()
            WHEN 'admin' THEN TRUE
            WHEN 'supervisor' THEN TRUE
            WHEN 'gerente' THEN
                codigo_erp_representante = public.fn_current_representante()
                OR codigo_erp_representante = ANY(public.fn_representantes_da_equipe())
            WHEN 'vendedor' THEN
                codigo_erp_representante = public.fn_current_representante()
            ELSE FALSE
        END
    )
    WITH CHECK (
        cod_tenant = public.fn_current_tenant()
        AND (
            public.fn_current_empresa() IS NULL
            OR cod_empresa = public.fn_current_empresa()
        )
        AND CASE public.fn_current_perfil()
            WHEN 'admin' THEN TRUE
            WHEN 'supervisor' THEN TRUE
            WHEN 'gerente' THEN
                codigo_erp_representante = public.fn_current_representante()
                OR codigo_erp_representante = ANY(public.fn_representantes_da_equipe())
            WHEN 'vendedor' THEN
                codigo_erp_representante = public.fn_current_representante()
            ELSE FALSE
        END
    );

DROP POLICY IF EXISTS tenant_isolation ON public.cliente;
CREATE POLICY tenant_isolation ON public.cliente
    FOR ALL
    TO PUBLIC
    USING (
        cod_tenant = public.fn_current_tenant()
        AND (
            public.fn_current_empresa() IS NULL
            OR cod_empresa = public.fn_current_empresa()
        )
        AND CASE public.fn_current_perfil()
            WHEN 'admin' THEN TRUE
            WHEN 'supervisor' THEN TRUE
            WHEN 'gerente' THEN EXISTS (
                SELECT 1
                FROM public.cliente_representante cr
                WHERE cr.cod_tenant = cliente.cod_tenant
                  AND cr.cod_empresa = cliente.cod_empresa
                  AND (
                      cr.cliente_id = cliente.codigo
                      OR (
                          cr.cliente_id IS NULL
                          AND cr.codigo_erp_cliente = cliente.codigo_erp
                      )
                  )
                  AND (
                      cr.codigo_erp_representante = public.fn_current_representante()
                      OR cr.codigo_erp_representante =
                         ANY(public.fn_representantes_da_equipe())
                  )
            )
            WHEN 'vendedor' THEN EXISTS (
                SELECT 1
                FROM public.cliente_representante cr
                WHERE cr.cod_tenant = cliente.cod_tenant
                  AND cr.cod_empresa = cliente.cod_empresa
                  AND (
                      cr.cliente_id = cliente.codigo
                      OR (
                          cr.cliente_id IS NULL
                          AND cr.codigo_erp_cliente = cliente.codigo_erp
                      )
                  )
                  AND cr.codigo_erp_representante = public.fn_current_representante()
            )
            ELSE FALSE
        END
    )
    WITH CHECK (
        cod_tenant = public.fn_current_tenant()
        AND (
            public.fn_current_empresa() IS NULL
            OR cod_empresa = public.fn_current_empresa()
        )
        AND CASE public.fn_current_perfil()
            WHEN 'admin' THEN TRUE
            WHEN 'supervisor' THEN TRUE
            WHEN 'gerente' THEN EXISTS (
                SELECT 1
                FROM public.cliente_representante cr
                WHERE cr.cod_tenant = cliente.cod_tenant
                  AND cr.cod_empresa = cliente.cod_empresa
                  AND (
                      cr.cliente_id = cliente.codigo
                      OR (
                          cr.cliente_id IS NULL
                          AND cr.codigo_erp_cliente = cliente.codigo_erp
                      )
                  )
                  AND (
                      cr.codigo_erp_representante = public.fn_current_representante()
                      OR cr.codigo_erp_representante =
                         ANY(public.fn_representantes_da_equipe())
                  )
            )
            WHEN 'vendedor' THEN EXISTS (
                SELECT 1
                FROM public.cliente_representante cr
                WHERE cr.cod_tenant = cliente.cod_tenant
                  AND cr.cod_empresa = cliente.cod_empresa
                  AND (
                      cr.cliente_id = cliente.codigo
                      OR (
                          cr.cliente_id IS NULL
                          AND cr.codigo_erp_cliente = cliente.codigo_erp
                      )
                  )
                  AND cr.codigo_erp_representante = public.fn_current_representante()
            )
            ELSE FALSE
        END
    );

GRANT SELECT ON public.usuario_representante TO visionflow_ia;
GRANT SELECT ON public.equipe_venda_gerente TO visionflow_ia;
GRANT EXECUTE ON FUNCTION public.fn_current_usuario() TO visionflow_ia;
GRANT EXECUTE ON FUNCTION public.fn_current_perfil() TO visionflow_ia;
GRANT EXECUTE ON FUNCTION public.fn_current_representante() TO visionflow_ia;
GRANT EXECUTE ON FUNCTION public.fn_representantes_da_equipe() TO visionflow_ia;

COMMIT;
