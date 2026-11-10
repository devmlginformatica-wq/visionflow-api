package cliente

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/celsolenonsander/visionflow-api/internal/authcontext"
	"github.com/celsolenonsander/visionflow-api/internal/db"
	"github.com/celsolenonsander/visionflow-api/internal/models"
	"github.com/celsolenonsander/visionflow-api/internal/platform/apperrors"
)

type Service struct {
	pool          *db.Pool
	repository    *Repository
	httpTransport http.RoundTripper
}

func New(pool *db.Pool) *Service {
	return &Service{pool: pool, repository: NewRepository()}
}

func (s *Service) withSession(ctx context.Context, tenantID, empresaID int, fn func(pgx.Tx) error) error {
	session, err := db.NewSession(ctx, s.pool, tenantID, empresaID, authcontext.UserID(ctx))
	if err != nil {
		return err
	}
	defer session.Rollback(ctx)
	if err := fn(session.Tx); err != nil {
		return err
	}
	return session.Commit(ctx)
}

func (s *Service) List(ctx context.Context, tenantID, empresaID int, search, status string, limit, cursor int) ([]map[string]any, string, error) {
	var items []map[string]any
	var next string
	err := s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		var err error
		items, err = s.repository.List(ctx, tx, tenantID, empresaID, search, status, limit, cursor)
		if err != nil {
			return err
		}
		items, next = db.CursorPage(items, limit, "id")
		return nil
	})
	return items, next, err
}

func (s *Service) Get(ctx context.Context, tenantID, empresaID, id int) (map[string]any, error) {
	var result map[string]any
	err := s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		item, err := s.repository.Get(ctx, tx, tenantID, empresaID, id)
		if err != nil {
			return err
		}
		addresses, err := s.repository.ListAddresses(ctx, tx, tenantID, empresaID, id)
		if err != nil {
			return err
		}
		receivables, err := s.repository.ListReceivables(ctx, tx, tenantID, empresaID, id)
		if err != nil {
			return err
		}
		item["enderecos"] = addresses
		item["contas_receber"] = receivables
		result = item
		return nil
	})
	return result, err
}

func (s *Service) Create(ctx context.Context, tenantID, empresaID int, input models.ClienteCreate) (map[string]any, error) {
	if err := validCliente(input); err != nil {
		return nil, err
	}
	var result map[string]any
	err := s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		duplicate, err := s.repository.IsDuplicate(ctx, tx, tenantID, empresaID, input.CNPJCPF, 0)
		if err != nil {
			return err
		}
		if duplicate {
			return apperrors.ErrDuplicateResource
		}
		codes, err := s.repository.resolveDefaultCodes(ctx, tx, tenantID, empresaID, input)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.InvalidInput("um dos IDs de padrão comercial não existe ou está inativo")
		}
		if err != nil {
			return err
		}
		result, err = s.repository.Create(ctx, tx, tenantID, empresaID, input, codes)
		return err
	})
	return result, err
}

func (s *Service) Update(ctx context.Context, tenantID, empresaID, id int, input models.ClienteCreate) (map[string]any, error) {
	if err := validCliente(input); err != nil {
		return nil, err
	}
	var result map[string]any
	err := s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		duplicate, err := s.repository.IsDuplicate(ctx, tx, tenantID, empresaID, input.CNPJCPF, id)
		if err != nil {
			return err
		}
		if duplicate {
			return apperrors.ErrDuplicateResource
		}
		codes, err := s.repository.resolveDefaultCodes(ctx, tx, tenantID, empresaID, input)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.InvalidInput("um dos IDs de padrão comercial não existe ou está inativo")
		}
		if err != nil {
			return err
		}
		result, err = s.repository.Update(ctx, tx, tenantID, empresaID, id, input, codes)
		return err
	})
	return result, err
}

func (s *Service) Deactivate(ctx context.Context, tenantID, empresaID, id int) error {
	return s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		return s.repository.Deactivate(ctx, tx, tenantID, empresaID, id)
	})
}

func (s *Service) ListAddresses(ctx context.Context, tenantID, empresaID, customerID int) ([]map[string]any, error) {
	var addresses []map[string]any
	err := s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		if _, err := s.repository.Get(ctx, tx, tenantID, empresaID, customerID); err != nil {
			return err
		}
		var err error
		addresses, err = s.repository.ListAddresses(ctx, tx, tenantID, empresaID, customerID)
		return err
	})
	return addresses, err
}

func (s *Service) CreateAddress(ctx context.Context, tenantID, empresaID, customerID int, input models.EnderecoCreate) (map[string]any, error) {
	dbType, err := validEndereco(input)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		var err error
		result, err = s.repository.CreateAddress(ctx, tx, tenantID, empresaID, customerID, input, dbType)
		return err
	})
	return result, err
}

func (s *Service) UpdateAddress(ctx context.Context, tenantID, empresaID, customerID, addressID int, input models.EnderecoCreate) (map[string]any, error) {
	dbType, err := validEndereco(input)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		var err error
		result, err = s.repository.UpdateAddress(ctx, tx, tenantID, empresaID, customerID, addressID, input, dbType)
		return err
	})
	return result, err
}

func (s *Service) RemoveAddress(ctx context.Context, tenantID, empresaID, customerID, addressID int) error {
	return s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		return s.repository.RemoveAddress(ctx, tx, tenantID, empresaID, customerID, addressID)
	})
}

func validEndereco(input models.EnderecoCreate) (string, error) {
	if strings.TrimSpace(input.TipoEndereco) == "" || strings.TrimSpace(input.CEP) == "" ||
		strings.TrimSpace(input.Logradouro) == "" || strings.TrimSpace(input.NumeroEndereco) == "" ||
		strings.TrimSpace(input.Bairro) == "" || strings.TrimSpace(input.Cidade) == "" ||
		len(input.Estado) != 2 {
		return "", apperrors.InvalidInput("campos obrigatórios do endereço ausentes ou inválidos")
	}
	return bancoTipoEndereco(input.TipoEndereco)
}

func (s *Service) Receivables(ctx context.Context, tenantID, empresaID, customerID int) ([]map[string]any, error) {
	var items []map[string]any
	err := s.withSession(ctx, tenantID, empresaID, func(tx pgx.Tx) error {
		if _, err := s.repository.Get(ctx, tx, tenantID, empresaID, customerID); err != nil {
			return err
		}
		var err error
		items, err = s.repository.ListReceivables(ctx, tx, tenantID, empresaID, customerID)
		return err
	})
	return items, err
}

func validCliente(input models.ClienteCreate) error {
	if strings.TrimSpace(input.NomeRazaoSocial) == "" {
		return apperrors.InvalidInput("nome_razao_social é obrigatório")
	}
	if input.TipoPessoa != "PF" && input.TipoPessoa != "PJ" {
		return apperrors.InvalidInput("tipo_pessoa deve ser PF ou PJ")
	}
	for _, reference := range []*int{
		input.TransportadoraPadraoID, input.CondicaoPagamentoPadraoID,
		input.FormaPagamentoPadraoID, input.TabelaPrecoPadraoID,
	} {
		if reference != nil && *reference <= 0 {
			return apperrors.InvalidInput("IDs de padrão comercial devem ser positivos")
		}
	}
	if input.DataUltimaVisita != nil && *input.DataUltimaVisita != "" {
		if _, err := time.Parse("2006-01-02", *input.DataUltimaVisita); err != nil {
			return apperrors.InvalidInput("data_ultima_visita deve estar no formato YYYY-MM-DD")
		}
	}
	if strings.TrimSpace(input.CNPJCPF) != "" {
		document := digitsOnly(input.CNPJCPF)
		expectedDigits := 14
		if input.TipoPessoa == "PF" {
			expectedDigits = 11
		}
		if len(document) != expectedDigits {
			return apperrors.InvalidInput(fmt.Sprintf("cnpj_cpf deve conter %d dígitos para tipo_pessoa %s", expectedDigits, input.TipoPessoa))
		}
	}
	if email := strings.TrimSpace(input.EmailContatoComprador); email != "" {
		parsed, err := mail.ParseAddress(email)
		if err != nil || parsed.Address != email {
			return apperrors.InvalidInput("email_contato_comprador inválido")
		}
	}
	for _, field := range []struct {
		value string
		name  string
		max   int
	}{
		{input.NomeRazaoSocial, "nome_razao_social", 100}, {input.NomeFantasia, "nome_fantasia", 100},
		{input.CNPJCPF, "cnpj_cpf", 20}, {input.InscricaoEstadualRG, "inscricao_estadual_rg", 20},
		{input.Telefone1, "telefone1", 20}, {input.Telefone2, "telefone2", 20}, {input.Celular, "celular", 20},
		{input.NomeContatoComprador, "nome_contato_comprador", 100}, {input.EmailContatoComprador, "email_contato_comprador", 100},
		{input.NomeContatoFaturamento, "nome_contato_faturamento", 100}, {input.EmailContatoFaturamento, "email_contato_faturamento", 100},
		{input.CEP, "cep", 10}, {input.Logradouro, "logradouro", 100}, {input.NumeroEndereco, "numero_endereco", 20},
		{input.Complemento, "complemento", 100}, {input.Bairro, "bairro", 100}, {input.Cidade, "cidade", 100}, {input.Estado, "estado", 2},
	} {
		if len([]rune(field.value)) > field.max {
			return apperrors.InvalidInput(fmt.Sprintf("%s excede %d caracteres", field.name, field.max))
		}
	}
	return nil
}

func bancoTipoPessoa(value string) string {
	if value == "PF" {
		return "F"
	}
	if value == "PJ" {
		return "J"
	}
	return value
}

func bancoTipoEndereco(value string) (string, error) {
	switch value {
	case "C", "cobranca":
		return "C", nil
	case "E", "entrega":
		return "E", nil
	case "A", "ambos":
		return "A", nil
	default:
		return "", apperrors.InvalidInput("tipo_endereco deve ser C, E ou A")
	}
}

func (s *Service) ConsultarCEP(ctx context.Context, cep string) (map[string]any, error) {
	digits := digitsOnly(cep)
	if len(digits) != 8 {
		return nil, apperrors.InvalidInput("CEP deve conter 8 dígitos")
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: s.httpTransport}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://viacep.com.br/ws/"+digits+"/json/", nil)
	if err != nil {
		return nil, fmt.Errorf("criar requisição de CEP: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("consultar serviço de CEP: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, pgx.ErrNoRows
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serviço de CEP retornou status %d", response.StatusCode)
	}
	var payload struct {
		CEP        string `json:"cep"`
		Logradouro string `json:"logradouro"`
		Bairro     string `json:"bairro"`
		Localidade string `json:"localidade"`
		UF         string `json:"uf"`
		Erro       bool   `json:"erro"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decodificar resposta do serviço de CEP: %w", err)
	}
	if payload.Erro {
		return nil, pgx.ErrNoRows
	}
	return map[string]any{
		"cep": payload.CEP, "logradouro": payload.Logradouro, "bairro": payload.Bairro,
		"cidade": payload.Localidade, "estado": payload.UF,
	}, nil
}

func (s *Service) ConsultarCNPJ(ctx context.Context, cnpj string) (map[string]any, error) {
	digits := digitsOnly(cnpj)
	if len(digits) != 14 {
		return nil, apperrors.InvalidInput("CNPJ deve conter 14 dígitos")
	}
	client := &http.Client{Timeout: 8 * time.Second, Transport: s.httpTransport}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://brasilapi.com.br/api/cnpj/v1/"+digits, nil)
	if err != nil {
		return nil, fmt.Errorf("criar requisição de CNPJ: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("consultar serviço de CNPJ: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, pgx.ErrNoRows
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serviço de CNPJ retornou status %d", response.StatusCode)
	}
	var payload struct {
		CNPJ                string `json:"cnpj"`
		RazaoSocial         string `json:"razao_social"`
		NomeFantasia        string `json:"nome_fantasia"`
		Email               string `json:"email"`
		Telefone            string `json:"ddd_telefone_1"`
		CEP                 string `json:"cep"`
		Logradouro          string `json:"logradouro"`
		Numero              string `json:"numero"`
		Complemento         string `json:"complemento"`
		Bairro              string `json:"bairro"`
		Cidade              string `json:"municipio"`
		Estado              string `json:"uf"`
		InscricoesEstaduais []struct {
			InscricaoEstadual string `json:"inscricao_estadual"`
			Ativo             bool   `json:"ativo"`
		} `json:"inscricoes_estaduais"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decodificar resposta do serviço de CNPJ: %w", err)
	}
	stateRegistration := ""
	for _, registration := range payload.InscricoesEstaduais {
		if registration.Ativo && registration.InscricaoEstadual != "" {
			stateRegistration = registration.InscricaoEstadual
			break
		}
		if stateRegistration == "" {
			stateRegistration = registration.InscricaoEstadual
		}
	}
	return map[string]any{
		"cnpj": payload.CNPJ, "razao_social": payload.RazaoSocial,
		"nome_fantasia": payload.NomeFantasia, "inscricao_estadual": stateRegistration,
		"email": payload.Email, "telefone": payload.Telefone,
		"endereco": map[string]any{
			"cep": payload.CEP, "logradouro": payload.Logradouro, "numero": payload.Numero,
			"complemento": payload.Complemento, "bairro": payload.Bairro,
			"cidade": payload.Cidade, "estado": payload.Estado,
		},
	}, nil
}

func digitsOnly(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
}
