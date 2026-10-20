CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

CREATE OR REPLACE FUNCTION public.unaccent_immutable(value text)
RETURNS text
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
STRICT
AS $function$
    SELECT public.unaccent('public.unaccent'::regdictionary, value)
$function$;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_cliente_busca_nome
    ON cliente USING gin (public.unaccent_immutable(nome_razao_social) gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_cliente_busca_fantasia
    ON cliente USING gin (public.unaccent_immutable(COALESCE(nome_fantasia, '')) gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_cliente_busca_documento
    ON cliente USING gin (public.unaccent_immutable(COALESCE(cnpj_cpf, '')) gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_cliente_busca_codigo_erp
    ON cliente USING gin (public.unaccent_immutable(codigo_erp::text) gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_item_busca_descricao
    ON item USING gin (public.unaccent_immutable(descricao) gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_item_busca_codigo_erp
    ON item USING gin (public.unaccent_immutable(codigo_erp::text) gin_trgm_ops);
