package relatorio

import (
	"math"
	"strconv"
	"strings"
)

func calcularEstatisticaNumerica(index int, linhas [][]any) (EstatisticaNumerica, bool) {
	values := make([]float64, 0, len(linhas))
	nonNullCount := 0
	for _, row := range linhas {
		if index >= len(row) || row[index] == nil {
			continue
		}
		nonNullCount++
		if value, ok := numericValue(row[index]); ok {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return EstatisticaNumerica{}, false
	}
	if nonNullCount > 0 && float64(len(values))/float64(nonNullCount) <= 0.8 {
		return EstatisticaNumerica{}, false
	}

	var soma, media, minimo, maximo float64
	minimo = values[0]
	maximo = values[0]
	for _, value := range values {
		soma += value
		if value < minimo {
			minimo = value
		}
		if value > maximo {
			maximo = value
		}
	}
	media = soma / float64(len(values))
	var variancia float64
	for _, value := range values {
		delta := value - media
		variancia += delta * delta
	}
	variancia /= float64(len(values) - 1)
	if len(values) <= 1 {
		variancia = 0
	}
	return EstatisticaNumerica{
		Soma:         soma,
		Media:        media,
		Minimo:       minimo,
		Maximo:       maximo,
		DesvioPadrao: math.Sqrt(variancia),
	}, true
}

func recomendarGrafico(colunas []string, linhas [][]any) *GraficoSugerido {
	if len(colunas) == 0 || len(linhas) == 0 {
		return nil
	}

	var numericIndexes, dateIndexes, textIndexes []int
	for index, column := range colunas {
		if isNumericColumn(index, linhas) {
			numericIndexes = append(numericIndexes, index)
		} else if isDateColumn(column) {
			dateIndexes = append(dateIndexes, index)
		} else {
			textIndexes = append(textIndexes, index)
		}
	}

	if len(dateIndexes) > 0 && len(numericIndexes) > 0 {
		return &GraficoSugerido{
			Tipo:  "line",
			EixoX: colunas[dateIndexes[0]],
			EixoY: colunas[numericIndexes[0]],
		}
	}
	if len(textIndexes) > 0 && len(numericIndexes) > 0 {
		return &GraficoSugerido{
			Tipo:  "bar",
			EixoX: colunas[textIndexes[0]],
			EixoY: colunas[numericIndexes[0]],
		}
	}
	if len(numericIndexes) >= 2 {
		return &GraficoSugerido{
			Tipo:  "scatter",
			EixoX: colunas[numericIndexes[0]],
			EixoY: colunas[numericIndexes[1]],
		}
	}
	return nil
}

func isNumericColumn(index int, rows [][]any) bool {
	nonNullCount := 0
	numericCount := 0
	for _, row := range rows {
		if index >= len(row) || row[index] == nil {
			continue
		}
		nonNullCount++
		if _, ok := numericValue(row[index]); ok {
			numericCount++
		}
	}
	return nonNullCount > 0 && float64(numericCount)/float64(nonNullCount) > 0.8
}

func isDateColumn(column string) bool {
	name := strings.ToLower(column)
	for _, marker := range []string{"data", "dt_", "mes", "ano", "periodo"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func numericValue(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}
