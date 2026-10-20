package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/celsolenonsander/visionflow-api/internal/service"
	"github.com/golang-jwt/jwt/v5"
)

func TestValidSearch(t *testing.T) {
	tests := []struct {
		name    string
		search  string
		wantErr bool
	}{
		{name: "empty search"},
		{name: "exactly 100 characters", search: strings.Repeat("a", 100)},
		{name: "100 unicode characters", search: strings.Repeat("é", 100)},
		{name: "over 100 characters", search: strings.Repeat("a", 101), wantErr: true},
		{name: "over 100 unicode characters", search: strings.Repeat("é", 101), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validSearch(test.search); (err != nil) != test.wantErr {
				t.Fatalf("validSearch() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{name: "empty", header: "", want: ""},
		{name: "missing scheme", header: "abc123", want: ""},
		{name: "valid", header: "Bearer abc123", want: "abc123"},
		{name: "case insensitive scheme", header: "bearer abc123", want: "abc123"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("Authorization", test.header)
			if got := extractBearerToken(request); got != test.want {
				t.Fatalf("extractBearerToken() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseToken(t *testing.T) {
	secret := "test-secret"
	handler := &Handler{JWTSecret: secret}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": 42,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	claims, err := handler.parseToken(signed)
	if err != nil {
		t.Fatalf("parseToken() error = %v", err)
	}
	if got := claims["sub"]; got != float64(42) {
		t.Fatalf("sub = %v, want 42", got)
	}

	wrongMethod := jwt.NewWithClaims(jwt.SigningMethodHS384, jwt.MapClaims{
		"sub": 42,
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	wrongSigned, err := wrongMethod.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign wrong-method token: %v", err)
	}
	if _, err := handler.parseToken(wrongSigned); err == nil {
		t.Fatal("parseToken() accepted a non-HS256 token")
	}
}

func TestWriteJSONAndLogoutEnvelope(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeJSON(recorder, http.StatusOK, nil, nil)

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["success"] != true || response["data"] != nil || response["error"] != nil {
		t.Fatalf("unexpected success envelope: %#v", response)
	}

	recorder = httptest.NewRecorder()
	(&Handler{}).Logout(recorder, httptest.NewRequest(http.MethodPost, "/auth/logout", nil))
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode logout response: %v", err)
	}
	if response["data"] != nil {
		t.Fatalf("logout data = %#v, want null", response["data"])
	}
}

func TestCursorValue(t *testing.T) {
	if got := cursorValue(""); got != nil {
		t.Fatalf("cursorValue(empty) = %#v, want nil", got)
	}
	if got := cursorValue("24129"); got != "24129" {
		t.Fatalf("cursorValue(nonempty) = %#v, want %q", got, "24129")
	}
}

func TestRoutesUsesConfiguredCORSOrigins(t *testing.T) {
	handler := &Handler{corsAllowedOrigins: []string{"https://app.example.test"}}
	request := httptest.NewRequest(http.MethodOptions, "/health", nil)
	request.Header.Set("Origin", "https://app.example.test")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	recorder := httptest.NewRecorder()

	handler.Routes().ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.test" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want configured origin", got)
	}
}

func TestWriteServiceErrorForInvalidInput(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeServiceError(recorder, service.InvalidInput("campo inválido"), "unexpected")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestProtectedRouteRejectsMissingOrInvalidToken(t *testing.T) {
	handler := (&Handler{}).Routes()
	tests := []struct {
		name       string
		authorize  string
		wantStatus int
		wantCode   string
	}{
		{name: "missing token", wantStatus: http.StatusUnauthorized, wantCode: "NAO_AUTENTICADO"},
		{name: "invalid token", authorize: "Bearer invalid", wantStatus: http.StatusUnauthorized, wantCode: "TOKEN_INVALIDO"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/clientes", nil)
			if test.authorize != "" {
				request.Header.Set("Authorization", test.authorize)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			var response struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Error.Code != test.wantCode {
				t.Fatalf("error code = %q, want %q", response.Error.Code, test.wantCode)
			}
		})
	}
}

func TestUpdateAddressRouteRequiresAuthentication(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/clientes/7/enderecos/15", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()

	(&Handler{}).Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("PUT /clientes/{id}/enderecos/{enderecoId} status = %d, want %d",
			recorder.Code, http.StatusUnauthorized)
	}
}

func TestDecodeRejectsInvalidRequestBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "unknown field", body: `{"email":"a@example.test","extra":true}`},
		{name: "multiple JSON values", body: `{"email":"a@example.test"} {}`},
		{name: "malformed JSON", body: `{"email":`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(test.body))
			recorder := httptest.NewRecorder()
			var payload struct {
				Email string `json:"email"`
			}

			if decode(recorder, request, &payload) {
				t.Fatal("decode() accepted an invalid request body")
			}
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestValidDevelopmentSQL(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		want      bool
	}{
		{name: "select", statement: "SELECT * FROM cliente", want: true},
		{name: "write", statement: "UPDATE cliente SET nome_fantasia='X'", want: true},
		{name: "cte", statement: "WITH rows AS (SELECT 1) SELECT * FROM rows", want: true},
		{name: "leading comments", statement: "/* teste */ -- query\nSELECT 1", want: true},
		{name: "empty", statement: "  -- somente comentário", want: false},
		{name: "ddl", statement: "DROP TABLE cliente", want: false},
		{name: "transaction command", statement: "COMMIT", want: false},
		{name: "change RLS context", statement: "SELECT set_config('app.cod_tenant','9',true)", want: false},
		{name: "change RLS context in cte", statement: "WITH x AS (SELECT set_config('app.cod_tenant','9',true)) SELECT * FROM cliente", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validDevelopmentSQL(test.statement); got != test.want {
				t.Fatalf("validDevelopmentSQL(%q) = %t, want %t", test.statement, got, test.want)
			}
		})
	}
}
