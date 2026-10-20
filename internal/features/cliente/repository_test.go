package cliente

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestClientSearchPredicate(t *testing.T) {
	databaseURL := os.Getenv("VISIONFLOW_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("VISIONFLOW_TEST_DATABASE_URL is not set")
	}

	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	tests := []struct {
		name    string
		search  string
		wantIDs []int
	}{
		{name: "empty search returns page candidates", wantIDs: []int{1, 2, 3}},
		{name: "prefix search", search: "Acme", wantIDs: []int{1}},
		{name: "case insensitive search", search: "acme", wantIDs: []int{1}},
		{name: "accent insensitive search", search: "jose", wantIDs: []int{2}},
		{name: "partial tax id search", search: "345678", wantIDs: []int{1}},
		{name: "ERP code search", search: "A-202", wantIDs: []int{2}},
		{name: "no matches", search: "missing-record"},
	}

	query := `SELECT c.id
		FROM (VALUES
			(1, 'Acme Industrial', 'Loja Centro', '12.345.678/0001-90', 'ERP-001'),
			(2, 'Distribuidora José', 'Mercado do José', '98.765.432/0001-10', 'A-202'),
			(3, 'Outro Comércio', 'Outro', '11.222.333/0001-44', 'ERP-003')
		) AS c(id, nome_razao_social, nome_fantasia, cnpj_cpf, codigo_erp)
		WHERE ` + strings.ReplaceAll(clientSearchPredicate, "$4", "$1") + `
		ORDER BY c.id
		LIMIT 20`

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows, err := conn.Query(context.Background(), query, test.search)
			if err != nil {
				t.Fatalf("execute client search predicate: %v", err)
			}
			defer rows.Close()

			gotIDs := make([]int, 0)
			for rows.Next() {
				var id int
				if err := rows.Scan(&id); err != nil {
					t.Fatalf("scan client id: %v", err)
				}
				gotIDs = append(gotIDs, id)
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("read client search rows: %v", err)
			}
			if len(gotIDs) != len(test.wantIDs) {
				t.Fatalf("client search IDs = %v, want %v", gotIDs, test.wantIDs)
			}
			for index := range gotIDs {
				if gotIDs[index] != test.wantIDs[index] {
					t.Fatalf("client search IDs = %v, want %v", gotIDs, test.wantIDs)
				}
			}
		})
	}
}
