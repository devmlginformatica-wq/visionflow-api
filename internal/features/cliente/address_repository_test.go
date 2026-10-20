package cliente

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/celsolenonsander/visionflow-api/internal/models"
	"github.com/celsolenonsander/visionflow-api/internal/platform/apperrors"
)

func TestUpdateAddressPreservesIDAndListReturnsUpdatedAddress(t *testing.T) {
	tx := addressTestTx{
		customer: addressTestCustomer{id: 7, tenantID: 1, empresaID: 2, erp: "CLI-7"},
		addresses: []addressTestAddress{{
			id: 15, tenantID: 1, empresaID: 2, customerERP: "CLI-7", erp: "ERP-END-15",
			addressType: "E", standard: false, cep: "01000-000", street: "Rua Antiga",
			number: "10", neighborhood: "Centro", city: "São Paulo", state: "SP",
		}},
	}
	input := validAddressInput()
	input.Logradouro = "Avenida Paulista"
	input.EnderecoPadrao = true

	updated, err := NewRepository().UpdateAddress(context.Background(), &tx, 1, 2, 7, 15, input, "C")
	if err != nil {
		t.Fatalf("UpdateAddress() error = %v", err)
	}
	if id := updated["id"]; id != int32(15) {
		t.Fatalf("updated id = %#v, want 15", id)
	}
	if updated["codigo_erp"] != "ERP-END-15" {
		t.Fatalf("updated codigo_erp = %#v, want existing code", updated["codigo_erp"])
	}
	if updated["logradouro"] != "Avenida Paulista" || updated["tipo_endereco"] != "cobranca" {
		t.Fatalf("updated response = %#v, want updated address values", updated)
	}

	listed, err := NewRepository().ListAddresses(context.Background(), &tx, 1, 2, 7)
	if err != nil {
		t.Fatalf("ListAddresses() error = %v", err)
	}
	if len(listed) != 1 || listed[0]["id"] != int32(15) ||
		listed[0]["logradouro"] != "Avenida Paulista" || listed[0]["endereco_padrao"] != true {
		t.Fatalf("ListAddresses() = %#v, want the updated address with the original ID", listed)
	}
}

func TestUpdateAddressRejectsAddressOutsideCustomerOrTenant(t *testing.T) {
	tests := []struct {
		name       string
		address    addressTestAddress
		customer   addressTestCustomer
		customerID int
	}{
		{
			name:       "another customer",
			address:    addressTestAddress{id: 15, tenantID: 1, empresaID: 2, customerERP: "CLI-8", erp: "ERP-END-15"},
			customer:   addressTestCustomer{id: 7, tenantID: 1, empresaID: 2, erp: "CLI-7"},
			customerID: 7,
		},
		{
			name:       "another tenant",
			address:    addressTestAddress{id: 15, tenantID: 3, empresaID: 2, customerERP: "CLI-7", erp: "ERP-END-15"},
			customer:   addressTestCustomer{id: 7, tenantID: 1, empresaID: 2, erp: "CLI-7"},
			customerID: 7,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := addressTestTx{customer: test.customer, addresses: []addressTestAddress{test.address}}
			_, err := NewRepository().UpdateAddress(context.Background(), &tx, 1, 2, test.customerID, 15, validAddressInput(), "E")
			if !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("UpdateAddress() error = %v, want pgx.ErrNoRows", err)
			}
			if tx.addresses[0].street != "" {
				t.Fatalf("address outside scope was modified: %#v", tx.addresses[0])
			}
		})
	}
}

func TestUpdateAddressRejectsMissingAddress(t *testing.T) {
	tx := addressTestTx{customer: addressTestCustomer{id: 7, tenantID: 1, empresaID: 2, erp: "CLI-7"}}
	_, err := NewRepository().UpdateAddress(context.Background(), &tx, 1, 2, 7, 999, validAddressInput(), "E")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("UpdateAddress() error = %v, want pgx.ErrNoRows", err)
	}
}

func TestUpdateAddressRejectsCustomerInIntegration(t *testing.T) {
	tx := addressTestTx{
		customer: addressTestCustomer{
			id: 7, tenantID: 1, empresaID: 2, erp: "CLI-7", status: "em_integracao",
		},
		addresses: []addressTestAddress{{
			id: 15, tenantID: 1, empresaID: 2, customerERP: "CLI-7", erp: "ERP-END-15",
		}},
	}

	_, err := NewRepository().UpdateAddress(
		context.Background(), &tx, 1, 2, 7, 15, validAddressInput(), "E",
	)
	if !errors.Is(err, apperrors.ErrIntegrationLocked) {
		t.Fatalf("UpdateAddress() error = %v, want integration lock", err)
	}
	if tx.addresses[0].street != "" {
		t.Fatalf("locked customer's address was modified: %#v", tx.addresses[0])
	}
}

func TestUpdateAddressSwitchesPrimaryWithinCustomer(t *testing.T) {
	tx := addressTestTx{
		customer: addressTestCustomer{id: 7, tenantID: 1, empresaID: 2, erp: "CLI-7"},
		addresses: []addressTestAddress{
			{id: 15, tenantID: 1, empresaID: 2, customerERP: "CLI-7", erp: "ERP-END-15", standard: true},
			{id: 16, tenantID: 1, empresaID: 2, customerERP: "CLI-7", erp: "ERP-END-16"},
			{id: 17, tenantID: 1, empresaID: 2, customerERP: "CLI-OTHER", erp: "ERP-END-17", standard: true},
		},
	}
	input := validAddressInput()
	input.EnderecoPadrao = true

	if _, err := NewRepository().UpdateAddress(context.Background(), &tx, 1, 2, 7, 16, input, "E"); err != nil {
		t.Fatalf("UpdateAddress() error = %v", err)
	}

	standardCount := 0
	for _, address := range tx.addresses {
		if address.tenantID == 1 && address.empresaID == 2 && address.customerERP == "CLI-7" && address.standard {
			standardCount++
		}
	}
	if standardCount != 1 {
		t.Fatalf("standard address count for customer = %d, want exactly 1", standardCount)
	}
	if !tx.addresses[1].standard || !tx.addresses[2].standard {
		t.Fatal("new primary was not set or another customer's primary was cleared")
	}
}

func validAddressInput() models.EnderecoCreate {
	return models.EnderecoCreate{
		TipoEndereco: "entrega", CEP: "01310-100", Logradouro: "Avenida Paulista",
		NumeroEndereco: "1000", Bairro: "Bela Vista", Cidade: "São Paulo", Estado: "SP",
	}
}

type addressTestCustomer struct {
	id        int
	tenantID  int
	empresaID int
	erp       string
	status    string
}

type addressTestAddress struct {
	id           int
	tenantID     int
	empresaID    int
	customerERP  string
	erp          string
	addressType  string
	standard     bool
	cep          string
	street       string
	number       string
	complement   string
	neighborhood string
	city         string
	state        string
	reference    string
	restrictions string
}

type addressTestTx struct {
	pgx.Tx
	customer  addressTestCustomer
	addresses []addressTestAddress
}

func (tx *addressTestTx) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, "FROM cliente") && strings.Contains(query, "FOR UPDATE") &&
		tx.customer.id == args[2] && tx.customer.tenantID == args[0] && tx.customer.empresaID == args[1] {
		if strings.Contains(query, "situacao_app") {
			return addressTestRow{values: []any{tx.customer.status}}
		}
		return addressTestRow{values: []any{tx.customer.erp}}
	}
	return addressTestRow{err: pgx.ErrNoRows}
}

func (tx *addressTestTx) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	affected := int64(0)
	if strings.Contains(query, "SET cep=$1") {
		for index := range tx.addresses {
			address := &tx.addresses[index]
			if address.id != args[13] || address.tenantID != args[11] ||
				address.empresaID != args[12] || tx.customer.id != args[14] ||
				address.customerERP != tx.customer.erp {
				continue
			}
			address.cep = args[0].(string)
			address.street = args[1].(string)
			address.number = args[2].(string)
			address.complement = args[3].(string)
			address.neighborhood = args[4].(string)
			address.city = args[5].(string)
			address.state = args[6].(string)
			address.addressType = args[7].(string)
			address.standard = args[8].(bool)
			address.reference = args[9].(string)
			address.restrictions = args[10].(string)
			affected++
		}
	} else if strings.Contains(query, "SET endereco_padrao=false") {
		for index := range tx.addresses {
			address := &tx.addresses[index]
			if address.tenantID == args[0] && address.empresaID == args[1] &&
				tx.customer.id == args[2] && address.customerERP == tx.customer.erp &&
				address.id != args[3] {
				address.standard = false
				affected++
			}
		}
	}
	return pgconn.NewCommandTag("UPDATE " + strconv.FormatInt(affected, 10)), nil
}

func (tx *addressTestTx) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	customerID, tenantID, empresaID := 0, args[0].(int), args[1].(int)
	if strings.Contains(query, "FROM cliente_endereco_adicional e") {
		addressID := args[2].(int)
		customerID = args[3].(int)
		return tx.addressRows(tenantID, empresaID, customerID, addressID)
	}
	customerID = args[2].(int)
	return tx.addressRows(tenantID, empresaID, customerID, 0)
}

func (tx *addressTestTx) addressRows(tenantID, empresaID, customerID, addressID int) (pgx.Rows, error) {
	if tx.customer.id != customerID || tx.customer.tenantID != tenantID || tx.customer.empresaID != empresaID {
		return &addressTestRows{}, nil
	}
	rows := &addressTestRows{}
	for _, address := range tx.addresses {
		if address.tenantID != tenantID || address.empresaID != empresaID ||
			address.customerERP != tx.customer.erp || (addressID != 0 && address.id != addressID) {
			continue
		}
		rows.values = append(rows.values, []any{
			int32(address.id), address.erp, addressTypeName(address.addressType), address.standard,
			address.cep, address.street, address.number, address.complement, address.neighborhood,
			address.city, address.state, address.reference, address.restrictions,
		})
	}
	return rows, nil
}

type addressTestRow struct {
	values []any
	err    error
}

func (row addressTestRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}
	for index, value := range row.values {
		target := reflect.ValueOf(dest[index])
		target.Elem().Set(reflect.ValueOf(value))
	}
	return nil
}

type addressTestRows struct {
	values [][]any
	index  int
}

var addressTestFields = []string{
	"id", "codigo_erp", "tipo_endereco", "endereco_padrao", "cep", "logradouro",
	"numero_endereco", "complemento_endereco", "bairro", "cidade", "estado",
	"ponto_referencia", "restricoes_horarios",
}

func (rows *addressTestRows) Close() {}

func (rows *addressTestRows) Err() error { return nil }

func (rows *addressTestRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("SELECT")
}

func (rows *addressTestRows) FieldDescriptions() []pgconn.FieldDescription {
	fields := make([]pgconn.FieldDescription, len(addressTestFields))
	for index, name := range addressTestFields {
		fields[index].Name = name
	}
	return fields
}

func (rows *addressTestRows) Next() bool {
	if rows.index >= len(rows.values) {
		return false
	}
	rows.index++
	return true
}

func (rows *addressTestRows) Scan(dest ...any) error {
	for index, value := range rows.values[rows.index-1] {
		target := reflect.ValueOf(dest[index])
		if value == nil {
			target.Elem().Set(reflect.Zero(target.Elem().Type()))
		} else {
			target.Elem().Set(reflect.ValueOf(value))
		}
	}
	return nil
}

func (rows *addressTestRows) Values() ([]any, error) { return rows.values[rows.index-1], nil }

func (rows *addressTestRows) RawValues() [][]byte { return nil }

func (rows *addressTestRows) Conn() *pgx.Conn { return nil }

func (rows *addressTestRows) TypeMap() *pgtype.Map { return nil }

func addressTypeName(value string) string {
	switch value {
	case "C":
		return "cobranca"
	case "E":
		return "entrega"
	case "A":
		return "ambos"
	default:
		return ""
	}
}
