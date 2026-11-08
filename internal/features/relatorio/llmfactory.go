package relatorio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/fake"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

func newLLM(cfg LLMConfig) (llms.Model, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		return nil, fmt.Errorf("LLM_PROVIDER não configurado")
	}
	if provider == "fake" {
		return fake.NewFakeLLM([]string{"SELECT 1 LIMIT 1"}), nil
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("LLM_MODEL obrigatório para o provedor %s", provider)
	}

	switch provider {
	case "groq", "openrouter":
		baseURL := strings.TrimSpace(cfg.BaseURL)
		if baseURL == "" {
			switch provider {
			case "groq":
				baseURL = "https://api.groq.com/openai/v1"
			case "openrouter":
				baseURL = "https://openrouter.ai/api/v1"
			}
		}
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, fmt.Errorf("LLM_API_KEY obrigatório para %s", provider)
		}
		options := []openai.Option{
			openai.WithModel(cfg.Model),
			openai.WithBaseURL(baseURL),
			openai.WithToken(cfg.APIKey),
		}
		if provider == "groq" {
			options = append(options, openai.WithHTTPClient(&groqReasoningClient{client: http.DefaultClient}))
		}
		return openai.New(options...)
	case "openai":
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, fmt.Errorf("LLM_API_KEY obrigatório para openai")
		}
		return openai.New(openai.WithModel(cfg.Model), openai.WithToken(cfg.APIKey))
	case "googleai":
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, fmt.Errorf("LLM_API_KEY obrigatório para googleai")
		}
		return googleai.New(context.Background(), googleai.WithAPIKey(cfg.APIKey), googleai.WithDefaultModel(cfg.Model))
	case "ollama":
		return ollama.New(ollama.WithModel(cfg.Model), ollama.WithServerURL(ollamaServerURL(cfg.BaseURL)))
	default:
		return nil, fmt.Errorf("provedor de LLM desconhecido: %s", provider)
	}
}

type groqReasoningClient struct {
	client *http.Client
}

func (c *groqReasoningClient) Do(request *http.Request) (*http.Response, error) {
	if request.URL.Path != "/openai/v1/chat/completions" && !strings.HasSuffix(request.URL.Path, "/chat/completions") {
		return c.client.Do(request)
	}
	if request.Body == nil {
		return nil, fmt.Errorf("corpo ausente na requisição de chat do Groq")
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, fmt.Errorf("ler requisição de chat do Groq: %w", err)
	}
	if err := request.Body.Close(); err != nil {
		return nil, fmt.Errorf("fechar corpo da requisição de chat do Groq: %w", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decodificar requisição de chat do Groq: %w", err)
	}
	reasoningEffort, err := json.Marshal("low")
	if err != nil {
		return nil, fmt.Errorf("codificar reasoning_effort do Groq: %w", err)
	}
	payload["reasoning_effort"] = reasoningEffort
	body, err = json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("codificar requisição de chat do Groq: %w", err)
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	request.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	return c.client.Do(request)
}

func ollamaServerURL(configured string) string {
	if baseURL := strings.TrimSpace(configured); baseURL != "" {
		return baseURL
	}
	return "http://localhost:11434"
}
