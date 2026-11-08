package relatorio

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/xwb1989/sqlparser"
)

var dangerousFunctionPattern = regexp.MustCompile(`(?i)\b(pg_sleep|pg_read_file|pg_read_other_files|pg_stat_file|pg_ls_dir|pg_file_read|copy|lo_import|lo_export|dblink|pg_term|pg_log_backend_memory_contexts|pg_cancel_backend|pg_terminate_backend)\s*\(`)

func validateReadOnlySQL(rawSQL string) error {
	query := strings.TrimSpace(rawSQL)
	if query == "" {
		return fmt.Errorf("SQL vazio")
	}
	query = normalizeSQL(query)
	if strings.Contains(query, ";") {
		return fmt.Errorf("a query gerada pela IA não pode conter múltiplas instruções")
	}
	stmt, err := sqlparser.Parse(query)
	if err != nil {
		return fmt.Errorf("SQL gerado inválido: %w", err)
	}
	if _, ok := stmt.(*sqlparser.Select); !ok {
		return fmt.Errorf("apenas SELECT é permitido na query gerada pela IA")
	}
	if dangerousFunctionPattern.MatchString(query) {
		return fmt.Errorf("função perigosa detectada na query gerada pela IA")
	}
	return nil
}

func normalizeSQL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
		if strings.Contains(trimmed, "\n") {
			lines := strings.Split(trimmed, "\n")
			if len(lines) > 1 {
				trimmed = strings.Join(lines[1:], "\n")
			}
		}
	}
	return strings.TrimSpace(trimmed)
}

func addLimitIfMissing(rawSQL string, limitDefault int) (string, int, error) {
	q := strings.TrimSpace(rawSQL)
	if q == "" {
		return "", 0, fmt.Errorf("SQL vazio")
	}
	lower := strings.ToLower(q)
	if regexp.MustCompile(`(?i)\blimit\b`).MatchString(lower) {
		match := regexp.MustCompile(`(?i)\blimit\s+(\d+)\b`).FindStringSubmatch(q)
		if len(match) >= 2 {
			limit, err := strconv.Atoi(match[1])
			if err == nil {
				return q, limit, nil
			}
		}
		return q, 0, nil
	}
	if limitDefault == 0 {
		return q, 0, nil
	}
	return q + " LIMIT " + strconv.Itoa(limitDefault), limitDefault, nil
}
