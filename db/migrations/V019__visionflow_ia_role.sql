-- Papel dedicado para consultas da feature de relatórios com IA.
-- A senha deve ser definida fora da migration com o comando psql \password.
--
-- A role deve ser criada e ajustada manualmente por um superusuário antes
-- desta migration, pois o usuário do Flyway (epare) não pode executar
-- ALTER ROLE ... NOBYPASSRLS. Exemplo, como postgres:
--
--   CREATE ROLE visionflow_ia LOGIN;
--   ALTER ROLE visionflow_ia NOSUPERUSER NOCREATEDB NOCREATEROLE
--       NOREPLICATION NOBYPASSRLS;
--   \password visionflow_ia
--
-- Esta migration apenas garante os grants de leitura e conexão.

GRANT CONNECT ON DATABASE epare_forca_de_vendas TO visionflow_ia;
GRANT pg_read_all_data TO visionflow_ia;