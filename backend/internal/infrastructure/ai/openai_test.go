package ai

import (
	"context"
	"encoding/json"
	"errors"
	"finance.local/amlens/internal/application"
	"finance.local/amlens/internal/config"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func settings() config.AI {
	return config.AI{Key: "unit-test-only-credential", Model: "test-model", BaseURL: "https://api.openai.com/v1", Timeout: time.Second}
}
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func envelope(text string) string {
	b, _ := json.Marshal(map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": text}}}}})
	return string(b)
}
func TestResponsesPayloadAndStrictOutput(t *testing.T) {
	calls := 0
	client := New(settings(), transport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://api.openai.com/v1/responses" || r.Header.Get("Authorization") != "Bearer "+settings().Key {
			t.Fatal("request")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["store"] != false || payload["model"] != "test-model" {
			t.Fatal("storage or model")
		}
		format := payload["text"].(map[string]any)["format"].(map[string]any)
		if format["strict"] != true || format["type"] != "json_schema" {
			t.Fatal("structured output missing")
		}
		input := payload["input"].([]any)[0].(map[string]any)
		if input["role"] != "user" || !strings.Contains(input["content"].(string), "Недоверенный вопрос") {
			t.Fatal("input placement")
		}
		return response(200, envelope(`{"answer":"Факты","referenced_gids":["1"]}`)), nil
	}))
	got, err := client.Answer(context.Background(), "Недоверенный вопрос", application.QuestionContext{})
	if err != nil || got.Answer != "Факты" || calls != 1 {
		t.Fatal(got, err, calls)
	}
}
func TestProviderFailuresAreBoundedAndSanitized(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"provider error", 401, "unit-test-only-credential"},
		{"redirect", 302, "redirect"},
		{"oversized", 200, strings.Repeat("x", (128<<10)+1)},
		{"incomplete", 200, `{"status":"incomplete","output":[]}`},
		{"bad json", 200, "invalid"},
		{"unknown field", 200, envelope(`{"answer":"a","referenced_gids":["1"],"extra":true}`)},
		{"multiple json values", 200, envelope(`{"answer":"a","referenced_gids":["1"]}{}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := New(settings(), transport(func(r *http.Request) (*http.Response, error) { calls++; return response(tc.status, tc.body), nil }))
			_, err := client.Answer(context.Background(), "q", application.QuestionContext{})
			if err == nil || strings.Contains(err.Error(), settings().Key) || calls != 1 {
				t.Fatal("unsanitized failure or retries", calls)
			}
		})
	}
	client := New(settings(), transport(func(r *http.Request) (*http.Response, error) { return nil, errors.New(settings().Key) }))
	_, err := client.Answer(context.Background(), "q", application.QuestionContext{})
	if err == nil || strings.Contains(err.Error(), settings().Key) {
		t.Fatal("transport leaked key")
	}
}
