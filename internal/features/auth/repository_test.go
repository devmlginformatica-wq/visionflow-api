package auth

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/celsolenonsander/visionflow-api/internal/models"
	"github.com/jackc/pgx/v5"
)

func TestAuthenticateUsesParameterizedCredentials(t *testing.T) {
	querier := &fakeRowQuerier{row: fakeRow{values: []any{42, "Vendedor", "user@example.test", "vendedor"}}}
	repository := Repository{db: querier}

	got, err := repository.Authenticate(context.Background(), "user@example.test", "not-a-real-password")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if want := (models.Usuario{ID: 42, Nome: "Vendedor", Email: "user@example.test", Tipo: "vendedor"}); got != want {
		t.Fatalf("Authenticate() = %#v, want %#v", got, want)
	}
	if !strings.Contains(querier.query, "u.tipo") {
		t.Fatalf("query does not load user profile: %s", querier.query)
	}
	if !strings.Contains(querier.query, "crypt($2,u.senha_hash)=u.senha_hash") {
		t.Fatalf("query does not verify password hash: %s", querier.query)
	}
	if !reflect.DeepEqual(querier.args, []any{"user@example.test", "not-a-real-password"}) {
		t.Fatalf("query args = %#v, want parameterized credentials", querier.args)
	}
}

type fakeRowQuerier struct {
	query string
	args  []any
	row   fakeRow
}

func (q *fakeRowQuerier) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.query = query
	q.args = args
	return q.row
}

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(destinations ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, value := range r.values {
		switch destination := destinations[i].(type) {
		case *int:
			*destination = value.(int)
		case *string:
			*destination = value.(string)
		default:
			panic("unexpected scan destination")
		}
	}
	return nil
}
