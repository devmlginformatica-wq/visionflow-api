package service

import (
	"context"
	"errors"

	"github.com/celsolenonsander/visionflow-api/internal/db"
	"github.com/celsolenonsander/visionflow-api/internal/features/agenda"
	"github.com/celsolenonsander/visionflow-api/internal/features/auth"
	"github.com/celsolenonsander/visionflow-api/internal/features/catalogo"
	"github.com/celsolenonsander/visionflow-api/internal/features/cliente"
	"github.com/celsolenonsander/visionflow-api/internal/features/home"
	"github.com/celsolenonsander/visionflow-api/internal/features/notificacao"
	"github.com/celsolenonsander/visionflow-api/internal/features/relatorio"
	"github.com/celsolenonsander/visionflow-api/internal/features/shell"
	"github.com/celsolenonsander/visionflow-api/internal/features/tarefa"
	"github.com/celsolenonsander/visionflow-api/internal/models"
	"github.com/celsolenonsander/visionflow-api/internal/platform/apperrors"
)

var ErrInvalidCredentials = auth.ErrInvalidCredentials
var ErrInvalidInput = apperrors.ErrInvalidInput
var ErrDuplicateResource = apperrors.ErrDuplicateResource
var ErrIntegrationLocked = apperrors.ErrIntegrationLocked
var ErrInvalidCustomer = errors.New("cliente inválido")

func InvalidInput(message string) error {
	return apperrors.InvalidInput(message)
}

type Service struct {
	pool         *db.Pool
	auth         *auth.Service
	catalogo     *catalogo.Service
	notificacoes *notificacao.Service
	cliente      *cliente.Service
	agenda       *agenda.Service
	tarefa       *tarefa.Service
	home         *home.Service
	shell        *shell.Service
	relatorio    *relatorio.Service
}

func New(pool *db.Pool, loggers ...relatorio.ErrorLogger) *Service {
	var reportLoggers []relatorio.ErrorLogger
	if len(loggers) > 0 {
		reportLoggers = append(reportLoggers, loggers[0])
	}
	return &Service{
		pool:         pool,
		auth:         auth.New(pool),
		catalogo:     catalogo.New(pool),
		notificacoes: notificacao.New(pool),
		cliente:      cliente.New(pool),
		agenda:       agenda.New(pool),
		tarefa:       tarefa.New(pool),
		home:         home.New(pool),
		shell:        shell.New(pool),
		relatorio:    relatorio.New(pool, reportLoggers...),
	}
}

func (s *Service) Catalogo(ctx context.Context, tenantID, empresaID, userID int, recurso, busca string, limit, cursor int) ([]map[string]any, string, error) {
	return s.catalogo.List(ctx, tenantID, empresaID, userID, recurso, busca, limit, cursor)
}

func (s *Service) CatalogoComPreco(ctx context.Context, tenantID, empresaID, userID int, recurso, busca string, limit, cursor int, clienteID, tabelaPrecoID *int) ([]map[string]any, string, error) {
	items, next, err := s.catalogo.ListWithPricing(ctx, tenantID, empresaID, userID, recurso, busca, limit, cursor, clienteID, tabelaPrecoID)
	if errors.Is(err, catalogo.ErrInvalidFilter) {
		return nil, "", InvalidInput("cliente_id ou tabela_preco_id não existe ou está inativo nesta empresa")
	}
	return items, next, err
}

// ─── AUTH ───────────────────────────────────────────────────

func (s *Service) Login(ctx context.Context, email, senha string) (models.Usuario, error) {
	return s.auth.Login(ctx, email, senha)
}

func (s *Service) NotificacoesAPI(ctx context.Context, tenantID, empresaID, userID, limit, cursor int, lida *bool) ([]map[string]any, string, error) {
	return s.notificacoes.List(ctx, tenantID, empresaID, userID, limit, cursor, lida)
}

func (s *Service) ContagemNotificacoes(ctx context.Context, tenantID, empresaID, userID int) (int, error) {
	return s.notificacoes.CountUnread(ctx, tenantID, empresaID, userID)
}

func (s *Service) ClientesAPI(ctx context.Context, tenantID, empresaID int, busca, status string, limit, cursor int) ([]map[string]any, string, error) {
	return s.cliente.List(ctx, tenantID, empresaID, busca, status, limit, cursor)
}

func (s *Service) ClienteAPI(ctx context.Context, tenantID, empresaID, id int) (map[string]any, error) {
	return s.cliente.Get(ctx, tenantID, empresaID, id)
}

func (s *Service) CriarCliente(ctx context.Context, tenantID, empresaID int, input models.ClienteCreate) (map[string]any, error) {
	return s.cliente.Create(ctx, tenantID, empresaID, input)
}

func (s *Service) AtualizarCliente(ctx context.Context, tenantID, empresaID, id int, input models.ClienteCreate) (map[string]any, error) {
	return s.cliente.Update(ctx, tenantID, empresaID, id, input)
}

func (s *Service) RemoverCliente(ctx context.Context, tenantID, empresaID, id int) error {
	return s.cliente.Deactivate(ctx, tenantID, empresaID, id)
}

func (s *Service) EnderecosCliente(ctx context.Context, tenantID, empresaID, id int) ([]map[string]any, error) {
	return s.cliente.ListAddresses(ctx, tenantID, empresaID, id)
}

func (s *Service) CriarEndereco(ctx context.Context, tenantID, empresaID, clienteID int, input models.EnderecoCreate) (map[string]any, error) {
	return s.cliente.CreateAddress(ctx, tenantID, empresaID, clienteID, input)
}

func (s *Service) AtualizarEndereco(ctx context.Context, tenantID, empresaID, clienteID, enderecoID int, input models.EnderecoCreate) (map[string]any, error) {
	return s.cliente.UpdateAddress(ctx, tenantID, empresaID, clienteID, enderecoID, input)
}

func (s *Service) RemoverEndereco(ctx context.Context, tenantID, empresaID, clienteID, enderecoID int) error {
	return s.cliente.RemoveAddress(ctx, tenantID, empresaID, clienteID, enderecoID)
}

func (s *Service) ContasReceberCliente(ctx context.Context, tenantID, empresaID, clienteID int) ([]map[string]any, error) {
	return s.cliente.Receivables(ctx, tenantID, empresaID, clienteID)
}

func (s *Service) ConsultarCEP(ctx context.Context, cep string) (map[string]any, error) {
	return s.cliente.ConsultarCEP(ctx, cep)
}

func (s *Service) ConsultarCNPJ(ctx context.Context, cnpj string) (map[string]any, error) {
	return s.cliente.ConsultarCNPJ(ctx, cnpj)
}

func (s *Service) AgendaListar(ctx context.Context, tenantID, empresaID, userID, limit, cursor int, concluida *bool) ([]map[string]any, string, error) {
	return s.agenda.List(ctx, tenantID, empresaID, userID, limit, cursor, concluida)
}

func (s *Service) CriarAgenda(ctx context.Context, tenantID, empresaID, userID int, input models.AgendaCreate) (map[string]any, error) {
	return s.agenda.Create(ctx, tenantID, empresaID, userID, input)
}

func (s *Service) ObterAgenda(ctx context.Context, tenantID, empresaID, userID, id int) (map[string]any, error) {
	return s.agenda.Get(ctx, tenantID, empresaID, userID, id)
}

func (s *Service) AtualizarAgenda(ctx context.Context, tenantID, empresaID, userID, id int, input models.AgendaCreate) (map[string]any, error) {
	return s.agenda.Update(ctx, tenantID, empresaID, userID, id, input)
}

func (s *Service) ConcluirAgenda(ctx context.Context, tenantID, empresaID, userID, id int) (map[string]any, error) {
	return s.agenda.Complete(ctx, tenantID, empresaID, userID, id)
}

func (s *Service) RemoverAgenda(ctx context.Context, tenantID, empresaID, userID, id int) error {
	return s.agenda.Delete(ctx, tenantID, empresaID, userID, id)
}

func (s *Service) TarefasListar(ctx context.Context, tenantID, empresaID, userID, limit, cursor int, concluida *bool, prioridade string) ([]map[string]any, string, error) {
	return s.tarefa.List(ctx, tenantID, empresaID, userID, limit, cursor, concluida, prioridade)
}

func (s *Service) CriarTarefa(ctx context.Context, tenantID, empresaID, userID int, input models.TarefaCreate) (map[string]any, error) {
	return s.tarefa.Create(ctx, tenantID, empresaID, userID, input)
}

func (s *Service) ObterTarefa(ctx context.Context, tenantID, empresaID, userID, id int) (map[string]any, error) {
	return s.tarefa.Get(ctx, tenantID, empresaID, userID, id)
}

func (s *Service) AtualizarTarefa(ctx context.Context, tenantID, empresaID, userID, id int, input models.TarefaCreate) (map[string]any, error) {
	return s.tarefa.Update(ctx, tenantID, empresaID, userID, id, input)
}

func (s *Service) ConcluirTarefa(ctx context.Context, tenantID, empresaID, userID, id int) (map[string]any, error) {
	return s.tarefa.Complete(ctx, tenantID, empresaID, userID, id)
}

func (s *Service) RemoverTarefa(ctx context.Context, tenantID, empresaID, userID, id int) error {
	return s.tarefa.Delete(ctx, tenantID, empresaID, userID, id)
}

func (s *Service) IndicadoresAPI(ctx context.Context, tenantID, empresaID, userID int) (map[string]any, error) {
	return s.home.Indicators(ctx, tenantID, empresaID, userID)
}

func (s *Service) ShellEmpresaAPI(ctx context.Context, tenantID, empresaID, userID int) (map[string]any, error) {
	return s.shell.Get(ctx, tenantID, empresaID, userID)
}

func (s *Service) RelatorioIA(ctx context.Context, tenantID, empresaID int, userID int, pergunta string) (relatorio.RelatorioResponse, error) {
	return s.relatorio.Generate(ctx, tenantID, empresaID, pergunta)
}
