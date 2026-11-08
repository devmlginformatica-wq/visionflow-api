package relatorio

import (
	"strconv"
	"strings"
	"time"
)

type RelatorioRequest struct {
	Pergunta string `json:"pergunta"`
}

type EstatisticaNumerica struct {
	Soma         float64 `json:"soma"`
	Media        float64 `json:"media"`
	Minimo       float64 `json:"minimo"`
	Maximo       float64 `json:"maximo"`
	DesvioPadrao float64 `json:"desvio_padrao"`
}

type Estatisticas struct {
	TotalRegistros   int                            `json:"total_registros"`
	ColunasNumericas map[string]EstatisticaNumerica `json:"colunas_numericas"`
}

type GraficoSugerido struct {
	Tipo  string `json:"tipo"`
	EixoX string `json:"eixo_x"`
	EixoY string `json:"eixo_y"`
}

type RelatorioResponse struct {
	Colunas         []string         `json:"colunas"`
	Linhas          [][]any          `json:"linhas"`
	SQLGerado       string           `json:"sql_gerado"`
	TempoMS         int64            `json:"tempo_ms"`
	ProvedorLLM     string           `json:"provedor_llm"`
	LimiteAplicado  int              `json:"limite_aplicado"`
	Estatisticas    Estatisticas     `json:"estatisticas"`
	GraficoSugerido *GraficoSugerido `json:"grafico_sugerido,omitempty"`
}

func newEstatisticas(colunas []string, linhas [][]any) Estatisticas {
	stats := Estatisticas{TotalRegistros: len(linhas), ColunasNumericas: map[string]EstatisticaNumerica{}}
	for idx, coluna := range colunas {
		numeric, ok := calcularEstatisticaNumerica(idx, linhas)
		if ok {
			stats.ColunasNumericas[coluna] = numeric
		}
	}
	return stats
}

func toHistogramValues(rows [][]any, idx int) []float64 {
	values := make([]float64, 0, len(rows))
	for _, row := range rows {
		if idx >= len(row) || row[idx] == nil {
			continue
		}
		switch v := row[idx].(type) {
		case int:
			values = append(values, float64(v))
		case int8:
			values = append(values, float64(v))
		case int16:
			values = append(values, float64(v))
		case int32:
			values = append(values, float64(v))
		case int64:
			values = append(values, float64(v))
		case float32:
			values = append(values, float64(v))
		case float64:
			values = append(values, v)
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err == nil {
				values = append(values, parsed)
			}
		case time.Time:
			values = append(values, float64(v.Unix()))
		}
	}
	return values
}
