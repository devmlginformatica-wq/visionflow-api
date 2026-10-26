package service

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"
)

func TestPedidoAddressIDsRoundTrip(t *testing.T) {
	const tenantID, empresaID = 7, 3
	const customerERP = "CLI-42"
	addresses := []pedidoAddress{
		{
			id: 101, tenantID: tenantID, empresaID: empresaID, customerERP: customerERP,
			identification: sql.NullString{String: "FAT-ID", Valid: true}, erpCode: "ERP-FAT",
		},
		{
			id: 202, tenantID: tenantID, empresaID: empresaID, customerERP: customerERP,
			identification: sql.NullString{Valid: false}, erpCode: "ERP-DEL",
		},
		{
			id: 303, tenantID: tenantID, empresaID: empresaID, customerERP: customerERP,
			identification: sql.NullString{String: "", Valid: true}, erpCode: "ERP-COB",
		},
	}

	// PUT persists these codes, and the detail lookup must reverse them to the same IDs.
	persistedCodes := [3]*string{}
	for index, address := range addresses {
		code := pedidoAddressCode(address.identification, address.erpCode)
		persistedCodes[index] = &code
	}
	ids := pedidoAddressIDs(persistedCodes, addresses, tenantID, empresaID, customerERP)
	response, err := json.Marshal(map[string]*int{
		"endereco_faturamento_id": ids[0],
		"endereco_entrega_id":     ids[1],
		"endereco_cobranca_id":    ids[2],
	})
	if err != nil {
		t.Fatalf("marshal address IDs: %v", err)
	}
	const want = `{"endereco_cobranca_id":303,"endereco_entrega_id":202,"endereco_faturamento_id":101}`
	if string(response) != want {
		t.Fatalf("address IDs JSON = %s, want %s", response, want)
	}
	if *ids[1] == *ids[2] {
		t.Fatalf("delivery and billing IDs must differ, both are %d", *ids[1])
	}
}

func TestPedidoAddressIDsRejectsCrossScopeDeletedAndAmbiguousMatches(t *testing.T) {
	const tenantID, empresaID = 7, 3
	const customerERP = "CLI-42"
	code := "SHARED"
	addresses := []pedidoAddress{
		{
			id: 10, tenantID: tenantID, empresaID: empresaID, customerERP: customerERP,
			identification: sql.NullString{String: code, Valid: true}, erpCode: "ERP-10",
		},
		{
			id: 11, tenantID: tenantID, empresaID: empresaID, customerERP: "OTHER-CLIENT",
			identification: sql.NullString{String: code, Valid: true}, erpCode: "ERP-11",
		},
		{
			id: 12, tenantID: tenantID, empresaID: empresaID + 1, customerERP: customerERP,
			identification: sql.NullString{String: code, Valid: true}, erpCode: "ERP-12",
		},
		{
			id: 13, tenantID: tenantID + 1, empresaID: empresaID, customerERP: customerERP,
			identification: sql.NullString{String: code, Valid: true}, erpCode: "ERP-13",
		},
		{
			id: 14, tenantID: tenantID, empresaID: empresaID, customerERP: customerERP,
			identification: sql.NullString{String: code, Valid: true}, erpCode: "ERP-14", deleted: true,
		},
	}
	codes := [3]*string{&code, nil, nil}
	ids := pedidoAddressIDs(codes, addresses, tenantID, empresaID, customerERP)
	if ids[0] == nil || *ids[0] != 10 {
		t.Fatalf("resolved invoice address ID = %v, want 10", ids[0])
	}
	if ids[1] != nil || ids[2] != nil {
		t.Fatalf("unset address codes should resolve to null, got %v", ids)
	}

	addresses = append(addresses, pedidoAddress{
		id: 15, tenantID: tenantID, empresaID: empresaID, customerERP: customerERP,
		identification: sql.NullString{String: code, Valid: true}, erpCode: "ERP-15",
	})
	ids = pedidoAddressIDs(codes, addresses, tenantID, empresaID, customerERP)
	if ids[0] != nil {
		t.Fatalf("ambiguous address code resolved to ID %d, want null", *ids[0])
	}
}

func TestPedidoAddressCode(t *testing.T) {
	tests := []struct {
		name           string
		identification sql.NullString
		erpCode        string
		want           string
	}{
		{
			name:    "null identification uses ERP code",
			erpCode: "ERP-123",
			want:    "ERP-123",
		},
		{
			name:           "empty identification uses ERP code",
			identification: sql.NullString{String: "", Valid: true},
			erpCode:        "ERP-123",
			want:           "ERP-123",
		},
		{
			name:           "nonempty identification takes precedence",
			identification: sql.NullString{String: "ADDR-456", Valid: true},
			erpCode:        "ERP-123",
			want:           "ADDR-456",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := pedidoAddressCode(test.identification, test.erpCode); got != test.want {
				t.Fatalf("pedidoAddressCode() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestValidPage(t *testing.T) {
	tests := []struct {
		name       string
		limit      string
		cursor     string
		wantLimit  int
		wantCursor int
		wantErr    bool
	}{
		{name: "defaults", wantLimit: 20},
		{name: "valid values", limit: "50", cursor: "24129", wantLimit: 50, wantCursor: 24129},
		{name: "limit too large", limit: "101", wantErr: true},
		{name: "nonpositive cursor", cursor: "0", wantErr: true},
		{name: "malformed cursor", cursor: "x", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limit, cursor, err := validPage(test.limit, test.cursor)
			if (err != nil) != test.wantErr {
				t.Fatalf("validPage() error = %v, wantErr %t", err, test.wantErr)
			}
			if err == nil && (limit != test.wantLimit || cursor != test.wantCursor) {
				t.Fatalf("validPage() = (%d, %d), want (%d, %d)",
					limit, cursor, test.wantLimit, test.wantCursor)
			}
		})
	}
}

func TestParseDateTime(t *testing.T) {
	input := "2026-10-14T15:30:00-03:00"
	got, err := parseDateTime(input)
	if err != nil {
		t.Fatalf("parseDateTime(%q) error = %v", input, err)
	}
	want := time.Date(2026, 10, 14, 18, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("parseDateTime(%q) = %s, want %s", input, got, want)
	}

	if _, err := parseDateTime("2026-10-14T15:30:00"); err == nil {
		t.Fatal("parseDateTime accepted a datetime without timezone")
	}
}

func TestCalcularTotais(t *testing.T) {
	items := []map[string]any{
		{
			"product": map[string]any{
				"id": 88, "_volumes": 1, "_cubagem": 0.008,
				"peso_liquido": 2.0, "peso_bruto": 2.1,
			},
			"quantidade_informada":     10.0,
			"sale_price":               8.5,
			"valor_desconto_comercial": 5.0,
		},
	}
	totals := calcularTotais(items)
	if got := number(totals["valor_bruto"]); got != 85 {
		t.Fatalf("valor_bruto = %v, want 85", got)
	}
	if got := number(totals["valor_desconto"]); got != 5 {
		t.Fatalf("valor_desconto = %v, want 5", got)
	}
	if got := number(totals["valor_liquido"]); got != 80 {
		t.Fatalf("valor_liquido = %v, want 80", got)
	}
	if got := totals["quantidade_mix"]; got != 1 {
		t.Fatalf("quantidade_mix = %v, want 1", got)
	}
}

func TestEffectivePedidoSalePrice(t *testing.T) {
	tests := []struct {
		name      string
		requested float64
		official  float64
		want      float64
	}{
		{name: "caps request at effective catalog price", requested: 4.5, official: 4, want: 4},
		{name: "preserves lower negotiated price", requested: 3.5, official: 4, want: 3.5},
		{name: "preserves equal price", requested: 4, official: 4, want: 4},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := effectivePedidoSalePrice(test.requested, test.official); got != test.want {
				t.Fatalf("effectivePedidoSalePrice(%v, %v) = %v, want %v",
					test.requested, test.official, got, test.want)
			}
		})
	}
}
