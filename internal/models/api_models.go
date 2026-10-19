package models

type ClienteCreate struct {
	NomeRazaoSocial           string  `json:"nome_razao_social"`
	NomeFantasia              string  `json:"nome_fantasia"`
	CNPJCPF                   string  `json:"cnpj_cpf"`
	InscricaoEstadualRG       string  `json:"inscricao_estadual_rg"`
	TipoPessoa                string  `json:"tipo_pessoa"`
	Prospect                  bool    `json:"prospect"`
	Telefone1                 string  `json:"telefone1"`
	Telefone2                 string  `json:"telefone2"`
	Celular                   string  `json:"celular"`
	NomeContatoComprador      string  `json:"nome_contato_comprador"`
	EmailContatoComprador     string  `json:"email_contato_comprador"`
	NomeContatoFaturamento    string  `json:"nome_contato_faturamento"`
	EmailContatoFaturamento   string  `json:"email_contato_faturamento"`
	LimiteCreditoTotal        float64 `json:"limite_credito_total"`
	AnotacaoERP               string  `json:"anotacao_erp"`
	CEP                       string  `json:"cep"`
	Logradouro                string  `json:"logradouro"`
	NumeroEndereco            string  `json:"numero_endereco"`
	Complemento               string  `json:"complemento"`
	Bairro                    string  `json:"bairro"`
	Cidade                    string  `json:"cidade"`
	Estado                    string  `json:"estado"`
	TransportadoraPadraoID    *int    `json:"transportadora_padrao_id"`
	CondicaoPagamentoPadraoID *int    `json:"condicao_pagamento_padrao_id"`
	FormaPagamentoPadraoID    *int    `json:"forma_pagamento_padrao_id"`
	TabelaPrecoPadraoID       *int    `json:"tabela_preco_padrao_id"`
	DataUltimaVisita          *string `json:"data_ultima_visita"`
	AnotacaoUsuario           string  `json:"anotacao_usuario"`
}

type EnderecoCreate struct {
	TipoEndereco        string `json:"tipo_endereco"`
	EnderecoPadrao      bool   `json:"endereco_padrao"`
	CEP                 string `json:"cep"`
	Logradouro          string `json:"logradouro"`
	NumeroEndereco      string `json:"numero_endereco"`
	ComplementoEndereco string `json:"complemento_endereco"`
	Bairro              string `json:"bairro"`
	Cidade              string `json:"cidade"`
	Estado              string `json:"estado"`
	PontoReferencia     string `json:"ponto_referencia"`
	RestricoesHorarios  string `json:"restricoes_horarios"`
}

type AgendaCreate struct {
	Titulo           string  `json:"titulo"`
	Descricao        *string `json:"descricao"`
	CodigoERPCliente *string `json:"codigo_erp_cliente"`
	DataAgendada     string  `json:"data_agendada"`
	HoraAgendada     *string `json:"hora_agendada"`
}

type TarefaCreate struct {
	Titulo     string  `json:"titulo"`
	Descricao  *string `json:"descricao"`
	Prazo      *string `json:"prazo"`
	Prioridade string  `json:"prioridade"`
}

type PedidoCreate struct {
	Tipo                  string            `json:"tipo"`
	NumeroCliente         *string           `json:"numero_cliente"`
	Status                *string           `json:"status"`
	DataEmissao           string            `json:"data_emissao"`
	DataEntrega           *string           `json:"data_entrega"`
	EmpresaID             *int              `json:"empresa_id"`
	ClienteID             *int              `json:"cliente_id"`
	CondicaoPagamentoID   *int              `json:"condicao_pagamento_id"`
	FormaPagamentoID      *int              `json:"forma_pagamento_id"`
	TransportadoraID      *int              `json:"transportadora_id"`
	EnderecoFaturamentoID *int              `json:"endereco_faturamento_id"`
	EnderecoEntregaID     *int              `json:"endereco_entrega_id"`
	EnderecoCobrancaID    *int              `json:"endereco_cobranca_id"`
	ValorFrete            float64           `json:"valor_frete"`
	Observacoes           string            `json:"observacoes"`
	ObservacoesPedido     *string           `json:"observacoes_pedido"`
	ObservacoesNotaFiscal *string           `json:"observacoes_nota_fiscal"`
	Itens                 []PedidoItemInput `json:"itens"`
}

type PedidoItemInput struct {
	ID                        *int         `json:"id"`
	Sequence                  int          `json:"sequence"`
	Product                   ProdutoInput `json:"product"`
	PriceTableID              *int         `json:"price_table_id"`
	PriceTableName            *string      `json:"price_table_name"`
	PriceTableValidity        *string      `json:"price_table_validity"`
	QuantidadeInformada       float64      `json:"quantidade_informada"`
	SalePrice                 float64      `json:"sale_price"`
	OriginalPrice             float64      `json:"original_price"`
	ValorDescontoComercial    float64      `json:"valor_desconto_comercial"`
	ValorTotalProdutosLiquido *float64     `json:"valor_total_produtos_liquido"`
	DataEntrega               *string      `json:"data_entrega"`
	ObservacaoItem            *string      `json:"observacao_item"`
	CodigoCliente             *string      `json:"codigo_cliente"`
	ProdutoID                 int          `json:"produto_id"`
	Quantidade                float64      `json:"quantidade"`
	PrecoVenda                float64      `json:"preco_venda"`
	ValorDesconto             float64      `json:"valor_desconto"`
}

type ProdutoInput struct {
	ID          int      `json:"id"`
	CodigoERP   string   `json:"codigo_erp"`
	Descricao   string   `json:"descricao"`
	Familia     *string  `json:"familia"`
	Marca       *string  `json:"marca"`
	Unidade     *string  `json:"unidade"`
	PrecoVenda  *float64 `json:"preco_venda"`
	Stock       *float64 `json:"stock"`
	PesoLiquido *float64 `json:"peso_liquido"`
	PesoBruto   *float64 `json:"peso_bruto"`
}
