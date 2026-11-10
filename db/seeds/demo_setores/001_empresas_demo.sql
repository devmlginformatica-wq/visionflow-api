-- ============================================================
-- SEED — Demos setoriais completas: atacado, SaaS e saúde
-- ============================================================
-- Cria três tenants fictícios com uma empresa cada, usuários,
-- cadastros comerciais, representantes/equipe, catálogo, clientes,
-- produtos/estoque/preços, pedidos e tabelas de vínculo.
-- Requer migrations até V021 e o revendedor "visionflow".
-- Idempotente: preserva configurações existentes e redefine as senhas demo.
-- ============================================================

BEGIN;

INSERT INTO revendedor (
    slug, nome, cnpj, email_contato, telefone,
    marca_nome, marca_cor_primaria, marca_cor_secundaria,
    plano, percentual_comissao, limite_tenants, ativo
)
SELECT
    'visionflow', 'VisionFlow Sistemas LTDA', '12.345.678/0001-90',
    'contato@visionflow.example.test', '(11) 3000-1000',
    'VisionFlow', '#0066CC', '#004A99', 'enterprise', 30.00, 100, TRUE
WHERE NOT EXISTS (
    SELECT 1 FROM revendedor WHERE slug = 'visionflow'
);

INSERT INTO tenant (
    cod_revendedor, slug, nome, cnpj, email_contato, telefone,
    plano, limite_empresas, limite_usuarios, ativo
)
SELECT
    r.cod_revendedor, x.slug, x.nome, x.cnpj, x.email, x.telefone,
    'pro', 5, 20, TRUE
FROM revendedor r
CROSS JOIN (VALUES
    ('demo-atacado-serranorte', 'Serra Norte Atacado e Distribuição LTDA',
     '00.000.000/0001-11', 'contato@atacado.serranorte.example.test', '(11) 3000-1100'),
    ('demo-saas-nuvempioneira', 'Nuvem Pioneira Sistemas LTDA',
     '00.000.000/0001-22', 'contato@saas.nuvempioneira.example.test', '(11) 3000-2200'),
    ('demo-saude-horizonteclinico', 'Horizonte Clínico Equipamentos LTDA',
     '00.000.000/0001-33', 'contato@saude.horizonteclinico.example.test', '(11) 3000-3300')
) AS x(slug, nome, cnpj, email, telefone)
WHERE r.slug = 'visionflow'
  AND NOT EXISTS (
      SELECT 1 FROM tenant t WHERE t.slug = x.slug
  );

INSERT INTO empresa (
    cod_tenant, cod_empresa, slug, cnpj, razao_social, nome_fantasia,
    email, telefone, cep, logradouro, numero, bairro, cidade, estado, ativo
)
SELECT
    t.cod_tenant, 1, 'matriz', x.cnpj, x.nome, x.fantasia,
    x.email, x.telefone, x.cep, x.logradouro, x.numero, x.bairro,
    x.cidade, x.estado, TRUE
FROM tenant t
JOIN (VALUES
    ('demo-atacado-serranorte', '00.000.000/0001-11',
     'Serra Norte Atacado e Distribuição LTDA', 'Serra Norte Atacado',
     'matriz@atacado.serranorte.example.test', '(11) 3000-1100',
     '00000-111', 'Rua das Amostras', '100', 'Centro', 'Campinas', 'SP'),
    ('demo-saas-nuvempioneira', '00.000.000/0001-22',
     'Nuvem Pioneira Sistemas LTDA', 'Nuvem Pioneira',
     'matriz@saas.nuvempioneira.example.test', '(11) 3000-2200',
     '00000-222', 'Rua do Protótipo', '200', 'Centro', 'São Paulo', 'SP'),
    ('demo-saude-horizonteclinico', '00.000.000/0001-33',
     'Horizonte Clínico Equipamentos LTDA', 'Horizonte Clínico',
     'matriz@saude.horizonteclinico.example.test', '(11) 3000-3300',
     '00000-333', 'Rua da Pesquisa', '300', 'Centro', 'Ribeirão Preto', 'SP')
) AS x(slug, cnpj, nome, fantasia, email, telefone,
       cep, logradouro, numero, bairro, cidade, estado)
  ON x.slug = t.slug
WHERE NOT EXISTS (
    SELECT 1 FROM empresa e
    WHERE e.cod_tenant = t.cod_tenant AND e.cod_empresa = 1
);

DO $$
DECLARE
    v_demo RECORD;
    v_usuario RECORD;
    v_vinculo RECORD;
    v_tenant INTEGER;
    v_empresa INTEGER := 1;
    v_admin INTEGER;
    v_email TEXT;
BEGIN
    FOR v_demo IN
        SELECT *
        FROM (VALUES
            ('demo-atacado-serranorte'),
            ('demo-saas-nuvempioneira'),
            ('demo-saude-horizonteclinico')
        ) AS demos(slug)
    LOOP
        SELECT t.cod_tenant INTO v_tenant
        FROM tenant t
        WHERE t.slug = v_demo.slug;

        IF v_tenant IS NULL THEN
            RAISE EXCEPTION 'Tenant % não foi criado; confira o revendedor visionflow.',
                v_demo.slug;
        END IF;

        PERFORM set_config('app.cod_tenant', v_tenant::TEXT, TRUE);
        PERFORM set_config('app.cod_empresa', v_empresa::TEXT, TRUE);

        FOR v_usuario IN
            SELECT *
            FROM (VALUES
                ('admin', 'Administradora de Demonstração', 'admin'),
                ('gerente', 'Gerente de Vendas', 'gerente'),
                ('vendedor1', 'Vendedora de Demonstração 1', 'vendedor'),
                ('vendedor2', 'Vendedor de Demonstração 2', 'vendedor')
            ) AS users(alias, nome, perfil)
        LOOP
            v_email := v_usuario.alias || '@' || v_demo.slug || '.example.test';

            INSERT INTO usuario (
                cod_tenant, email, senha_hash, nome, tipo, ativo
            )
            SELECT
                v_tenant, v_email, crypt('123456', gen_salt('bf', 12)),
                v_usuario.nome, v_usuario.perfil, TRUE
            WHERE NOT EXISTS (
                SELECT 1 FROM usuario u
                WHERE u.cod_tenant = v_tenant AND u.email = v_email
            );

            UPDATE usuario
            SET senha_hash = crypt('123456', gen_salt('bf', 12)),
                nome = v_usuario.nome,
                tipo = v_usuario.perfil,
                ativo = TRUE
            WHERE cod_tenant = v_tenant
              AND email = v_email;
        END LOOP;

        INSERT INTO usuario_empresa (
            cod_usuario, cod_tenant, cod_empresa, empresa_padrao
        )
        SELECT u.cod_usuario, v_tenant, v_empresa, TRUE
        FROM usuario u
        WHERE u.cod_tenant = v_tenant
          AND u.email IN (
              'admin@' || v_demo.slug || '.example.test',
              'gerente@' || v_demo.slug || '.example.test',
              'vendedor1@' || v_demo.slug || '.example.test',
              'vendedor2@' || v_demo.slug || '.example.test'
          )
          AND NOT EXISTS (
              SELECT 1 FROM usuario_empresa ue
              WHERE ue.cod_usuario = u.cod_usuario
                AND ue.cod_tenant = v_tenant
                AND ue.cod_empresa = v_empresa
          );

        SELECT u.cod_usuario INTO v_admin
        FROM usuario u
        WHERE u.cod_tenant = v_tenant
          AND u.email = 'admin@' || v_demo.slug || '.example.test';

        IF v_admin IS NULL THEN
            RAISE EXCEPTION 'Usuário admin não encontrado para %.', v_demo.slug;
        END IF;

        PERFORM set_config('app.cod_usuario', v_admin::TEXT, TRUE);

        INSERT INTO configuracao (cod_tenant, cod_empresa, chave, valor)
        VALUES (v_tenant, v_empresa, 'vendedor_ve_dados_equipe', 'false')
        ON CONFLICT (cod_tenant, cod_empresa, chave) DO NOTHING;

        INSERT INTO unidade_medida (
            cod_tenant, cod_empresa, codigo_erp, sigla, descricao,
            casas_decimais, ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, x.sigla, x.descricao, x.decimais, TRUE
        FROM (VALUES
            ('UN', 'UN', 'Unidade', 0),
            ('CX', 'CX', 'Caixa', 0),
            ('KG', 'KG', 'Quilograma', 3)
        ) AS x(codigo, sigla, descricao, decimais)
        WHERE NOT EXISTS (
            SELECT 1 FROM unidade_medida um
            WHERE um.cod_tenant = v_tenant
              AND um.cod_empresa = v_empresa
              AND um.codigo_erp = x.codigo
        );

        INSERT INTO categoria_item (
            cod_tenant, cod_empresa, codigo_erp, descricao, codigo_erp_pai, ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, x.descricao, x.pai, TRUE
        FROM (VALUES
            ('CAT-01', 'Linha principal', NULL::VARCHAR),
            ('CAT-11', 'Produtos e licenças', 'CAT-01'),
            ('CAT-12', 'Acessórios e serviços', 'CAT-01')
        ) AS x(codigo, descricao, pai)
        WHERE NOT EXISTS (
            SELECT 1 FROM categoria_item ci
            WHERE ci.cod_tenant = v_tenant
              AND ci.cod_empresa = v_empresa
              AND ci.codigo_erp = x.codigo
        );

        INSERT INTO fabricante (
            cod_tenant, cod_empresa, codigo_erp, apelido_fantasia, ativo
        )
        SELECT v_tenant, v_empresa, 'FAB-01', x.nome, TRUE
        FROM (VALUES
            ('demo-atacado-serranorte', 'Serra Norte Alimentos'),
            ('demo-saas-nuvempioneira', 'Nuvem Pioneira Tecnologia'),
            ('demo-saude-horizonteclinico', 'Horizonte Clínico Medical')
        ) AS x(slug, nome)
        WHERE x.slug = v_demo.slug
          AND NOT EXISTS (
              SELECT 1 FROM fabricante f
              WHERE f.cod_tenant = v_tenant
                AND f.cod_empresa = v_empresa
                AND f.codigo_erp = 'FAB-01'
          );

        INSERT INTO classificacao_fiscal (
            cod_tenant, cod_empresa, codigo_erp, descricao,
            classificacao_fiscal, percentual_ipi, ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, x.descricao, x.ncm, x.ipi, TRUE
        FROM (VALUES
            ('CF-01', 'Mercadorias e alimentos', '2106.90.90', 0::NUMERIC),
            ('CF-02', 'Licenças e serviços digitais', '0000.00.00', 0::NUMERIC),
            ('CF-03', 'Equipamentos médicos e acessórios', '9018.19.80', 5::NUMERIC)
        ) AS x(codigo, descricao, ncm, ipi)
        WHERE NOT EXISTS (
            SELECT 1 FROM classificacao_fiscal cf
            WHERE cf.cod_tenant = v_tenant
              AND cf.cod_empresa = v_empresa
              AND cf.codigo_erp = x.codigo
        );

        INSERT INTO grupo_cliente (
            cod_tenant, cod_empresa, codigo_erp, descricao, ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, x.descricao, TRUE
        FROM (VALUES
            ('GR-01', 'Conta padrão'),
            ('GR-02', 'Conta estratégica')
        ) AS x(codigo, descricao)
        WHERE NOT EXISTS (
            SELECT 1 FROM grupo_cliente gc
            WHERE gc.cod_tenant = v_tenant
              AND gc.cod_empresa = v_empresa
              AND gc.codigo_erp = x.codigo
        );

        INSERT INTO ramo_atividade (
            cod_tenant, cod_empresa, codigo_erp, descricao, ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, x.descricao, TRUE
        FROM (VALUES
            ('RAM-01', 'Atacado e varejo'),
            ('RAM-02', 'Tecnologia e serviços'),
            ('RAM-03', 'Saúde e diagnóstico')
        ) AS x(codigo, descricao)
        WHERE NOT EXISTS (
            SELECT 1 FROM ramo_atividade ra
            WHERE ra.cod_tenant = v_tenant
              AND ra.cod_empresa = v_empresa
              AND ra.codigo_erp = x.codigo
        );

        INSERT INTO rota_simples (
            cod_tenant, cod_empresa, codigo_erp, descricao, ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, x.descricao, TRUE
        FROM (VALUES
            ('RT-01', 'Rota Centro'),
            ('RT-02', 'Rota Norte')
        ) AS x(codigo, descricao)
        WHERE NOT EXISTS (
            SELECT 1 FROM rota_simples rs
            WHERE rs.cod_tenant = v_tenant
              AND rs.cod_empresa = v_empresa
              AND rs.codigo_erp = x.codigo
        );

        INSERT INTO risco_cliente (
            cod_tenant, cod_empresa, codigo_erp, descricao,
            controle_cliente_receber, controle_limite_credito,
            controle_geral, cor, ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, x.descricao, 'N', 'N', 'N', x.cor, TRUE
        FROM (VALUES
            ('RIS-OK', 'Regular', 'HUE_GREEN'),
            ('RIS-AT', 'Acompanhamento', 'HUE_YELLOW')
        ) AS x(codigo, descricao, cor)
        WHERE NOT EXISTS (
            SELECT 1 FROM risco_cliente rc
            WHERE rc.cod_tenant = v_tenant
              AND rc.cod_empresa = v_empresa
              AND rc.codigo_erp = x.codigo
        );

        INSERT INTO tipo_venda (
            cod_tenant, cod_empresa, codigo_erp, descricao, ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, x.descricao, TRUE
        FROM (VALUES ('1', 'Venda normal'), ('2', 'Bonificação'))
            AS x(codigo, descricao)
        WHERE NOT EXISTS (
            SELECT 1 FROM tipo_venda tv
            WHERE tv.cod_tenant = v_tenant
              AND tv.cod_empresa = v_empresa
              AND tv.codigo_erp = x.codigo
        );

        INSERT INTO tipo_frete (
            cod_tenant, cod_empresa, codigo_erp, descricao, ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, x.descricao, TRUE
        FROM (VALUES ('0', 'CIF - remetente'), ('1', 'FOB - destinatário'))
            AS x(codigo, descricao)
        WHERE NOT EXISTS (
            SELECT 1 FROM tipo_frete tf
            WHERE tf.cod_tenant = v_tenant
              AND tf.cod_empresa = v_empresa
              AND tf.codigo_erp = x.codigo
        );

        INSERT INTO forma_pagamento (
            cod_tenant, cod_empresa, codigo_erp, descricao,
            exibir_pedidos_negociados, ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, x.descricao, TRUE, TRUE
        FROM (VALUES ('FP-PIX', 'PIX'), ('FP-BOL', 'Boleto bancário'))
            AS x(codigo, descricao)
        WHERE NOT EXISTS (
            SELECT 1 FROM forma_pagamento fp
            WHERE fp.cod_tenant = v_tenant
              AND fp.cod_empresa = v_empresa
              AND fp.codigo_erp = x.codigo
        );

        INSERT INTO condicao_pagamento (
            cod_tenant, cod_empresa, codigo_erp, descricao,
            quantidade_parcelas, prazo_medio, tipo_calculo, ativo, ordem_exibicao
        )
        SELECT v_tenant, v_empresa, x.codigo, x.descricao, x.parcelas,
               x.prazo_medio, '9', TRUE, x.ordem
        FROM (VALUES
            ('CP-00', 'À vista', 1, 0, 1),
            ('CP-30', '30 dias', 1, 30, 2),
            ('CP-3060', '30 e 60 dias', 2, 45, 3)
        ) AS x(codigo, descricao, parcelas, prazo_medio, ordem)
        WHERE NOT EXISTS (
            SELECT 1 FROM condicao_pagamento cp
            WHERE cp.cod_tenant = v_tenant
              AND cp.cod_empresa = v_empresa
              AND cp.codigo_erp = x.codigo
        );

        INSERT INTO condicao_pagamento_parcela (
            cod_tenant, cod_empresa, codigo_erp_condicao_pagamento,
            numero_parcela, quantidade_dias, percentual_rateio
        )
        SELECT v_tenant, v_empresa, x.condicao, x.numero, x.dias, x.rateio
        FROM (VALUES
            ('CP-30', 1, 30, 100::NUMERIC),
            ('CP-3060', 1, 30, 50::NUMERIC),
            ('CP-3060', 2, 60, 50::NUMERIC)
        ) AS x(condicao, numero, dias, rateio)
        WHERE NOT EXISTS (
            SELECT 1 FROM condicao_pagamento_parcela cpp
            WHERE cpp.cod_tenant = v_tenant
              AND cpp.cod_empresa = v_empresa
              AND cpp.codigo_erp_condicao_pagamento = x.condicao
              AND cpp.numero_parcela = x.numero
        );

        INSERT INTO transportadora (
            cod_tenant, cod_empresa, codigo_erp, tipo_pessoa, cnpj_cpf,
            nome_razao, nome_fantasia, ativo
        )
        SELECT v_tenant, v_empresa, 'TR-01', 'J', x.cnpj,
               x.razao, x.fantasia, TRUE
        FROM (VALUES
            ('demo-atacado-serranorte', '00.000.000/0001-41', 'Rota Segura Logística LTDA', 'Rota Segura'),
            ('demo-saas-nuvempioneira', '00.000.000/0001-42', 'Entrega Digital Serviços LTDA', 'Entrega Digital'),
            ('demo-saude-horizonteclinico', '00.000.000/0001-43', 'Transporte Clínico Especializado LTDA', 'Transporte Clínico')
        ) AS x(slug, cnpj, razao, fantasia)
        WHERE x.slug = v_demo.slug
          AND NOT EXISTS (
              SELECT 1 FROM transportadora t
              WHERE t.cod_tenant = v_tenant
                AND t.cod_empresa = v_empresa
                AND t.codigo_erp = 'TR-01'
          );

        INSERT INTO tabela_preco (
            cod_tenant, cod_empresa, codigo_erp, descricao,
            percentual_maximo_desconto, permitir_alterar_preco,
            disponibilizar_todos_clientes, ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, x.descricao, x.desconto,
               FALSE, x.todos, TRUE
        FROM (VALUES
            ('TP-01', 'Tabela padrão', 5::NUMERIC, TRUE),
            ('TP-02', 'Tabela estratégica', 10::NUMERIC, FALSE)
        ) AS x(codigo, descricao, desconto, todos)
        WHERE NOT EXISTS (
            SELECT 1 FROM tabela_preco tp
            WHERE tp.cod_tenant = v_tenant
              AND tp.cod_empresa = v_empresa
              AND tp.codigo_erp = x.codigo
        );

        INSERT INTO unidade_negocio (
            cod_tenant, cod_empresa, codigo_erp, tipo_pessoa,
            nome_razao, nome_fantasia, cidade, estado, ativo
        )
        SELECT v_tenant, v_empresa, 'UN-01', 'J', x.razao,
               x.fantasia, x.cidade, x.estado, TRUE
        FROM (VALUES
            ('demo-atacado-serranorte', 'Serra Norte Atacado e Distribuição LTDA', 'Centro de Distribuição Campinas', 'Campinas', 'SP'),
            ('demo-saas-nuvempioneira', 'Nuvem Pioneira Sistemas LTDA', 'Unidade de Serviços Digitais', 'São Paulo', 'SP'),
            ('demo-saude-horizonteclinico', 'Horizonte Clínico Equipamentos LTDA', 'Centro de Distribuição Médica', 'Ribeirão Preto', 'SP')
        ) AS x(slug, razao, fantasia, cidade, estado)
        WHERE x.slug = v_demo.slug
          AND NOT EXISTS (
              SELECT 1 FROM unidade_negocio un
              WHERE un.cod_tenant = v_tenant
                AND un.cod_empresa = v_empresa
                AND un.codigo_erp = 'UN-01'
          );

        INSERT INTO equipe_venda (
            cod_tenant, cod_empresa, codigo_erp, descricao, ativo
        )
        SELECT v_tenant, v_empresa, 'EQ-01', 'Equipe Comercial', TRUE
        WHERE NOT EXISTS (
            SELECT 1 FROM equipe_venda ev
            WHERE ev.cod_tenant = v_tenant
              AND ev.cod_empresa = v_empresa
              AND ev.codigo_erp = 'EQ-01'
        );

        INSERT INTO embalagem_venda (
            cod_tenant, cod_empresa, codigo_erp, descricao,
            codigo_erp_unidade_medida, ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, x.descricao, x.unidade, TRUE
        FROM (VALUES ('EMB-UN', 'Unidade', 'UN'), ('EMB-CX', 'Caixa', 'CX'))
            AS x(codigo, descricao, unidade)
        WHERE NOT EXISTS (
            SELECT 1 FROM embalagem_venda emb
            WHERE emb.cod_tenant = v_tenant
              AND emb.cod_empresa = v_empresa
              AND emb.codigo_erp = x.codigo
        );

        INSERT INTO representante (
            cod_tenant, cod_empresa, codigo_erp, tipo_pessoa,
            nome_razao, nome_fantasia, email, codigo_erp_equipe_venda, ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, 'F',
               x.nome, x.nome,
               x.alias || '@' || v_demo.slug || '.example.test',
               'EQ-01', TRUE
        FROM (VALUES
            ('REP-GER', 'Representante Gerente', 'gerente'),
            ('REP-01', 'Representante Vendedora 1', 'vendedor1'),
            ('REP-02', 'Representante Vendedor 2', 'vendedor2')
        ) AS x(codigo, nome, alias)
        WHERE NOT EXISTS (
            SELECT 1 FROM representante r
            WHERE r.cod_tenant = v_tenant
              AND r.cod_empresa = v_empresa
              AND r.codigo_erp = x.codigo
        );

        UPDATE representante r
        SET tipo_pessoa = 'F',
            nome_razao = x.nome,
            nome_fantasia = x.nome,
            email = x.alias || '@' || v_demo.slug || '.example.test',
            codigo_erp_equipe_venda = 'EQ-01',
            ativo = TRUE
        FROM (VALUES
            ('REP-GER', 'Representante Gerente', 'gerente'),
            ('REP-01', 'Representante Vendedora 1', 'vendedor1'),
            ('REP-02', 'Representante Vendedor 2', 'vendedor2')
        ) AS x(codigo, nome, alias)
        WHERE r.cod_tenant = v_tenant
          AND r.cod_empresa = v_empresa
          AND r.codigo_erp = x.codigo;

        FOR v_vinculo IN
            SELECT *
            FROM (VALUES
                ('gerente', 'REP-GER'),
                ('vendedor1', 'REP-01'),
                ('vendedor2', 'REP-02')
            ) AS links(alias, representante)
        LOOP
            v_email := v_vinculo.alias || '@' || v_demo.slug || '.example.test';

            INSERT INTO usuario_representante (
                cod_tenant, cod_empresa, cod_usuario, codigo_erp_representante
            )
            SELECT v_tenant, v_empresa, u.cod_usuario, v_vinculo.representante
            FROM usuario u
            WHERE u.cod_tenant = v_tenant AND u.email = v_email
              AND NOT EXISTS (
                  SELECT 1 FROM usuario_representante ur
                  WHERE ur.cod_tenant = v_tenant
                    AND ur.cod_empresa = v_empresa
                    AND ur.cod_usuario = u.cod_usuario
              );
        END LOOP;

        INSERT INTO equipe_venda_gerente (
            cod_tenant, cod_empresa, cod_usuario_gerente, codigo_erp_representante
        )
        SELECT v_tenant, v_empresa, u.cod_usuario, x.representante
        FROM usuario u
        CROSS JOIN (VALUES ('REP-01'), ('REP-02')) AS x(representante)
        WHERE u.cod_tenant = v_tenant
          AND u.email = 'gerente@' || v_demo.slug || '.example.test'
          AND NOT EXISTS (
              SELECT 1 FROM equipe_venda_gerente evg
              WHERE evg.cod_tenant = v_tenant
                AND evg.cod_empresa = v_empresa
                AND evg.cod_usuario_gerente = u.cod_usuario
                AND evg.codigo_erp_representante = x.representante
          );

        INSERT INTO item (
            cod_tenant, cod_empresa, codigo_erp, descricao,
            codigo_erp_categoria_nivel1, descricao_categoria_nivel1,
            codigo_erp_categoria_nivel2, descricao_categoria_nivel2,
            codigo_erp_unidade_medida, codigo_erp_fabricante,
            codigo_erp_classificacao_fiscal, percentual_ipi_classificacao_fiscal,
            preco_venda, preco_promocional, percentual_maximo_desconto,
            quantidade_disponivel_estoque, permite_venda_sem_estoque,
            caracteristicas, ficha_tecnica, peso_bruto, peso_liquido,
            ativo
        )
        SELECT v_tenant, v_empresa, x.codigo, x.descricao,
               'CAT-01', 'Linha principal', x.categoria, x.categoria_desc,
               CASE WHEN x.codigo IN ('AT-002', 'AT-003') THEN 'CX' ELSE 'UN' END,
               'FAB-01', x.classificacao, x.ipi,
               x.preco, x.promocional, 10, x.estoque, TRUE,
               x.caracteristicas, x.ficha, x.peso, x.peso, TRUE
        FROM (VALUES
            ('demo-atacado-serranorte', 'AT-001', 'Kit mercearia essencial', 'CAT-11', 'Mercearia', 'CF-01', 0::NUMERIC, 149.90, 139.90, 120, 'Kit de itens de mercearia para varejo.', 'Caixa fechada com itens variados.', 8.0),
            ('demo-atacado-serranorte', 'AT-002', 'Caixa de bebidas sem álcool', 'CAT-11', 'Bebidas', 'CF-01', 0::NUMERIC, 89.50, 84.90, 80, 'Seleção de bebidas não alcoólicas.', 'Caixa com unidades sortidas.', 12.0),
            ('demo-atacado-serranorte', 'AT-003', 'Fardo de produtos de limpeza', 'CAT-12', 'Limpeza', 'CF-01', 0::NUMERIC, 112.00, 105.00, 65, 'Produtos de limpeza para estabelecimentos comerciais.', 'Fardo lacrado para distribuição.', 10.0),
            ('demo-saas-nuvempioneira', 'SW-001', 'Plano Nuvem Pioneira Essencial - mensal', 'CAT-11', 'Assinaturas', 'CF-02', 0::NUMERIC, 490.00, 490.00, 999, 'Acesso mensal para pequena operação.', 'Licença SaaS mensal, renovação recorrente.', 0.0),
            ('demo-saas-nuvempioneira', 'SW-002', 'Plano Nuvem Pioneira Equipe - mensal', 'CAT-11', 'Assinaturas', 'CF-02', 0::NUMERIC, 1290.00, 1290.00, 999, 'Acesso multiusuário para equipes comerciais.', 'Licença SaaS mensal com gestão de usuários.', 0.0),
            ('demo-saas-nuvempioneira', 'SW-003', 'Implantação assistida', 'CAT-12', 'Serviços', 'CF-02', 0::NUMERIC, 2400.00, 2400.00, 999, 'Serviço de implantação e treinamento remoto.', 'Pacote de horas de consultoria inicial.', 0.0),
            ('demo-saude-horizonteclinico', 'HC-001', 'Monitor clínico multiparâmetro NX-4', 'CAT-11', 'Monitores', 'CF-03', 5::NUMERIC, 12400.00, 11900.00, 12, 'Monitor multiparâmetro para demonstração clínica.', 'Equipamento de demonstração fictício, não destinado a uso clínico.', 6.5),
            ('demo-saude-horizonteclinico', 'HC-002', 'Bomba de infusão demonstrativa HF-2', 'CAT-11', 'Bombas', 'CF-03', 5::NUMERIC, 7850.00, 7600.00, 9, 'Equipamento demonstrativo para treinamento.', 'Equipamento fictício, sem certificação ou uso em pacientes.', 4.2),
            ('demo-saude-horizonteclinico', 'HC-003', 'Kit de sensores reutilizáveis NX', 'CAT-12', 'Acessórios', 'CF-03', 5::NUMERIC, 980.00, 950.00, 40, 'Acessórios para demonstração técnica.', 'Kit de sensores de demonstração fictício.', 0.8)
        ) AS x(slug, codigo, descricao, categoria, categoria_desc, classificacao,
               ipi, preco, promocional, estoque, caracteristicas, ficha, peso)
        WHERE x.slug = v_demo.slug
          AND NOT EXISTS (
              SELECT 1 FROM item i
              WHERE i.cod_tenant = v_tenant
                AND i.cod_empresa = v_empresa
                AND i.codigo_erp = x.codigo
          );

        UPDATE item i
        SET codigo_erp_categoria_nivel1 = 'CAT-01',
            descricao_categoria_nivel1 = 'Linha principal',
            codigo_erp_categoria_nivel2 = x.categoria,
            descricao_categoria_nivel2 = x.categoria_desc,
            codigo_erp_unidade_medida = meta.unidade,
            codigo_erp_fabricante = 'FAB-01',
            apelido_fantasia_fabricante = meta.fabricante,
            codigo_erp_classificacao_fiscal = x.classificacao,
            percentual_ipi_classificacao_fiscal = x.ipi,
            preco_venda = x.preco,
            preco_promocional = x.promocional,
            percentual_maximo_desconto = 10,
            quantidade_disponivel_estoque = x.estoque,
            caracteristicas = x.caracteristicas,
            ficha_tecnica = x.ficha,
            peso_bruto = meta.peso,
            peso_liquido = meta.peso,
            ativo = TRUE
        FROM (VALUES
            ('AT-001', 'CAT-11', 'Mercearia', 'CF-01', 0::NUMERIC, 149.90, 139.90, 120, 'Itens variados para varejo.', 'Caixa fechada.'),
            ('AT-002', 'CAT-11', 'Bebidas', 'CF-01', 0::NUMERIC, 89.50, 84.90, 80, 'Bebidas não alcoólicas.', 'Caixa sortida.'),
            ('AT-003', 'CAT-12', 'Limpeza', 'CF-01', 0::NUMERIC, 112.00, 105.00, 65, 'Produtos para comércio.', 'Fardo lacrado.'),
            ('SW-001', 'CAT-11', 'Assinaturas', 'CF-02', 0::NUMERIC, 490.00, 490.00, 999, 'Plano mensal de entrada.', 'Licença SaaS mensal.'),
            ('SW-002', 'CAT-11', 'Assinaturas', 'CF-02', 0::NUMERIC, 1290.00, 1290.00, 999, 'Plano para equipes.', 'Licença SaaS multiusuário.'),
            ('SW-003', 'CAT-12', 'Serviços', 'CF-02', 0::NUMERIC, 2400.00, 2400.00, 999, 'Implantação e treinamento remoto.', 'Pacote de consultoria.'),
            ('HC-001', 'CAT-11', 'Monitores', 'CF-03', 5::NUMERIC, 12400.00, 11900.00, 12, 'Monitor para demonstração.', 'Fictício; não destinado a uso clínico.'),
            ('HC-002', 'CAT-11', 'Bombas', 'CF-03', 5::NUMERIC, 7850.00, 7600.00, 9, 'Equipamento demonstrativo.', 'Fictício; não destinado a uso clínico.'),
            ('HC-003', 'CAT-12', 'Acessórios', 'CF-03', 5::NUMERIC, 980.00, 950.00, 40, 'Acessórios de demonstração.', 'Produto demonstrativo fictício.')
        ) AS x(codigo, categoria, categoria_desc, classificacao, ipi,
               preco, promocional, estoque, caracteristicas, ficha)
        JOIN (VALUES
            ('AT-001', 'UN', 'Serra Norte Alimentos', 0::NUMERIC),
            ('AT-002', 'CX', 'Serra Norte Alimentos', 0::NUMERIC),
            ('AT-003', 'CX', 'Serra Norte Alimentos', 0::NUMERIC),
            ('SW-001', 'UN', 'Nuvem Pioneira Tecnologia', 0::NUMERIC),
            ('SW-002', 'UN', 'Nuvem Pioneira Tecnologia', 0::NUMERIC),
            ('SW-003', 'UN', 'Nuvem Pioneira Tecnologia', 0::NUMERIC),
            ('HC-001', 'UN', 'Horizonte Clínico Medical', 6.5::NUMERIC),
            ('HC-002', 'UN', 'Horizonte Clínico Medical', 4.2::NUMERIC),
            ('HC-003', 'UN', 'Horizonte Clínico Medical', 0.8::NUMERIC)
        ) AS meta(codigo, unidade, fabricante, peso) USING (codigo)
        WHERE i.cod_tenant = v_tenant
          AND i.cod_empresa = v_empresa
          AND i.codigo_erp = x.codigo
          AND x.codigo LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'SW-%'
              ELSE 'HC-%'
          END;

        INSERT INTO item_estoque (
            cod_tenant, cod_empresa, codigo_erp_unidade_negocio,
            codigo_erp_item, quantidade, quantidade_est_prevista,
            data_est_prevista
        )
        SELECT v_tenant, v_empresa, 'UN-01', i.codigo_erp,
               i.quantidade_disponivel_estoque, 0, CURRENT_DATE + 14
        FROM item i
        WHERE i.cod_tenant = v_tenant
          AND i.cod_empresa = v_empresa
          AND i.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'SW-%'
              ELSE 'HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM item_estoque ie
              WHERE ie.cod_tenant = v_tenant
                AND ie.cod_empresa = v_empresa
                AND ie.codigo_erp_unidade_negocio = 'UN-01'
                AND ie.codigo_erp_item = i.codigo_erp
          );

        INSERT INTO tabela_preco_item (
            cod_tenant, cod_empresa, codigo_erp_tabela_preco,
            codigo_erp_item_tabela_preco, codigo_erp_item,
            quantidade_inicial, quantidade_final, preco_venda,
            validade_inicial, validade_final, perc_desconto_maximo,
            preco_venda_promocao, validade_inicial_promocao,
            validade_final_promocao, perc_desconto_maximo_promocao
        )
        SELECT v_tenant, v_empresa, price.codigo,
               price.codigo || '-' || i.codigo_erp, i.codigo_erp,
               0, 999999, ROUND(i.preco_venda * price.fator, 2),
               DATE '2026-01-01', DATE '2999-12-31', price.desconto,
               ROUND(i.preco_promocional * price.fator, 2),
               DATE '2026-01-01', DATE '2999-12-31', price.desconto
        FROM item i
        CROSS JOIN (VALUES
            ('TP-01', 1::NUMERIC, 5::NUMERIC),
            ('TP-02', 0.95::NUMERIC, 10::NUMERIC)
        ) AS price(codigo, fator, desconto)
        WHERE i.cod_tenant = v_tenant
          AND i.cod_empresa = v_empresa
          AND i.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'SW-%'
              ELSE 'HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM tabela_preco_item tpi
              WHERE tpi.cod_tenant = v_tenant
                AND tpi.cod_empresa = v_empresa
                AND tpi.codigo_erp_tabela_preco = price.codigo
                AND tpi.codigo_erp_item = i.codigo_erp
          );

        INSERT INTO item_imposto (
            cod_tenant, cod_empresa, codigo_erp_item,
            codigo_erp_classificacao_fiscal, percentual_ipi, codigo_cst_origem
        )
        SELECT v_tenant, v_empresa, i.codigo_erp,
               i.codigo_erp_classificacao_fiscal,
               i.percentual_ipi_classificacao_fiscal, '0'
        FROM item i
        WHERE i.cod_tenant = v_tenant
          AND i.cod_empresa = v_empresa
          AND i.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'SW-%'
              ELSE 'HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM item_imposto ii
              WHERE ii.cod_tenant = v_tenant
                AND ii.cod_empresa = v_empresa
                AND ii.codigo_erp_item = i.codigo_erp
          );

        INSERT INTO item_unidade_negocio (
            cod_tenant, cod_empresa, codigo_erp_item, codigo_erp_unidade_negocio
        )
        SELECT v_tenant, v_empresa, i.codigo_erp, 'UN-01'
        FROM item i
        WHERE i.cod_tenant = v_tenant
          AND i.cod_empresa = v_empresa
          AND i.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'SW-%'
              ELSE 'HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM item_unidade_negocio iun
              WHERE iun.cod_tenant = v_tenant
                AND iun.cod_empresa = v_empresa
                AND iun.codigo_erp_item = i.codigo_erp
                AND iun.codigo_erp_unidade_negocio = 'UN-01'
          );

        INSERT INTO item_embalagem_venda (
            cod_tenant, cod_empresa, codigo_erp_item,
            codigo_erp_embalagem_venda, tipo_calculo, fator_conversao
        )
        SELECT v_tenant, v_empresa, x.item, x.embalagem, 'MULT', x.fator
        FROM (VALUES
            ('AT-001', 'EMB-CX', 1::NUMERIC),
            ('AT-002', 'EMB-CX', 12::NUMERIC),
            ('AT-003', 'EMB-CX', 6::NUMERIC),
            ('SW-001', 'EMB-UN', 1::NUMERIC),
            ('SW-002', 'EMB-UN', 1::NUMERIC),
            ('SW-003', 'EMB-UN', 1::NUMERIC),
            ('HC-001', 'EMB-UN', 1::NUMERIC),
            ('HC-002', 'EMB-UN', 1::NUMERIC),
            ('HC-003', 'EMB-CX', 1::NUMERIC)
        ) AS x(item, embalagem, fator)
        WHERE x.item LIKE CASE v_demo.slug
            WHEN 'demo-atacado-serranorte' THEN 'AT-%'
            WHEN 'demo-saas-nuvempioneira' THEN 'SW-%'
            ELSE 'HC-%'
        END
          AND NOT EXISTS (
              SELECT 1 FROM item_embalagem_venda iev
              WHERE iev.cod_tenant = v_tenant
                AND iev.cod_empresa = v_empresa
                AND iev.codigo_erp_item = x.item
                AND iev.codigo_erp_embalagem_venda = x.embalagem
          );

        INSERT INTO item_campo_adicional (
            cod_tenant, cod_empresa, codigo_erp_item, field, descricao, value
        )
        SELECT v_tenant, v_empresa, x.item, 'linha_setorial',
               'Segmento de demonstração', x.segmento
        FROM (VALUES
            ('AT-001', 'Atacado'), ('AT-002', 'Atacado'), ('AT-003', 'Atacado'),
            ('SW-001', 'SaaS'), ('SW-002', 'SaaS'), ('SW-003', 'SaaS'),
            ('HC-001', 'Saúde'), ('HC-002', 'Saúde'), ('HC-003', 'Saúde')
        ) AS x(item, segmento)
        WHERE x.item LIKE CASE v_demo.slug
            WHEN 'demo-atacado-serranorte' THEN 'AT-%'
            WHEN 'demo-saas-nuvempioneira' THEN 'SW-%'
            ELSE 'HC-%'
        END
          AND NOT EXISTS (
              SELECT 1 FROM item_campo_adicional ica
              WHERE ica.cod_tenant = v_tenant
                AND ica.cod_empresa = v_empresa
                AND ica.codigo_erp_item = x.item
                AND ica.field = 'linha_setorial'
          );

        INSERT INTO equipe_venda_categoria (
            cod_tenant, cod_empresa, codigo_erp_equipe_venda, codigo_categoria
        )
        SELECT v_tenant, v_empresa, 'EQ-01', x.categoria
        FROM (VALUES ('CAT-11'), ('CAT-12')) AS x(categoria)
        WHERE NOT EXISTS (
            SELECT 1 FROM equipe_venda_categoria evc
            WHERE evc.cod_tenant = v_tenant
              AND evc.cod_empresa = v_empresa
              AND evc.codigo_erp_equipe_venda = 'EQ-01'
              AND evc.codigo_categoria = x.categoria
        );

        INSERT INTO equipe_venda_item (
            cod_tenant, cod_empresa, codigo_erp_equipe_venda, codigo_erp_item
        )
        SELECT v_tenant, v_empresa, 'EQ-01', i.codigo_erp
        FROM item i
        WHERE i.cod_tenant = v_tenant
          AND i.cod_empresa = v_empresa
          AND i.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'SW-%'
              ELSE 'HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM equipe_venda_item evi
              WHERE evi.cod_tenant = v_tenant
                AND evi.cod_empresa = v_empresa
                AND evi.codigo_erp_equipe_venda = 'EQ-01'
                AND evi.codigo_erp_item = i.codigo_erp
          );

        INSERT INTO cliente (
            cod_tenant, cod_empresa, codigo_erp, prospect, tipo_pessoa,
            cnpj_cpf, nome_razao_social, nome_fantasia, cep, logradouro,
            numero_endereco, bairro, cidade, estado, telefone1, celular,
            nome_contato_comprador, email_contato_comprador,
            codigo_erp_ramo_atividade, descricao_ramo_atividade,
            codigo_erp_grupo_cliente, descricao_grupo_cliente,
            codigo_erp_tabela_preco, codigo_erp_condicao_pagamento,
            codigo_erp_forma_pagamento, codigo_erp_tipo_venda,
            codigo_erp_tipo_frete, codigo_erp_transportadora,
            limite_credito_total, tipo_bloqueio_para_venda,
            ativo, situacao_app
        )
        SELECT v_tenant, v_empresa, x.codigo, FALSE, 'J',
               x.cnpj, x.razao, x.fantasia, x.cep, x.logradouro,
               x.numero, x.bairro, x.cidade, x.estado,
               '(11) 4000-1234', '(11) 99999-1234',
               x.contato, x.email, x.ramo, x.ramo_desc, x.grupo,
               CASE x.grupo WHEN 'GR-01' THEN 'Conta padrão'
                            ELSE 'Conta estratégica' END,
               x.tabela, x.condicao, x.forma, '1', '0', 'TR-01',
               x.limite, 'N', TRUE, 'integrado'
        FROM (VALUES
            ('demo-atacado-serranorte', 'CL-AT-001', '11.111.111/0001-11', 'Mercado das Araucárias LTDA', 'Mercado Araucárias', '13000-001', 'Rua dos Ipês', '101', 'Centro', 'Campinas', 'SP', 'Marina Duarte', 'compras@araucarias.example.test', 'RAM-01', 'Atacado e varejo', 'GR-01', 'TP-01', 'CP-30', 'FP-PIX', 25000::NUMERIC),
            ('demo-atacado-serranorte', 'CL-AT-002', '11.111.111/0001-12', 'Empório Riacho Claro LTDA', 'Empório Riacho Claro', '13200-002', 'Av. das Palmeiras', '202', 'Vila Nova', 'Jundiaí', 'SP', 'Paulo Nogueira', 'compras@riachoclaro.example.test', 'RAM-01', 'Atacado e varejo', 'GR-01', 'TP-01', 'CP-3060', 'FP-BOL', 32000::NUMERIC),
            ('demo-atacado-serranorte', 'CL-AT-003', '11.111.111/0001-13', 'Hotel Estação das Flores LTDA', 'Hotel Estação das Flores', '18000-003', 'Rua da Estação', '303', 'Centro', 'Sorocaba', 'SP', 'Lívia Reis', 'compras@estacaoflores.example.test', 'RAM-01', 'Atacado e varejo', 'GR-02', 'TP-02', 'CP-3060', 'FP-BOL', 50000::NUMERIC),
            ('demo-atacado-serranorte', 'CL-AT-004', '11.111.111/0001-14', 'Mercado Boa Colheita LTDA', 'Mercado Boa Colheita', '13000-004', 'Rua do Comércio', '404', 'Jardim Sul', 'Campinas', 'SP', 'Rafael Lima', 'compras@boacolheita.example.test', 'RAM-01', 'Atacado e varejo', 'GR-02', 'TP-02', 'CP-30', 'FP-PIX', 60000::NUMERIC),
            ('demo-saas-nuvempioneira', 'CL-SW-001', '22.222.222/0001-21', 'Oficina Horizonte Digital LTDA', 'Horizonte Digital', '01000-001', 'Rua das Empresas', '101', 'Centro', 'São Paulo', 'SP', 'Nina Rocha', 'ti@horizontedigital.example.test', 'RAM-02', 'Tecnologia e serviços', 'GR-01', 'TP-01', 'CP-30', 'FP-PIX', 40000::NUMERIC),
            ('demo-saas-nuvempioneira', 'CL-SW-002', '22.222.222/0001-22', 'Cooperativa Campo Novo LTDA', 'Cooperativa Campo Novo', '80000-002', 'Av. do Sistema', '202', 'Centro Cívico', 'Curitiba', 'PR', 'Bruno Leal', 'sistemas@camponovo.example.test', 'RAM-02', 'Tecnologia e serviços', 'GR-01', 'TP-01', 'CP-30', 'FP-BOL', 45000::NUMERIC),
            ('demo-saas-nuvempioneira', 'CL-SW-003', '22.222.222/0001-23', 'Logística Vento Sul LTDA', 'Vento Sul Logística', '90000-003', 'Rua da Nuvem', '303', 'Floresta', 'Porto Alegre', 'RS', 'Marta Farias', 'ti@ventosul.example.test', 'RAM-02', 'Tecnologia e serviços', 'GR-02', 'TP-02', 'CP-3060', 'FP-BOL', 75000::NUMERIC),
            ('demo-saas-nuvempioneira', 'CL-SW-004', '22.222.222/0001-24', 'Grupo Sete Pontos LTDA', 'Grupo Sete Pontos', '01000-004', 'Rua do Código', '404', 'Centro', 'São Paulo', 'SP', 'Caio Mendes', 'compras@setepontos.example.test', 'RAM-02', 'Tecnologia e serviços', 'GR-02', 'TP-02', 'CP-30', 'FP-PIX', 90000::NUMERIC),
            ('demo-saude-horizonteclinico', 'CL-HC-001', '33.333.333/0001-31', 'Instituto Clínico do Vale LTDA', 'Clínica do Vale', '14000-001', 'Rua da Saúde', '101', 'Centro', 'Ribeirão Preto', 'SP', 'Laura Pires', 'compras@clinicadovale.example.test', 'RAM-03', 'Saúde e diagnóstico', 'GR-01', 'TP-01', 'CP-3060', 'FP-BOL', 100000::NUMERIC),
            ('demo-saude-horizonteclinico', 'CL-HC-002', '33.333.333/0001-32', 'Hospital Santa Aurora LTDA', 'Hospital Santa Aurora', '14400-002', 'Av. do Cuidado', '202', 'Jardim Norte', 'Franca', 'SP', 'Rui Campos', 'suprimentos@santaaurora.example.test', 'RAM-03', 'Saúde e diagnóstico', 'GR-02', 'TP-02', 'CP-3060', 'FP-BOL', 180000::NUMERIC),
            ('demo-saude-horizonteclinico', 'CL-HC-003', '33.333.333/0001-33', 'Centro Diagnóstico Ipê LTDA', 'Diagnóstico Ipê', '13500-003', 'Rua dos Exames', '303', 'Centro', 'São Carlos', 'SP', 'Ana Mota', 'compras@diagnosticoipe.example.test', 'RAM-03', 'Saúde e diagnóstico', 'GR-01', 'TP-01', 'CP-30', 'FP-PIX', 65000::NUMERIC),
            ('demo-saude-horizonteclinico', 'CL-HC-004', '33.333.333/0001-34', 'Rede Médica Horizonte LTDA', 'Rede Médica Horizonte', '14000-004', 'Rua da Pesquisa', '404', 'Jardim Sul', 'Ribeirão Preto', 'SP', 'Sofia Neves', 'suprimentos@redemedica.example.test', 'RAM-03', 'Saúde e diagnóstico', 'GR-02', 'TP-02', 'CP-3060', 'FP-BOL', 220000::NUMERIC)
        ) AS x(slug, codigo, cnpj, razao, fantasia, cep, logradouro, numero,
               bairro, cidade, estado, contato, email, ramo, ramo_desc, grupo, tabela,
               condicao, forma, limite)
        WHERE x.slug = v_demo.slug
          AND NOT EXISTS (
              SELECT 1 FROM cliente c
              WHERE c.cod_tenant = v_tenant
                AND c.cod_empresa = v_empresa
                AND c.codigo_erp = x.codigo
          );

        UPDATE cliente c
        SET cnpj_cpf = x.cnpj,
            nome_razao_social = x.razao,
            nome_fantasia = x.fantasia,
            cep = x.cep,
            logradouro = x.logradouro,
            numero_endereco = x.numero,
            bairro = x.bairro,
            cidade = x.cidade,
            estado = x.estado,
            telefone1 = '(11) 4000-1234',
            celular = '(11) 99999-1234',
            nome_contato_comprador = x.contato,
            email_contato_comprador = x.email,
            codigo_erp_ramo_atividade = x.ramo,
            descricao_ramo_atividade = x.ramo_desc,
            codigo_erp_grupo_cliente = x.grupo,
            descricao_grupo_cliente = CASE x.grupo
                WHEN 'GR-01' THEN 'Conta padrão'
                ELSE 'Conta estratégica'
            END,
            codigo_erp_tabela_preco = x.tabela,
            codigo_erp_condicao_pagamento = x.condicao,
            codigo_erp_forma_pagamento = x.forma,
            codigo_erp_tipo_venda = '1',
            codigo_erp_tipo_frete = '0',
            codigo_erp_transportadora = 'TR-01',
            limite_credito_total = x.limite,
            situacao_app = 'integrado',
            ativo = TRUE
        FROM (VALUES
            ('CL-AT-001', '11.111.111/0001-11', 'Mercado das Araucárias LTDA', 'Mercado Araucárias', '13000-001', 'Rua dos Ipês', '101', 'Centro', 'Campinas', 'SP', 'Marina Duarte', 'compras@araucarias.example.test', 'RAM-01', 'Atacado e varejo', 'GR-01', 'TP-01', 'CP-30', 'FP-PIX', 25000::NUMERIC),
            ('CL-AT-002', '11.111.111/0001-12', 'Empório Riacho Claro LTDA', 'Empório Riacho Claro', '13200-002', 'Av. das Palmeiras', '202', 'Vila Nova', 'Jundiaí', 'SP', 'Paulo Nogueira', 'compras@riachoclaro.example.test', 'RAM-01', 'Atacado e varejo', 'GR-01', 'TP-01', 'CP-3060', 'FP-BOL', 32000::NUMERIC),
            ('CL-AT-003', '11.111.111/0001-13', 'Hotel Estação das Flores LTDA', 'Hotel Estação das Flores', '18000-003', 'Rua da Estação', '303', 'Centro', 'Sorocaba', 'SP', 'Lívia Reis', 'compras@estacaoflores.example.test', 'RAM-01', 'Atacado e varejo', 'GR-02', 'TP-02', 'CP-3060', 'FP-BOL', 50000::NUMERIC),
            ('CL-AT-004', '11.111.111/0001-14', 'Mercado Boa Colheita LTDA', 'Mercado Boa Colheita', '13000-004', 'Rua do Comércio', '404', 'Jardim Sul', 'Campinas', 'SP', 'Rafael Lima', 'compras@boacolheita.example.test', 'RAM-01', 'Atacado e varejo', 'GR-02', 'TP-02', 'CP-30', 'FP-PIX', 60000::NUMERIC),
            ('CL-SW-001', '22.222.222/0001-21', 'Oficina Horizonte Digital LTDA', 'Horizonte Digital', '01000-001', 'Rua das Empresas', '101', 'Centro', 'São Paulo', 'SP', 'Nina Rocha', 'ti@horizontedigital.example.test', 'RAM-02', 'Tecnologia e serviços', 'GR-01', 'TP-01', 'CP-30', 'FP-PIX', 40000::NUMERIC),
            ('CL-SW-002', '22.222.222/0001-22', 'Cooperativa Campo Novo LTDA', 'Cooperativa Campo Novo', '80000-002', 'Av. do Sistema', '202', 'Centro Cívico', 'Curitiba', 'PR', 'Bruno Leal', 'sistemas@camponovo.example.test', 'RAM-02', 'Tecnologia e serviços', 'GR-01', 'TP-01', 'CP-30', 'FP-BOL', 45000::NUMERIC),
            ('CL-SW-003', '22.222.222/0001-23', 'Logística Vento Sul LTDA', 'Vento Sul Logística', '90000-003', 'Rua da Nuvem', '303', 'Floresta', 'Porto Alegre', 'RS', 'Marta Farias', 'ti@ventosul.example.test', 'RAM-02', 'Tecnologia e serviços', 'GR-02', 'TP-02', 'CP-3060', 'FP-BOL', 75000::NUMERIC),
            ('CL-SW-004', '22.222.222/0001-24', 'Grupo Sete Pontos LTDA', 'Grupo Sete Pontos', '01000-004', 'Rua do Código', '404', 'Centro', 'São Paulo', 'SP', 'Caio Mendes', 'compras@setepontos.example.test', 'RAM-02', 'Tecnologia e serviços', 'GR-02', 'TP-02', 'CP-30', 'FP-PIX', 90000::NUMERIC),
            ('CL-HC-001', '33.333.333/0001-31', 'Instituto Clínico do Vale LTDA', 'Clínica do Vale', '14000-001', 'Rua da Saúde', '101', 'Centro', 'Ribeirão Preto', 'SP', 'Dra. Laura Pires', 'compras@clinicadovale.example.test', 'RAM-03', 'Saúde e diagnóstico', 'GR-01', 'TP-01', 'CP-3060', 'FP-BOL', 100000::NUMERIC),
            ('CL-HC-002', '33.333.333/0001-32', 'Hospital Santa Aurora LTDA', 'Hospital Santa Aurora', '14400-002', 'Av. do Cuidado', '202', 'Jardim Norte', 'Franca', 'SP', 'Rui Campos', 'suprimentos@santaaurora.example.test', 'RAM-03', 'Saúde e diagnóstico', 'GR-02', 'TP-02', 'CP-3060', 'FP-BOL', 180000::NUMERIC),
            ('CL-HC-003', '33.333.333/0001-33', 'Centro Diagnóstico Ipê LTDA', 'Diagnóstico Ipê', '13500-003', 'Rua dos Exames', '303', 'Centro', 'São Carlos', 'SP', 'Ana Mota', 'compras@diagnosticoipe.example.test', 'RAM-03', 'Saúde e diagnóstico', 'GR-01', 'TP-01', 'CP-30', 'FP-PIX', 65000::NUMERIC),
            ('CL-HC-004', '33.333.333/0001-34', 'Rede Médica Horizonte LTDA', 'Rede Médica Horizonte', '14000-004', 'Rua da Pesquisa', '404', 'Jardim Sul', 'Ribeirão Preto', 'SP', 'Dra. Sofia Neves', 'suprimentos@redemedica.example.test', 'RAM-03', 'Saúde e diagnóstico', 'GR-02', 'TP-02', 'CP-3060', 'FP-BOL', 220000::NUMERIC)
        ) AS x(codigo, cnpj, razao, fantasia, cep, logradouro, numero, bairro,
               cidade, estado, contato, email, ramo, ramo_desc, grupo, tabela,
               condicao, forma, limite)
        WHERE c.cod_tenant = v_tenant
          AND c.cod_empresa = v_empresa
          AND c.codigo_erp = x.codigo
          AND x.codigo LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'CL-AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'CL-SW-%'
              ELSE 'CL-HC-%'
          END;

        INSERT INTO cliente_endereco_adicional (
            cod_tenant, cod_empresa, cliente_id, codigo_erp_cliente,
            nosso_codigo_endereco, codigo_erp_endereco,
            identificacao_endereco, tipo_logradouro, logradouro,
            numero_endereco, bairro, cidade, estado, cep,
            tipo_endereco, endereco_padrao
        )
        SELECT v_tenant, v_empresa, c.codigo, c.codigo_erp,
               'END-01', 'END-01', 'PRINCIPAL', 'Rua', c.logradouro,
               c.numero_endereco, c.bairro, c.cidade, c.estado, c.cep,
               'A', TRUE
        FROM cliente c
        WHERE c.cod_tenant = v_tenant
          AND c.cod_empresa = v_empresa
          AND c.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'CL-AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'CL-SW-%'
              ELSE 'CL-HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM cliente_endereco_adicional cea
              WHERE cea.cod_tenant = v_tenant
                AND cea.cod_empresa = v_empresa
                AND cea.cliente_id = c.codigo
                AND cea.codigo_erp_endereco = 'END-01'
          );

        INSERT INTO cliente_contato (
            cod_tenant, cod_empresa, codigo_erp_cliente,
            identificacao_contato, nome_apelido, departamento_trabalha,
            cargo_funcao_exerce, telefone, celular, email,
            receber_email_pedido_venda, receber_email_nota_fiscal_nfe,
            receber_email_boleto, receber_email_marketing
        )
        SELECT v_tenant, v_empresa, c.codigo_erp, 'COMPRAS',
               c.nome_contato_comprador, 'Compras', 'Responsável de compras',
               c.telefone1, c.celular, c.email_contato_comprador,
               TRUE, TRUE, TRUE, FALSE
        FROM cliente c
        WHERE c.cod_tenant = v_tenant
          AND c.cod_empresa = v_empresa
          AND c.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'CL-AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'CL-SW-%'
              ELSE 'CL-HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM cliente_contato cc
              WHERE cc.cod_tenant = v_tenant
                AND cc.cod_empresa = v_empresa
                AND cc.codigo_erp_cliente = c.codigo_erp
                AND cc.identificacao_contato = 'COMPRAS'
          );

        INSERT INTO cliente_representante (
            cod_tenant, cod_empresa, cliente_id,
            codigo_erp_cliente, codigo_erp_representante
        )
        SELECT v_tenant, v_empresa, c.codigo, c.codigo_erp, x.representante
        FROM (VALUES
            ('CL-AT-001', 'REP-01'), ('CL-AT-002', 'REP-02'), ('CL-AT-003', 'REP-01'), ('CL-AT-004', 'REP-GER'),
            ('CL-SW-001', 'REP-01'), ('CL-SW-002', 'REP-02'), ('CL-SW-003', 'REP-01'), ('CL-SW-004', 'REP-GER'),
            ('CL-HC-001', 'REP-01'), ('CL-HC-002', 'REP-02'), ('CL-HC-003', 'REP-01'), ('CL-HC-004', 'REP-GER')
        ) AS x(cliente, representante)
        JOIN cliente c
          ON c.cod_tenant = v_tenant
         AND c.cod_empresa = v_empresa
         AND c.codigo_erp = x.cliente
        WHERE x.cliente LIKE CASE v_demo.slug
            WHEN 'demo-atacado-serranorte' THEN 'CL-AT-%'
            WHEN 'demo-saas-nuvempioneira' THEN 'CL-SW-%'
            ELSE 'CL-HC-%'
        END
          AND NOT EXISTS (
              SELECT 1 FROM cliente_representante cr
              WHERE cr.cod_tenant = v_tenant
                AND cr.cod_empresa = v_empresa
                AND cr.cliente_id = c.codigo
                AND cr.codigo_erp_representante = x.representante
          );

        INSERT INTO cliente_tabela_preco (
            cod_tenant, cod_empresa, codigo_erp_cliente, codigo_erp_tabela_preco
        )
        SELECT v_tenant, v_empresa, c.codigo_erp, c.codigo_erp_tabela_preco
        FROM cliente c
        WHERE c.cod_tenant = v_tenant
          AND c.cod_empresa = v_empresa
          AND c.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'CL-AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'CL-SW-%'
              ELSE 'CL-HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM cliente_tabela_preco ctp
              WHERE ctp.cod_tenant = v_tenant
                AND ctp.cod_empresa = v_empresa
                AND ctp.codigo_erp_cliente = c.codigo_erp
                AND ctp.codigo_erp_tabela_preco = c.codigo_erp_tabela_preco
          );

        INSERT INTO cliente_condicao_pagamento (
            cod_tenant, cod_empresa, codigo_erp_cliente,
            codigo_erp_condicao_pagamento
        )
        SELECT v_tenant, v_empresa, c.codigo_erp, c.codigo_erp_condicao_pagamento
        FROM cliente c
        WHERE c.cod_tenant = v_tenant
          AND c.cod_empresa = v_empresa
          AND c.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'CL-AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'CL-SW-%'
              ELSE 'CL-HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM cliente_condicao_pagamento ccp
              WHERE ccp.cod_tenant = v_tenant
                AND ccp.cod_empresa = v_empresa
                AND ccp.codigo_erp_cliente = c.codigo_erp
                AND ccp.codigo_erp_condicao_pagamento = c.codigo_erp_condicao_pagamento
          );

        INSERT INTO cliente_forma_pagamento (
            cod_tenant, cod_empresa, codigo_erp_cliente, codigo_erp_forma_pagamento
        )
        SELECT v_tenant, v_empresa, c.codigo_erp, c.codigo_erp_forma_pagamento
        FROM cliente c
        WHERE c.cod_tenant = v_tenant
          AND c.cod_empresa = v_empresa
          AND c.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'CL-AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'CL-SW-%'
              ELSE 'CL-HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM cliente_forma_pagamento cfp
              WHERE cfp.cod_tenant = v_tenant
                AND cfp.cod_empresa = v_empresa
                AND cfp.codigo_erp_cliente = c.codigo_erp
                AND cfp.codigo_erp_forma_pagamento = c.codigo_erp_forma_pagamento
          );

        INSERT INTO cliente_imposto (
            cod_tenant, cod_empresa, codigo_erp_cliente,
            codigo_erp_classificacao_fiscal, percentual_ipi, codigo_cst_origem
        )
        SELECT v_tenant, v_empresa, c.codigo_erp,
               CASE v_demo.slug
                   WHEN 'demo-atacado-serranorte' THEN 'CF-01'
                   WHEN 'demo-saas-nuvempioneira' THEN 'CF-02'
                   ELSE 'CF-03'
               END,
               CASE WHEN v_demo.slug = 'demo-saude-horizonteclinico' THEN 5 ELSE 0 END,
               '0'
        FROM cliente c
        WHERE c.cod_tenant = v_tenant
          AND c.cod_empresa = v_empresa
          AND c.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'CL-AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'CL-SW-%'
              ELSE 'CL-HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM cliente_imposto ci
              WHERE ci.cod_tenant = v_tenant
                AND ci.cod_empresa = v_empresa
                AND ci.codigo_erp_cliente = c.codigo_erp
          );

        INSERT INTO cliente_campo_adicional (
            cod_tenant, cod_empresa, codigo_erp_cliente, field, value
        )
        SELECT v_tenant, v_empresa, c.codigo_erp, 'segmento_demo', v_demo.slug
        FROM cliente c
        WHERE c.cod_tenant = v_tenant
          AND c.cod_empresa = v_empresa
          AND c.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'CL-AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'CL-SW-%'
              ELSE 'CL-HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM cliente_campo_adicional cca
              WHERE cca.cod_tenant = v_tenant
                AND cca.cod_empresa = v_empresa
                AND cca.codigo_erp_cliente = c.codigo_erp
                AND cca.field = 'segmento_demo'
          );

        INSERT INTO cliente_rota_simples (
            cod_tenant, cod_empresa, codigo_erp_cliente,
            codigo_erp_rota_simples, descricao_rota_simples,
            sequenciamento_ou_horario, sequencia_horario
        )
        SELECT v_tenant, v_empresa, c.codigo_erp, 'RT-01', 'Rota Centro', 'S',
               LPAD(row_number() OVER (ORDER BY c.codigo_erp)::TEXT, 2, '0')
        FROM cliente c
        WHERE c.cod_tenant = v_tenant
          AND c.cod_empresa = v_empresa
          AND c.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'CL-AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'CL-SW-%'
              ELSE 'CL-HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM cliente_rota_simples crs
              WHERE crs.cod_tenant = v_tenant
                AND crs.cod_empresa = v_empresa
                AND crs.codigo_erp_cliente = c.codigo_erp
                AND crs.codigo_erp_rota_simples = 'RT-01'
          );

        INSERT INTO representante_tipo_venda (
            cod_tenant, cod_empresa, codigo_erp_representante, codigo_erp_tipo_venda
        )
        SELECT v_tenant, v_empresa, x.representante, x.tipo_venda
        FROM (VALUES
            ('REP-GER', '1'), ('REP-01', '1'), ('REP-02', '1'),
            ('REP-01', '2'), ('REP-02', '2')
        ) AS x(representante, tipo_venda)
        WHERE NOT EXISTS (
            SELECT 1 FROM representante_tipo_venda rtv
            WHERE rtv.cod_tenant = v_tenant
              AND rtv.cod_empresa = v_empresa
              AND rtv.codigo_erp_representante = x.representante
              AND rtv.codigo_erp_tipo_venda = x.tipo_venda
        );

        INSERT INTO representante_supervisor (
            cod_tenant, cod_empresa, codigo_erp_representante, codigo_erp_supervisor
        )
        SELECT v_tenant, v_empresa, x.representante, 'REP-GER'
        FROM (VALUES ('REP-01'), ('REP-02')) AS x(representante)
        WHERE NOT EXISTS (
            SELECT 1 FROM representante_supervisor rs
            WHERE rs.cod_tenant = v_tenant
              AND rs.cod_empresa = v_empresa
              AND rs.codigo_erp_representante = x.representante
              AND rs.codigo_erp_supervisor = 'REP-GER'
        );

        INSERT INTO tabela_preco_representante (
            cod_tenant, cod_empresa, codigo_erp_tabela_preco,
            codigo_erp_representante
        )
        SELECT v_tenant, v_empresa, x.tabela, x.representante
        FROM (VALUES
            ('TP-01', 'REP-GER'), ('TP-01', 'REP-01'), ('TP-01', 'REP-02'),
            ('TP-02', 'REP-GER'), ('TP-02', 'REP-01'), ('TP-02', 'REP-02')
        ) AS x(tabela, representante)
        WHERE NOT EXISTS (
            SELECT 1 FROM tabela_preco_representante tpr
            WHERE tpr.cod_tenant = v_tenant
              AND tpr.cod_empresa = v_empresa
              AND tpr.codigo_erp_tabela_preco = x.tabela
              AND tpr.codigo_erp_representante = x.representante
        );

        INSERT INTO tabela_preco_condicao_pagamento (
            cod_tenant, cod_empresa, codigo_erp_tabela_preco,
            codigo_erp_condicao_pagamento
        )
        SELECT v_tenant, v_empresa, x.tabela, x.condicao
        FROM (VALUES
            ('TP-01', 'CP-00'), ('TP-01', 'CP-30'),
            ('TP-02', 'CP-30'), ('TP-02', 'CP-3060')
        ) AS x(tabela, condicao)
        WHERE NOT EXISTS (
            SELECT 1 FROM tabela_preco_condicao_pagamento tpc
            WHERE tpc.cod_tenant = v_tenant
              AND tpc.cod_empresa = v_empresa
              AND tpc.codigo_erp_tabela_preco = x.tabela
              AND tpc.codigo_erp_condicao_pagamento = x.condicao
        );

        INSERT INTO tabela_preco_tipo_venda (
            cod_tenant, cod_empresa, codigo_erp_tabela_preco, codigo_erp_tipo_venda
        )
        SELECT v_tenant, v_empresa, x.tabela, x.tipo_venda
        FROM (VALUES ('TP-01', '1'), ('TP-02', '1'), ('TP-02', '2'))
            AS x(tabela, tipo_venda)
        WHERE NOT EXISTS (
            SELECT 1 FROM tabela_preco_tipo_venda tpt
            WHERE tpt.cod_tenant = v_tenant
              AND tpt.cod_empresa = v_empresa
              AND tpt.codigo_erp_tabela_preco = x.tabela
              AND tpt.codigo_erp_tipo_venda = x.tipo_venda
        );

        INSERT INTO tabela_preco_cliente (
            cod_tenant, cod_empresa, codigo_erp_tabela_preco, codigo_erp_cliente
        )
        SELECT v_tenant, v_empresa, c.codigo_erp_tabela_preco, c.codigo_erp
        FROM cliente c
        WHERE c.cod_tenant = v_tenant
          AND c.cod_empresa = v_empresa
          AND c.codigo_erp LIKE CASE v_demo.slug
              WHEN 'demo-atacado-serranorte' THEN 'CL-AT-%'
              WHEN 'demo-saas-nuvempioneira' THEN 'CL-SW-%'
              ELSE 'CL-HC-%'
          END
          AND NOT EXISTS (
              SELECT 1 FROM tabela_preco_cliente tpc
              WHERE tpc.cod_tenant = v_tenant
                AND tpc.cod_empresa = v_empresa
                AND tpc.codigo_erp_tabela_preco = c.codigo_erp_tabela_preco
                AND tpc.codigo_erp_cliente = c.codigo_erp
          );

        INSERT INTO tabela_preco_grupo_cliente (
            cod_tenant, cod_empresa, codigo_erp_tabela_preco,
            codigo_erp_grupo_cliente
        )
        SELECT v_tenant, v_empresa, x.tabela, x.grupo
        FROM (VALUES ('TP-01', 'GR-01'), ('TP-02', 'GR-02'))
            AS x(tabela, grupo)
        WHERE NOT EXISTS (
            SELECT 1 FROM tabela_preco_grupo_cliente tpg
            WHERE tpg.cod_tenant = v_tenant
              AND tpg.cod_empresa = v_empresa
              AND tpg.codigo_erp_tabela_preco = x.tabela
              AND tpg.codigo_erp_grupo_cliente = x.grupo
        );

        INSERT INTO tabela_preco_unidade_negocio (
            cod_tenant, cod_empresa, codigo_erp_tabela_preco,
            codigo_erp_unidade_negocio
        )
        SELECT v_tenant, v_empresa, x.tabela, 'UN-01'
        FROM (VALUES ('TP-01'), ('TP-02')) AS x(tabela)
        WHERE NOT EXISTS (
            SELECT 1 FROM tabela_preco_unidade_negocio tpun
            WHERE tpun.cod_tenant = v_tenant
              AND tpun.cod_empresa = v_empresa
              AND tpun.codigo_erp_tabela_preco = x.tabela
              AND tpun.codigo_erp_unidade_negocio = 'UN-01'
        );

        INSERT INTO condicao_pagamento_forma (
            cod_tenant, cod_empresa, codigo_erp_condicao_pagamento,
            codigo_erp_forma_pagamento
        )
        SELECT v_tenant, v_empresa, x.condicao, x.forma
        FROM (VALUES
            ('CP-00', 'FP-PIX'), ('CP-30', 'FP-PIX'), ('CP-30', 'FP-BOL'),
            ('CP-3060', 'FP-BOL')
        ) AS x(condicao, forma)
        WHERE NOT EXISTS (
            SELECT 1 FROM condicao_pagamento_forma cpf
            WHERE cpf.cod_tenant = v_tenant
              AND cpf.cod_empresa = v_empresa
              AND cpf.codigo_erp_condicao_pagamento = x.condicao
              AND cpf.codigo_erp_forma_pagamento = x.forma
        );

        INSERT INTO condicao_pagamento_unidade (
            cod_tenant, cod_empresa, codigo_erp_condicao_pagamento,
            codigo_erp_unidade_negocio
        )
        SELECT v_tenant, v_empresa, x.condicao, 'UN-01'
        FROM (VALUES ('CP-00'), ('CP-30'), ('CP-3060')) AS x(condicao)
        WHERE NOT EXISTS (
            SELECT 1 FROM condicao_pagamento_unidade cpu
            WHERE cpu.cod_tenant = v_tenant
              AND cpu.cod_empresa = v_empresa
              AND cpu.codigo_erp_condicao_pagamento = x.condicao
              AND cpu.codigo_erp_unidade_negocio = 'UN-01'
        );

        INSERT INTO tipo_venda_condicao_pagamento (
            cod_tenant, cod_empresa, codigo_erp_tipo_venda,
            codigo_erp_condicao_pagamento
        )
        SELECT v_tenant, v_empresa, x.tipo_venda, x.condicao
        FROM (VALUES
            ('1', 'CP-00'), ('1', 'CP-30'), ('1', 'CP-3060'),
            ('2', 'CP-00')
        ) AS x(tipo_venda, condicao)
        WHERE NOT EXISTS (
            SELECT 1 FROM tipo_venda_condicao_pagamento tvcp
            WHERE tvcp.cod_tenant = v_tenant
              AND tvcp.cod_empresa = v_empresa
              AND tvcp.codigo_erp_tipo_venda = x.tipo_venda
              AND tvcp.codigo_erp_condicao_pagamento = x.condicao
        );

        INSERT INTO tipo_venda_unidade_negocio (
            cod_tenant, cod_empresa, codigo_erp_tipo_venda,
            codigo_erp_unidade_negocio
        )
        SELECT v_tenant, v_empresa, x.tipo_venda, 'UN-01'
        FROM (VALUES ('1'), ('2')) AS x(tipo_venda)
        WHERE NOT EXISTS (
            SELECT 1 FROM tipo_venda_unidade_negocio tvun
            WHERE tvun.cod_tenant = v_tenant
              AND tvun.cod_empresa = v_empresa
              AND tvun.codigo_erp_tipo_venda = x.tipo_venda
              AND tvun.codigo_erp_unidade_negocio = 'UN-01'
        );

        INSERT INTO forma_pagamento_unidade_negocio (
            cod_tenant, cod_empresa, codigo_erp_forma_pagamento,
            codigo_erp_unidade_negocio
        )
        SELECT v_tenant, v_empresa, x.forma, 'UN-01'
        FROM (VALUES ('FP-PIX'), ('FP-BOL')) AS x(forma)
        WHERE NOT EXISTS (
            SELECT 1 FROM forma_pagamento_unidade_negocio fpun
            WHERE fpun.cod_tenant = v_tenant
              AND fpun.cod_empresa = v_empresa
              AND fpun.codigo_erp_forma_pagamento = x.forma
              AND fpun.codigo_erp_unidade_negocio = 'UN-01'
        );

        INSERT INTO pedido (
            cod_tenant, cod_empresa, codigo_erp, nosso_codigo_pedido,
            cliente_id, codigo_erp_cliente, cliente_tipo_pessoa,
            codigo_erp_representante, data_digitacao, data_emissao,
            e_um_orcamento, situacao_pedido, situacao_app, observacoes_pedido
        )
        SELECT v_tenant, v_empresa, x.pedido, x.pedido,
               c.codigo, c.codigo_erp, c.tipo_pessoa,
               x.representante, x.data_emissao, x.data_emissao,
               FALSE, '1', 'aprovado', x.observacao
        FROM (VALUES
            ('demo-atacado-serranorte', 'PD-AT-001', 'CL-AT-001', 'REP-01', 'AT-001', DATE '2026-10-20', 'Reposição mensal de mercearia'),
            ('demo-atacado-serranorte', 'PD-AT-002', 'CL-AT-002', 'REP-02', 'AT-002', DATE '2026-10-22', 'Pedido para abastecimento da loja'),
            ('demo-atacado-serranorte', 'PD-AT-003', 'CL-AT-003', 'REP-01', 'AT-003', DATE '2026-10-25', 'Entrega programada para o hotel'),
            ('demo-saas-nuvempioneira', 'PD-SW-001', 'CL-SW-001', 'REP-01', 'SW-001', DATE '2026-10-20', 'Assinatura mensal do plano essencial'),
            ('demo-saas-nuvempioneira', 'PD-SW-002', 'CL-SW-002', 'REP-02', 'SW-002', DATE '2026-10-22', 'Assinatura de equipe com onboarding'),
            ('demo-saas-nuvempioneira', 'PD-SW-003', 'CL-SW-003', 'REP-01', 'SW-003', DATE '2026-10-25', 'Projeto de implantação assistida'),
            ('demo-saude-horizonteclinico', 'PD-HC-001', 'CL-HC-001', 'REP-01', 'HC-001', DATE '2026-10-20', 'Equipamento para avaliação clínica'),
            ('demo-saude-horizonteclinico', 'PD-HC-002', 'CL-HC-002', 'REP-02', 'HC-002', DATE '2026-10-22', 'Equipamento para demonstração e treinamento'),
            ('demo-saude-horizonteclinico', 'PD-HC-003', 'CL-HC-003', 'REP-01', 'HC-003', DATE '2026-10-25', 'Reposição de acessórios compatíveis')
        ) AS x(slug, pedido, cliente, representante, produto, data_emissao, observacao)
        JOIN cliente c
          ON c.cod_tenant = v_tenant
         AND c.cod_empresa = v_empresa
         AND c.codigo_erp = x.cliente
        WHERE x.slug = v_demo.slug
          AND NOT EXISTS (
              SELECT 1 FROM pedido p
              WHERE p.cod_tenant = v_tenant
                AND p.cod_empresa = v_empresa
                AND p.codigo_erp = x.pedido
          );

        INSERT INTO pedido_item (
            cod_tenant, cod_empresa, pedido_id,
            codigo_erp_pedido, codigo_erp_item_pedido, codigo_erp_produto,
            preco_venda_original_sugerido, preco_venda_praticado_digitado,
            quantidade_informada, valor_total_produtos_liquido,
            data_entrega, situacao_item_pedido, quantidade_faturada
        )
        SELECT v_tenant, v_empresa, p.codigo, x.pedido, x.pedido || '-01',
               x.produto, i.preco_venda, i.preco_venda, x.quantidade,
               i.preco_venda * x.quantidade, p.data_emissao + 7, '1', 0
        FROM (VALUES
            ('demo-atacado-serranorte', 'PD-AT-001', 'AT-001', 8::NUMERIC),
            ('demo-atacado-serranorte', 'PD-AT-002', 'AT-002', 12::NUMERIC),
            ('demo-atacado-serranorte', 'PD-AT-003', 'AT-003', 5::NUMERIC),
            ('demo-saas-nuvempioneira', 'PD-SW-001', 'SW-001', 1::NUMERIC),
            ('demo-saas-nuvempioneira', 'PD-SW-002', 'SW-002', 1::NUMERIC),
            ('demo-saas-nuvempioneira', 'PD-SW-003', 'SW-003', 1::NUMERIC),
            ('demo-saude-horizonteclinico', 'PD-HC-001', 'HC-001', 1::NUMERIC),
            ('demo-saude-horizonteclinico', 'PD-HC-002', 'HC-002', 1::NUMERIC),
            ('demo-saude-horizonteclinico', 'PD-HC-003', 'HC-003', 4::NUMERIC)
        ) AS x(slug, pedido, produto, quantidade)
        JOIN pedido p
          ON p.cod_tenant = v_tenant
         AND p.cod_empresa = v_empresa
         AND p.codigo_erp = x.pedido
        JOIN item i
          ON i.cod_tenant = v_tenant
         AND i.cod_empresa = v_empresa
         AND i.codigo_erp = x.produto
        WHERE x.slug = v_demo.slug
          AND NOT EXISTS (
              SELECT 1 FROM pedido_item pi
              WHERE pi.cod_tenant = v_tenant
                AND pi.cod_empresa = v_empresa
                AND pi.codigo_erp_pedido = x.pedido
                AND pi.codigo_erp_item_pedido = x.pedido || '-01'
          );

        RAISE NOTICE 'Seed setorial pronto: tenant %, cod_tenant %.', v_demo.slug, v_tenant;/*  */
    END LOOP;
END
$$;

COMMIT;
