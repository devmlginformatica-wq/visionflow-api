package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/celsolenonsander/visionflow-api/internal/models"
)

func TestLoginMapsMissingUserToInvalidCredentials(t *testing.T) {
	service := Service{repository: fakeLoginRepository{err: pgx.ErrNoRows}}
	if _, err := service.Login(context.Background(), "user@example.test", "not-a-real-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want invalid credentials", err)
	}
}

func TestLoginReturnsUser(t *testing.T) {
	want := models.Usuario{ID: 42, Nome: "Vendedor", Email: "user@example.test", Tipo: "vendedor"}
	service := Service{repository: fakeLoginRepository{user: want}}

	got, err := service.Login(context.Background(), want.Email, "not-a-real-password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if got != want {
		t.Fatalf("Login() = %#v, want %#v", got, want)
	}
}

func TestLoginWrapsRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	service := Service{repository: fakeLoginRepository{err: wantErr}}

	if _, err := service.Login(context.Background(), "user@example.test", "not-a-real-password"); !errors.Is(err, wantErr) {
		t.Fatalf("Login() error = %v, want wrapped repository error", err)
	}
}

type fakeLoginRepository struct {
	user models.Usuario
	err  error
}

func (r fakeLoginRepository) Authenticate(context.Context, string, string) (models.Usuario, error) {
	return r.user, r.err
}
