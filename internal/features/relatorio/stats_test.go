package relatorio

import (
	"math"
	"testing"
)

func TestNewEstatisticasCalculatesNumericColumns(t *testing.T) {
	stats := newEstatisticas(
		[]string{"valor"},
		[][]any{{10}, {20}, {30}},
	)
	got := stats.ColunasNumericas["valor"]
	if got.Soma != 60 || got.Media != 20 || got.Minimo != 10 || got.Maximo != 30 {
		t.Fatalf("estatísticas inesperadas: %+v", got)
	}
	if math.Abs(got.DesvioPadrao-10) > 1e-9 {
		t.Fatalf("desvio padrão amostral = %v; esperado 10", got.DesvioPadrao)
	}
}

func TestNewEstatisticasInfersMostlyNumericMixedColumn(t *testing.T) {
	stats := newEstatisticas(
		[]string{"valor"},
		[][]any{{"10"}, {"20"}, {"30"}, {"40"}, {"50"}, {"texto"}},
	)
	if _, ok := stats.ColunasNumericas["valor"]; !ok {
		t.Fatal("coluna com mais de 80% de valores numéricos não foi inferida como numérica")
	}
}

func TestNewEstatisticasIgnoresTextColumns(t *testing.T) {
	stats := newEstatisticas([]string{"nome"}, [][]any{{"Ana"}, {"Bia"}})
	if len(stats.ColunasNumericas) != 0 {
		t.Fatalf("esperava mapa numérico vazio, recebeu %+v", stats.ColunasNumericas)
	}
}

func TestNewEstatisticasEmptyRows(t *testing.T) {
	stats := newEstatisticas([]string{"valor"}, nil)
	if stats.TotalRegistros != 0 || len(stats.ColunasNumericas) != 0 {
		t.Fatalf("estatísticas inesperadas para resultado vazio: %+v", stats)
	}
}

func TestRecomendarGrafico(t *testing.T) {
	tests := []struct {
		name    string
		columns []string
		rows    [][]any
		want    string
	}{
		{name: "texto e numérico", columns: []string{"cliente", "total"}, rows: [][]any{{"A", 10}}, want: "bar"},
		{name: "data e numérico", columns: []string{"data_pedido", "total"}, rows: [][]any{{"2026-01-01", 10}}, want: "line"},
		{name: "duas numéricas", columns: []string{"quantidade", "valor"}, rows: [][]any{{2, 10.5}}, want: "scatter"},
		{name: "somente texto", columns: []string{"cliente"}, rows: [][]any{{"A"}}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recomendarGrafico(tt.columns, tt.rows)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("esperava nil, recebeu %+v", got)
				}
				return
			}
			if got == nil || got.Tipo != tt.want {
				t.Fatalf("gráfico = %+v; tipo esperado %q", got, tt.want)
			}
		})
	}
}
