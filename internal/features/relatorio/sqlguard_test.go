package relatorio

import (
	"strings"
	"testing"
)

func TestValidateReadOnlySQLAcceptsSelectStatements(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{name: "select simples", sql: "SELECT id FROM cliente"},
		{name: "select com join e where", sql: "SELECT c.id FROM cliente c JOIN pedido p ON p.cliente_id = c.id WHERE c.cod_tenant = 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateReadOnlySQL(tt.sql); err != nil {
				t.Fatalf("esperava SELECT válido, recebeu erro: %v", err)
			}
		})
	}
}

func TestValidateReadOnlySQLRejectsWrites(t *testing.T) {
	tests := []struct {
		name   string
		sql    string
		reason string
	}{
		{name: "insert", sql: "INSERT INTO cliente(id) VALUES (1)", reason: "apenas SELECT"},
		{name: "update", sql: "UPDATE cliente SET id = 1", reason: "apenas SELECT"},
		{name: "delete", sql: "DELETE FROM cliente", reason: "apenas SELECT"},
		{name: "drop", sql: "DROP TABLE cliente", reason: "apenas SELECT"},
		{name: "alter", sql: "ALTER TABLE cliente ADD COLUMN x int", reason: "apenas SELECT"},
		{name: "truncate", sql: "TRUNCATE TABLE cliente", reason: "apenas SELECT"},
		{name: "multiple statements", sql: "SELECT 1; DROP TABLE cliente", reason: "múltiplas instruções"},
		{name: "dangerous sleep", sql: "SELECT pg_sleep(1)", reason: "função perigosa"},
		{name: "dangerous file read", sql: "SELECT pg_read_file('/etc/passwd')", reason: "função perigosa"},
		{name: "comment with extra semicolon", sql: "SELECT 1 -- comentário;\n; SELECT 2", reason: "múltiplas instruções"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReadOnlySQL(tt.sql)
			if err == nil {
				t.Fatal("esperava SQL rejeitado")
			}
			if !strings.Contains(err.Error(), tt.reason) {
				t.Fatalf("erro %q não contém o motivo esperado %q", err, tt.reason)
			}
		})
	}
}
