package relatorio

import "testing"

func TestParseLimitDefault(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "default", value: "", want: 1000},
		{name: "configured", value: "500", want: 500},
		{name: "disabled", value: "0", want: 0},
		{name: "negative", value: "-1", wantErr: true},
		{name: "not integer", value: "abc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RELATORIO_IA_LIMIT_DEFAULT", tt.value)
			got, err := parseLimitDefault()
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseLimitDefault() erro = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("parseLimitDefault() = %d, esperado %d", got, tt.want)
			}
		})
	}
}
