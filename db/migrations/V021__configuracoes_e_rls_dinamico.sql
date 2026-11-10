-- Configuração de visibilidade por empresa e resolução segura de representantes.

BEGIN;

CREATE TABLE public.configuracao (
    cod_tenant      INTEGER         NOT NULL,
    cod_empresa     INTEGER         NOT NULL,
    chave           VARCHAR(100)    NOT NULL,
    valor           TEXT            NOT NULL,
    dt_inclusao     TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    dt_alteracao    TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_configuracao PRIMARY KEY (cod_tenant, cod_empresa, chave),
    CONSTRAINT fk_configuracao_empresa
        FOREIGN KEY (cod_tenant, cod_empresa)
        REFERENCES public.empresa (cod_tenant, cod_empresa)
        ON DELETE CASCADE
);

CREATE TRIGGER tg_configuracao_dt_alteracao
    BEFORE UPDATE ON public.configuracao
    FOR EACH ROW EXECUTE FUNCTION public.fn_atualiza_dt_alteracao();

COMMENT ON TABLE public.configuracao IS
    'Configurações funcionais específicas de cada empresa.';
COMMENT ON COLUMN public.configuracao.chave IS
    'Chave única da configuração dentro da empresa.';

-- A configuração fica desativada por padrão; preserve valores já configurados
-- caso esta migration seja reaplicada em um ambiente restaurado.
INSERT INTO public.configuracao (cod_tenant, cod_empresa, chave, valor)
SELECT e.cod_tenant, e.cod_empresa, 'vendedor_ve_dados_equipe', 'false'
FROM public.empresa e
ON CONFLICT (cod_tenant, cod_empresa, chave) DO NOTHING;

ALTER TABLE public.configuracao ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.configuracao FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON public.configuracao;
CREATE POLICY tenant_isolation ON public.configuracao
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

GRANT SELECT, INSERT, UPDATE, DELETE ON public.configuracao TO epare;
GRANT SELECT ON public.configuracao TO visionflow_ia;
GRANT SELECT ON public.usuario, public.representante, public.cliente_representante
    TO visionflow_ia;

-- Helpers de vínculo executam como a role de leitura da IA, que tem SELECT
-- sobre as tabelas necessárias. FORCE ROW LEVEL SECURITY continua limitando
-- essas consultas ao tenant/empresa presentes na transação.
CREATE OR REPLACE FUNCTION public.fn_representantes_do_usuario(p_cod_usuario INTEGER)
RETURNS VARCHAR[]
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
    SELECT COALESCE(
        array_agg(DISTINCT ur.codigo_erp_representante),
        ARRAY[]::VARCHAR[]
    )
    FROM public.usuario_representante ur
    WHERE ur.cod_usuario = p_cod_usuario
      AND ur.cod_usuario = public.fn_current_usuario()
      AND ur.cod_tenant = public.fn_current_tenant()
      AND ur.cod_empresa = public.fn_current_empresa()
$$;

CREATE OR REPLACE FUNCTION public.fn_equipe_do_gerente(p_cod_usuario_gerente INTEGER)
RETURNS VARCHAR[]
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
    SELECT COALESCE(
        array_agg(DISTINCT evg.codigo_erp_representante),
        ARRAY[]::VARCHAR[]
    )
    FROM public.equipe_venda_gerente evg
    JOIN public.usuario u
      ON u.cod_usuario = evg.cod_usuario_gerente
     AND u.cod_tenant = evg.cod_tenant
    WHERE evg.cod_usuario_gerente = p_cod_usuario_gerente
      AND evg.cod_usuario_gerente = public.fn_current_usuario()
      AND evg.cod_tenant = public.fn_current_tenant()
      AND evg.cod_empresa = public.fn_current_empresa()
      AND u.tipo = 'gerente'
      AND u.ativo = TRUE
$$;

CREATE OR REPLACE FUNCTION public.fn_representantes_permitidos()
RETURNS VARCHAR[]
LANGUAGE plpgsql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
    v_cod_usuario INTEGER := public.fn_current_usuario();
    v_cod_tenant INTEGER := public.fn_current_tenant();
    v_cod_empresa INTEGER := public.fn_current_empresa();
    v_perfil VARCHAR := public.fn_current_perfil();
    v_representantes_proprios VARCHAR[];
    v_representantes_equipe VARCHAR[];
    v_ve_dados_equipe BOOLEAN := FALSE;
BEGIN
    IF v_cod_usuario IS NULL OR v_cod_tenant IS NULL OR v_cod_empresa IS NULL THEN
        RETURN ARRAY[]::VARCHAR[];
    END IF;

    IF v_perfil IN ('admin', 'supervisor') THEN
        SELECT COALESCE(
            array_agg(DISTINCT r.codigo_erp),
            ARRAY[]::VARCHAR[]
        )
        INTO v_representantes_proprios
        FROM public.representante r
        WHERE r.cod_tenant = v_cod_tenant
          AND r.cod_empresa = v_cod_empresa;
        RETURN v_representantes_proprios;
    END IF;

    v_representantes_proprios :=
        public.fn_representantes_do_usuario(v_cod_usuario);

    IF v_perfil = 'gerente' THEN
        v_representantes_equipe :=
            public.fn_equipe_do_gerente(v_cod_usuario);
        RETURN ARRAY(
            SELECT DISTINCT rep
            FROM unnest(
                COALESCE(v_representantes_proprios, ARRAY[]::VARCHAR[])
                || COALESCE(v_representantes_equipe, ARRAY[]::VARCHAR[])
            ) AS allowed(rep)
            WHERE rep IS NOT NULL
            ORDER BY rep
        );
    END IF;

    IF v_perfil = 'vendedor' THEN
        SELECT lower(btrim(c.valor)) = 'true'
        INTO v_ve_dados_equipe
        FROM public.configuracao c
        WHERE c.cod_tenant = v_cod_tenant
          AND c.cod_empresa = v_cod_empresa
          AND c.chave = 'vendedor_ve_dados_equipe';

        IF COALESCE(v_ve_dados_equipe, FALSE) THEN
            SELECT COALESCE(
                array_agg(DISTINCT team.codigo_erp_representante),
                ARRAY[]::VARCHAR[]
            )
            INTO v_representantes_equipe
            FROM public.equipe_venda_gerente team
            JOIN public.usuario gerente
              ON gerente.cod_usuario = team.cod_usuario_gerente
             AND gerente.cod_tenant = team.cod_tenant
            WHERE team.cod_tenant = v_cod_tenant
              AND team.cod_empresa = v_cod_empresa
              AND gerente.tipo = 'gerente'
              AND gerente.ativo = TRUE
              AND team.cod_usuario_gerente IN (
                  SELECT manager_team.cod_usuario_gerente
                  FROM public.equipe_venda_gerente manager_team
                  WHERE manager_team.cod_tenant = v_cod_tenant
                    AND manager_team.cod_empresa = v_cod_empresa
                    AND manager_team.codigo_erp_representante =
                        ANY(COALESCE(v_representantes_proprios, ARRAY[]::VARCHAR[]))
              );

            RETURN ARRAY(
                SELECT DISTINCT rep
                FROM unnest(
                    COALESCE(v_representantes_proprios, ARRAY[]::VARCHAR[])
                    || COALESCE(v_representantes_equipe, ARRAY[]::VARCHAR[])
                ) AS allowed(rep)
                WHERE rep IS NOT NULL
                ORDER BY rep
            );
        END IF;

        RETURN COALESCE(v_representantes_proprios, ARRAY[]::VARCHAR[]);
    END IF;

    RETURN ARRAY[]::VARCHAR[];
END;
$$;

-- Evita expor os vínculos de outros usuários por chamadas diretas aos helpers.
REVOKE ALL ON FUNCTION public.fn_representantes_do_usuario(INTEGER) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.fn_equipe_do_gerente(INTEGER) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.fn_representantes_permitidos() FROM PUBLIC;

ALTER FUNCTION public.fn_representantes_do_usuario(INTEGER) OWNER TO visionflow_ia;
ALTER FUNCTION public.fn_equipe_do_gerente(INTEGER) OWNER TO visionflow_ia;
ALTER FUNCTION public.fn_representantes_permitidos() OWNER TO visionflow_ia;

GRANT EXECUTE ON FUNCTION public.fn_representantes_permitidos() TO epare, visionflow_ia;

CREATE OR REPLACE FUNCTION public.fn_cliente_tem_representante_permitido(
    p_cod_tenant INTEGER,
    p_cod_empresa INTEGER,
    p_cliente_id INTEGER,
    p_codigo_erp_cliente VARCHAR,
    p_representantes VARCHAR[]
)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
    SELECT
        p_cod_tenant = public.fn_current_tenant()
        AND p_cod_empresa = public.fn_current_empresa()
        AND EXISTS (
            SELECT 1
            FROM public.cliente_representante cr
            WHERE cr.cod_tenant = p_cod_tenant
              AND cr.cod_empresa = p_cod_empresa
              AND (
                  cr.cliente_id = p_cliente_id
                  OR (
                      cr.cliente_id IS NULL
                      AND cr.codigo_erp_cliente = p_codigo_erp_cliente
                  )
              )
              AND cr.codigo_erp_representante = ANY(
                  COALESCE(p_representantes, ARRAY[]::VARCHAR[])
              )
        )
$$;

ALTER FUNCTION public.fn_cliente_tem_representante_permitido(
    INTEGER, INTEGER, INTEGER, VARCHAR, VARCHAR[]
) OWNER TO visionflow_ia;
REVOKE ALL ON FUNCTION public.fn_cliente_tem_representante_permitido(
    INTEGER, INTEGER, INTEGER, VARCHAR, VARCHAR[]
) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.fn_cliente_tem_representante_permitido(
    INTEGER, INTEGER, INTEGER, VARCHAR, VARCHAR[]
) TO epare, visionflow_ia;

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
        AND (
            public.fn_current_perfil() IN ('admin', 'supervisor')
            OR (
                public.fn_current_perfil() IN ('gerente', 'vendedor')
                AND codigo_erp_representante =
                    ANY(public.fn_representantes_permitidos())
            )
        )
    )
    WITH CHECK (
        cod_tenant = public.fn_current_tenant()
        AND (
            public.fn_current_empresa() IS NULL
            OR cod_empresa = public.fn_current_empresa()
        )
        AND (
            public.fn_current_perfil() IN ('admin', 'supervisor')
            OR (
                public.fn_current_perfil() IN ('gerente', 'vendedor')
                AND codigo_erp_representante =
                    ANY(public.fn_representantes_permitidos())
            )
        )
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
        AND (
            public.fn_current_perfil() IN ('admin', 'supervisor')
            OR (
                public.fn_current_perfil() IN ('gerente', 'vendedor')
                AND public.fn_cliente_tem_representante_permitido(
                    cod_tenant,
                    cod_empresa,
                    codigo,
                    codigo_erp,
                    public.fn_representantes_permitidos()
                )
            )
        )
    )
    WITH CHECK (
        cod_tenant = public.fn_current_tenant()
        AND (
            public.fn_current_empresa() IS NULL
            OR cod_empresa = public.fn_current_empresa()
        )
        AND (
            public.fn_current_perfil() IN ('admin', 'supervisor')
            OR (
                public.fn_current_perfil() IN ('gerente', 'vendedor')
                AND public.fn_cliente_tem_representante_permitido(
                    cod_tenant,
                    cod_empresa,
                    codigo,
                    codigo_erp,
                    public.fn_representantes_permitidos()
                )
            )
        )
    );

COMMIT;
