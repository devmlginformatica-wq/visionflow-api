package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/celsolenonsander/visionflow-api/internal/db"
	"github.com/celsolenonsander/visionflow-api/internal/models"
	"github.com/celsolenonsander/visionflow-api/internal/service"
)

type Handler struct {
	Service            *service.Service
	Pool               *db.Pool
	JWTSecret          string
	AppEnv             string
	SQLConsoleEnabled  bool
	corsAllowedOrigins []string
}

func NewHandler(s *service.Service, pool *db.Pool, jwtSecret string, corsAllowedOrigins []string) *Handler {
	return &Handler{
		Service:            s,
		Pool:               pool,
		JWTSecret:          jwtSecret,
		corsAllowedOrigins: corsAllowedOrigins,
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}, err *models.APIError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(models.APIResponse{
		Success: err == nil,
		Data:    data,
		Error:   err,
	})
}

func bad(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, nil, &models.APIError{Code: code, Message: msg})
}

func decode(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "JSON inválido.")
		return false
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err == nil {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "A requisição deve conter apenas um objeto JSON.")
		return false
	} else if !errors.Is(err, io.EOF) {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "JSON inválido.")
		return false
	}
	return true
}

// ─── AUTH ──────────────────────────────────────────────────

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if !decode(w, r, &req) {
		return
	}
	email, emailErr := mail.ParseAddress(req.Email)
	if strings.TrimSpace(req.Email) == "" || emailErr != nil ||
		email.Address != req.Email || req.Senha == "" {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "email válido e senha são obrigatórios.")
		return
	}

	u, err := h.Service.Login(r.Context(), req.Email, req.Senha)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			bad(w, http.StatusUnauthorized, "CREDENCIAIS_INVALIDAS", "E-mail ou senha incorretos.")
			return
		}
		bad(w, http.StatusInternalServerError, "ERRO_INTERNO", "Erro inesperado no servidor.")
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": u.ID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(8 * time.Hour).Unix(),
	})

	signed, err := token.SignedString([]byte(h.JWTSecret))
	if err != nil {
		bad(w, http.StatusInternalServerError, "TOKEN_ERRO", "Não foi possível gerar o token.")
		return
	}
	u.Token = signed

	writeJSON(w, http.StatusOK, models.LoginData{Usuario: u}, nil)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, nil, nil)
}

// ─── CLIENTES ──────────────────────────────────────────────

func (h *Handler) Clientes(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, 401, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	limit, cursor, err := validPage(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		bad(w, 400, "REQUISICAO_INVALIDA", err.Error())
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "ativo" && status != "inativo" && status != "prospect" && status != "bloqueado" {
		bad(w, 400, "REQUISICAO_INVALIDA", "status deve ser ativo, inativo, prospect ou bloqueado.")
		return
	}
	items, next, err := h.Service.ClientesAPI(r.Context(), tenantID, empresaID,
		r.URL.Query().Get("busca"), status, limit, cursor)
	if err != nil {
		writeServiceError(w, err, "Não foi possível listar clientes.")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "nextCursor": cursorValue(next)}, nil)
}

func (h *Handler) Cliente(w http.ResponseWriter, r *http.Request) {
	tenantID, empresaID, _, ok := authenticatedIDs(r)
	if !ok {
		bad(w, 401, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}
	id, err := requestID(chi.URLParam(r, "id"))
	if err != nil {
		bad(w, 400, "REQUISICAO_INVALIDA", "ID do cliente inválido.")
		return
	}
	c, err := h.Service.ClienteAPI(r.Context(), tenantID, empresaID, id)
	if err != nil {
		writeServiceError(w, err, "Não foi possível consultar o cliente.")
		return
	}
	writeJSON(w, 200, c, nil)
}

func (h *Handler) ClienteCNPJ(w http.ResponseWriter, r *http.Request) {
	c, err := h.Service.ConsultarCNPJ(r.Context(), chi.URLParam(r, "cnpj"))
	if err != nil {
		writeServiceError(w, err, "Não foi possível consultar o CNPJ.")
		return
	}
	writeJSON(w, 200, c, nil)
}

// ─── HELPERS ───────────────────────────────────────────────

func authenticatedIDs(r *http.Request) (int, int, int, bool) {
	userID, userOK := UserID(r.Context())
	tenantID, tenantOK := TenantID(r.Context())
	empresaID, empresaOK := EmpresaID(r.Context())
	return tenantID, empresaID, userID, userOK && tenantOK && empresaOK
}

func requestID(value string) (int, error) {
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return 0, errors.New("identificador inválido")
	}
	return id, nil
}

func validPage(limitText, cursorText string) (int, int, error) {
	limit := 20
	if limitText != "" {
		value, err := strconv.Atoi(limitText)
		if err != nil || value < 1 || value > 100 {
			return 0, 0, errors.New("limit deve estar entre 1 e 100.")
		}
		limit = value
	}
	cursor := 0
	if cursorText != "" {
		value, err := strconv.Atoi(cursorText)
		if err != nil || value < 1 {
			return 0, 0, errors.New("cursor deve ser um identificador positivo.")
		}
		cursor = value
	}
	return limit, cursor, nil
}

func writeServiceError(w http.ResponseWriter, err error, internalMessage string) {
	if errors.Is(err, service.ErrInvalidInput) {
		message := strings.TrimPrefix(err.Error(), service.ErrInvalidInput.Error()+": ")
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", message)
		return
	}
	if errors.Is(err, service.ErrDuplicateResource) {
		bad(w, http.StatusConflict, "RECURSO_DUPLICADO", "Já existe um recurso com estes dados.")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		bad(w, http.StatusNotFound, "RECURSO_NAO_ENCONTRADO", "Recurso não encontrado.")
		return
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		bad(w, http.StatusConflict, "RECURSO_DUPLICADO", "Já existe um recurso com estes dados.")
		return
	}
	bad(w, http.StatusInternalServerError, "ERRO_INTERNO", internalMessage)
}
