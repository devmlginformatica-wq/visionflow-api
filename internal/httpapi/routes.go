package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

// Routes monta o roteador HTTP com todas as rotas.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	allowedOrigins := h.corsAllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			"http://localhost:3000",
			"http://127.0.0.1:3000",
		}
	}
	// ─── Middlewares globais ─────────────────────────────
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// ─── Health check (público) ──────────────────────────
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/swagger.yaml", swaggerYAML)
	r.Get("/swagger/index.html", swaggerUI)

	// ─── Autenticação (pública) ──────────────────────────
	r.Post("/auth/login", h.Login)

	// ─── Rotas autenticadas ──────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(h.Auth)
		r.Post("/auth/logout", h.Logout)
		r.Post("/dev/sql", h.ExecuteDevelopmentSQL)

		// Home
		r.Get("/home/indicadores", h.HomeIndicadores)
		r.Post("/relatorios/ia", h.RelatorioIA)

		// Shell
		r.Get("/shell/empresa", h.ShellEmpresaAPI)
		r.Get("/shell/notificacoes", h.HomeShellNotificacoes)

		// Notificações
		r.Get("/notificacoes", h.NotificacoesAPI)
		r.Get("/notificacoes/nao-lidas/count", h.ContagemNotificacoesAPI)

		// Clientes
		r.Get("/clientes", h.Clientes)
		r.Post("/clientes", h.CriarCliente)
		r.Get("/clientes/cnpj/{cnpj}", h.ClienteCNPJ)
		r.Get("/clientes/{id}/enderecos", h.EnderecosCliente)
		r.Post("/clientes/{id}/enderecos", h.CriarEndereco)
		r.Put("/clientes/{id}/enderecos/{enderecoId}", h.AtualizarEndereco)
		r.Delete("/clientes/{id}/enderecos/{enderecoId}", h.RemoverEndereco)
		r.Get("/clientes/{id}/contas-receber", h.ContasReceberCliente)
		r.Put("/clientes/{id}", h.AtualizarCliente)
		r.Delete("/clientes/{id}", h.RemoverCliente)
		r.Post("/clientes/{id}/finalizar", h.FinalizarClienteIntegracao)
		r.Get("/clientes/{id}", h.Cliente)
		r.Get("/cep/{cep}", h.ConsultarCEP)

		// Pedidos
		r.Get("/pedidos", h.PedidosAPI)
		r.Get("/pedidos/{id}", h.PedidoAPI)
		r.Post("/pedidos", h.SalvarPedidoAPI)
		r.Put("/pedidos/{id}", h.AtualizarPedidoAPI)
		r.Delete("/pedidos/{id}", h.RemoverPedido)
		r.Post("/pedidos/{id}/finalizar", h.FinalizarPedidoIntegracao)
		r.Post("/orcamentos/{id}/finalizar", h.FinalizarOrcamentoIntegracao)

		// Integração ERP
		r.Get("/integracao/{entidade}/pendentes", h.IntegracaoPendentes)
		r.Post("/integracao/{entidade}/{id}/retorno", h.IntegracaoRetorno)

		// Orçamentos
		r.Get("/orcamentos", h.OrcamentosAPI)
		r.Get("/orcamentos/{id}", h.OrcamentoAPI)

		// Catálogos auxiliares
		r.Get("/empresas", h.Catalogo)
		r.Get("/condicoes-pagamento", h.Catalogo)
		r.Get("/formas-pagamento", h.Catalogo)
		r.Get("/transportadoras", h.Catalogo)
		r.Get("/tabelas-preco", h.Catalogo)
		r.Get("/produtos", h.Catalogo)

		// Agenda e tarefas
		r.Get("/agenda", h.AgendaListar)
		r.Post("/agenda", h.AgendaCriar)
		r.Get("/agenda/{id}", h.AgendaObter)
		r.Put("/agenda/{id}", h.AgendaAtualizar)
		r.Patch("/agenda/{id}/concluir", h.AgendaConcluir)
		r.Delete("/agenda/{id}", h.AgendaRemover)
		r.Get("/tarefas", h.TarefasListarAPI)
		r.Post("/tarefas", h.TarefaCriar)
		r.Get("/tarefas/{id}", h.TarefaObter)
		r.Put("/tarefas/{id}", h.TarefaAtualizar)
		r.Patch("/tarefas/{id}/concluir", h.TarefaConcluir)
		r.Delete("/tarefas/{id}", h.TarefaRemover)
	})

	return r
}
