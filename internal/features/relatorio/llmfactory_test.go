package relatorio

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

func TestNewLLM(t *testing.T) {
	t.Run("fake provider", func(t *testing.T) {
		model, err := newLLM(LLMConfig{Provider: "fake"})
		if err != nil {
			t.Fatalf("newLLM retornou erro: %v", err)
		}
		if _, ok := model.(llms.Model); !ok || model == nil {
			t.Fatal("esperava uma implementação de llms.Model")
		}
	})

	t.Run("unknown provider", func(t *testing.T) {
		_, err := newLLM(LLMConfig{Provider: "inexistente", Model: "modelo"})
		if err == nil || !strings.Contains(err.Error(), "provedor de LLM desconhecido") {
			t.Fatalf("esperava erro claro em português, recebeu %v", err)
		}
	})

	t.Run("groq requires API key", func(t *testing.T) {
		_, err := newLLM(LLMConfig{Provider: "groq", Model: "llama"})
		if err == nil || !strings.Contains(err.Error(), "LLM_API_KEY") {
			t.Fatalf("esperava erro indicando LLM_API_KEY, recebeu %v", err)
		}
	})

	t.Run("Groq reasoning effort", func(t *testing.T) {
		var got map[string]json.RawMessage
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ler corpo recebido: %v", err)
			}
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decodificar corpo recebido: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"test","object":"chat.completion","created":1,"model":"openai/gpt-oss-20b","choices":[{"index":0,"message":{"role":"assistant","content":"SELECT 1"},"finish_reason":"stop"}]}`)
		}))
		defer server.Close()

		model, err := newLLM(LLMConfig{
			Provider: "groq",
			Model:    "openai/gpt-oss-20b",
			BaseURL:  server.URL + "/openai/v1",
			APIKey:   "test-key",
		})
		if err != nil {
			t.Fatalf("newLLM retornou erro: %v", err)
		}
		response, err := model.GenerateContent(context.Background(), []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeHuman, "gere uma consulta"),
		}, llms.WithMaxTokens(4096))
		if err != nil {
			t.Fatalf("GenerateContent retornou erro: %v", err)
		}
		if response == nil || len(response.AssistantMessage().Parts) == 0 {
			t.Fatal("resposta do Groq inesperada")
		}
		var effort string
		if err := json.Unmarshal(got["reasoning_effort"], &effort); err != nil {
			t.Fatalf("reasoning_effort não foi enviado: %v", err)
		}
		if effort != "low" {
			t.Fatalf("reasoning_effort = %q, want %q", effort, "low")
		}
		var maxTokens int
		if err := json.Unmarshal(got["max_completion_tokens"], &maxTokens); err != nil {
			t.Fatalf("max_completion_tokens não foi enviado: %v", err)
		}
		if maxTokens != 4096 {
			t.Fatalf("max_completion_tokens = %d, want 4096", maxTokens)
		}
	})

	t.Run("ollama default URL", func(t *testing.T) {
		if got := ollamaServerURL(""); got != "http://localhost:11434" {
			t.Fatalf("URL padrão = %q", got)
		}
		model, err := newLLM(LLMConfig{Provider: "ollama", Model: "llama3"})
		if err != nil || model == nil {
			t.Fatalf("ollama sem URL configurada deve usar o padrão: model=%v err=%v", model, err)
		}
	})

	t.Run("empty provider", func(t *testing.T) {
		_, err := newLLM(LLMConfig{})
		if err == nil || !strings.Contains(err.Error(), "não configurado") {
			t.Fatalf("esperava erro de feature não configurada, recebeu %v", err)
		}
	})
}
