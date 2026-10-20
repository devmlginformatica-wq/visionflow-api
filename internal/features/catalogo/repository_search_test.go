package catalogo

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestProductSearchPredicate(t *testing.T) {
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
		{name: "description prefix search", search: "Café", wantIDs: []int{1}},
		{name: "case insensitive search", search: "cafe", wantIDs: []int{1}},
		{name: "accent insensitive search", search: "cafe torrado", wantIDs: []int{1}},
		{name: "ERP code search", search: "PROD-002", wantIDs: []int{2}},
		{name: "no matches", search: "missing-record"},
	}

	query := `SELECT i.id
		FROM (VALUES
			(1, 'PROD-001', 'Café Torrado Especial'),
			(2, 'PROD-002', 'Farinha de Trigo'),
			(3, 'PROD-003', 'Arroz Integral')
		) AS i(id, codigo_erp, descricao)
		WHERE ` + strings.ReplaceAll(productSearchPredicate, "$2", "$1") + `
		ORDER BY i.id
		LIMIT 20`

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows, err := conn.Query(context.Background(), query, test.search)
			if err != nil {
				t.Fatalf("execute product search predicate: %v", err)
			}
			defer rows.Close()

			gotIDs := make([]int, 0)
			for rows.Next() {
				var id int
				if err := rows.Scan(&id); err != nil {
					t.Fatalf("scan product id: %v", err)
				}
				gotIDs = append(gotIDs, id)
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("read product search rows: %v", err)
			}
			if len(gotIDs) != len(test.wantIDs) {
				t.Fatalf("product search IDs = %v, want %v", gotIDs, test.wantIDs)
			}
			for index := range gotIDs {
				if gotIDs[index] != test.wantIDs[index] {
					t.Fatalf("product search IDs = %v, want %v", gotIDs, test.wantIDs)
				}
			}
		})
	}
}
