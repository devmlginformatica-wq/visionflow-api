package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	sqlConsoleTimeout        = 8 * time.Second
	sqlConsoleMaxRows        = 500
	sqlConsoleMaxSize        = 16 * 1024
	sqlConsoleMaxResultBytes = 1 << 20
)

var sqlConsoleForbiddenPattern = regexp.MustCompile(`(?i)\bset_config\b`)

type sqlConsoleRequest struct {
	SQL string `json:"sql"`
}

func (h *Handler) ExecuteDevelopmentSQL(w http.ResponseWriter, r *http.Request) {
	if !h.SQLConsoleEnabled || !strings.EqualFold(h.AppEnv, "development") {
		http.NotFound(w, r)
		return
	}
	tenantID, _, userID, ok := authenticatedIDs(r)
	if !ok {
		bad(w, http.StatusUnauthorized, "NAO_AUTENTICADO", "Contexto de autenticação inválido.")
		return
	}

	var role string
	err := h.Pool.QueryRow(r.Context(),
		`SELECT tipo FROM usuario WHERE cod_usuario=$1 AND cod_tenant=$2 AND ativo=true`,
		userID, tenantID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && role != "admin") {
		bad(w, http.StatusForbidden, "SEM_PERMISSAO", "O console SQL é restrito a administradores.")
		return
	}
	if err != nil {
		bad(w, http.StatusInternalServerError, "ERRO_INTERNO", "Não foi possível validar a permissão do console SQL.")
		return
	}

	var input sqlConsoleRequest
	if !decode(w, r, &input) {
		return
	}
	statement := strings.TrimSpace(input.SQL)
	if len(statement) == 0 || len(statement) > sqlConsoleMaxSize {
		bad(w, http.StatusBadRequest, "REQUISICAO_INVALIDA", "Informe um comando SQL com até 16 KiB.")
		return
	}
	if !validDevelopmentSQL(statement) {
		bad(w, http.StatusBadRequest, "SQL_NAO_PERMITIDO", "Use um único comando SELECT, WITH, INSERT, UPDATE ou DELETE; comandos de transação e alteração do contexto RLS não são permitidos.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), sqlConsoleTimeout)
	defer cancel()
	session, err := h.Session(ctx)
	if err != nil {
		bad(w, http.StatusInternalServerError, "ERRO_INTERNO", "Não foi possível iniciar a sessão SQL.")
		return
	}
	defer session.Rollback(ctx)

	var bypassesRLS bool
	if err := session.Tx.QueryRow(ctx, `
		SELECT db_role.rolsuper OR db_role.rolbypassrls OR EXISTS (
			SELECT 1
			FROM pg_class relation
			JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
			WHERE namespace.nspname='public'
			  AND relation.relkind IN ('r','p')
			  AND relation.relrowsecurity
			  AND NOT relation.relforcerowsecurity
			  AND pg_has_role(current_user, relation.relowner, 'USAGE')
		)
		FROM pg_roles db_role
		WHERE db_role.rolname=current_user`).Scan(&bypassesRLS); err != nil {
		bad(w, http.StatusInternalServerError, "ERRO_INTERNO", "Não foi possível verificar as proteções do banco.")
		return
	}
	if bypassesRLS {
		bad(w, http.StatusForbidden, "SQL_RLS_INSEGURO", "O console SQL exige uma role sem SUPERUSER/BYPASSRLS e que não seja proprietária de tabelas públicas protegidas por RLS sem FORCE ROW LEVEL SECURITY.")
		return
	}
	if _, err := session.Tx.Exec(ctx, "SET LOCAL row_security = on"); err != nil {
		bad(w, http.StatusInternalServerError, "ERRO_INTERNO", "Não foi possível ativar o RLS.")
		return
	}
	if _, err := session.Tx.Exec(ctx, "SET LOCAL statement_timeout = '8s'"); err != nil {
		bad(w, http.StatusInternalServerError, "ERRO_INTERNO", "Não foi possível definir o limite de execução SQL.")
		return
	}

	rows, err := session.Tx.Query(ctx, statement)
	if err != nil {
		bad(w, http.StatusBadRequest, "SQL_EXECUCAO_FALHOU", err.Error())
		return
	}
	fields := rows.FieldDescriptions()
	columns := make([]string, len(fields))
	for index, field := range fields {
		columns[index] = string(field.Name)
	}
	resultRows := make([][]any, 0)
	truncated := false
	resultBytes := 0
	for rows.Next() {
		if len(resultRows) == sqlConsoleMaxRows {
			truncated = true
			break
		}
		values, err := rows.Values()
		if err != nil {
			bad(w, http.StatusBadRequest, "SQL_EXECUCAO_FALHOU", err.Error())
			return
		}
		for index, value := range values {
			if bytes, ok := value.([]byte); ok {
				values[index] = string(bytes)
			}
		}
		encodedRow, err := json.Marshal(values)
		if err != nil {
			rows.Close()
			bad(w, http.StatusBadRequest, "SQL_RESULTADO_INVALIDO", "O resultado contém valores que não podem ser serializados em JSON.")
			return
		}
		if resultBytes+len(encodedRow) > sqlConsoleMaxResultBytes {
			rows.Close()
			bad(w, http.StatusRequestEntityTooLarge, "SQL_RESULTADO_MUITO_GRANDE", "O resultado excede o limite de 1 MiB; reduza as colunas ou o número de linhas.")
			return
		}
		resultBytes += len(encodedRow)
		resultRows = append(resultRows, values)
	}
	if err := rows.Err(); err != nil {
		bad(w, http.StatusBadRequest, "SQL_EXECUCAO_FALHOU", err.Error())
		return
	}
	rows.Close()
	command := rows.CommandTag().String()
	if err := session.Commit(ctx); err != nil {
		bad(w, http.StatusInternalServerError, "ERRO_INTERNO", "Não foi possível concluir a execução SQL.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"columns": columns, "rows": resultRows, "row_count": len(resultRows),
		"truncated": truncated, "command": command,
	}, nil)
}

func validDevelopmentSQL(statement string) bool {
	keyword := firstSQLKeyword(statement)
	switch keyword {
	case "SELECT", "WITH", "INSERT", "UPDATE", "DELETE":
	default:
		return false
	}
	return !sqlConsoleForbiddenPattern.MatchString(statement)
}

func firstSQLKeyword(statement string) string {
	for {
		statement = strings.TrimLeft(statement, " \t\r\n")
		switch {
		case strings.HasPrefix(statement, "--"):
			if newline := strings.IndexByte(statement, '\n'); newline >= 0 {
				statement = statement[newline+1:]
				continue
			}
			return ""
		case strings.HasPrefix(statement, "/*"):
			end := strings.Index(statement[2:], "*/")
			if end < 0 {
				return ""
			}
			statement = statement[end+4:]
			continue
		}
		break
	}

	end := 0
	for end < len(statement) && ((statement[end] >= 'A' && statement[end] <= 'Z') ||
		(statement[end] >= 'a' && statement[end] <= 'z')) {
		end++
	}
	return strings.ToUpper(statement[:end])
}
