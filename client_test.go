package langfuse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				PublicKey: "pk-test",
				SecretKey: "sk-test",
			},
			wantErr: false,
		},
		{
			name: "missing public key",
			config: Config{
				SecretKey: "sk-test",
			},
			wantErr: true,
		},
		{
			name: "missing secret key",
			config: Config{
				PublicKey: "pk-test",
			},
			wantErr: true,
		},
		{
			name: "custom base URL",
			config: Config{
				PublicKey: "pk-test",
				SecretKey: "sk-test",
				BaseURL:   "https://custom.langfuse.com",
			},
			wantErr: false,
		},
		{
			name: "custom HTTP client",
			config: Config{
				PublicKey:  "pk-test",
				SecretKey:  "sk-test",
				HTTPClient: &http.Client{Timeout: 10 * time.Second},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client without error")
			}
			if !tt.wantErr && tt.config.BaseURL != "" && client.baseURL != tt.config.BaseURL {
				t.Errorf("NewClient() baseURL = %v, want %v", client.baseURL, tt.config.BaseURL)
			}
		})
	}
}

func TestClient_CreateTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/traces" {
			t.Errorf("expected /api/public/traces, got %s", r.URL.Path)
		}

		// Check authentication
		username, password, ok := r.BasicAuth()
		if !ok || username != "pk-test" || password != "sk-test" {
			t.Error("missing or invalid basic auth")
		}

		var trace Trace
		if err := json.NewDecoder(r.Body).Decode(&trace); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if trace.Name != "test-trace" {
			t.Errorf("expected name 'test-trace', got %s", trace.Name)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(TraceResponse{ID: "trace-123"})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	now := time.Now()
	trace, err := client.CreateTrace(context.Background(), Trace{
		Name:      "test-trace",
		UserID:    "user-123",
		Timestamp: &now,
	})
	if err != nil {
		t.Fatalf("CreateTrace() error = %v", err)
	}
	if trace.ID != "trace-123" {
		t.Errorf("CreateTrace() ID = %v, want trace-123", trace.ID)
	}
}

func TestClient_CreateTrace_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid request"}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.CreateTrace(context.Background(), Trace{Name: "test"})
	if err == nil {
		t.Error("CreateTrace() expected error, got nil")
	}
	if !IsAPIError(err) {
		t.Error("CreateTrace() expected APIError")
	}
	apiErr := err.(*APIError)
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("CreateTrace() status code = %v, want %v", apiErr.StatusCode, http.StatusBadRequest)
	}
}

func TestClient_FlushAndShutdown(t *testing.T) {
	client, err := NewClient(Config{PublicKey: "pk-test", SecretKey: "sk-test"})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := client.Flush(); err != nil {
		t.Errorf("Flush() error = %v", err)
	}
	if err := client.Shutdown(); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestClient_Score_MissingName(t *testing.T) {
	client, err := NewClient(Config{PublicKey: "pk-test", SecretKey: "sk-test"})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.Score(context.Background(), Score{TraceID: "trace-1", Value: 1.0})
	if err == nil {
		t.Error("Score() expected error when name is missing, got nil")
	}
}

func TestClient_WithLogger(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(TraceResponse{ID: "trace-logged"})
	}))
	defer server.Close()

	logger := &captureLogger{}
	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
		Logger:    logger,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.CreateTrace(context.Background(), Trace{Name: "logged-trace"})
	if err != nil {
		t.Fatalf("CreateTrace() error = %v", err)
	}
	if len(logger.requests) == 0 {
		t.Error("Logger.LogRequest was not called")
	}
	if len(logger.responses) == 0 {
		t.Error("Logger.LogResponse was not called")
	}
}

// captureLogger records log calls for assertions.
type captureLogger struct {
	requests  []string
	responses []int
}

func (l *captureLogger) LogRequest(method, url string, body interface{}) {
	l.requests = append(l.requests, method+" "+url)
}

func (l *captureLogger) LogResponse(statusCode int, body []byte, err error) {
	l.responses = append(l.responses, statusCode)
}

func TestClient_UpdateTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// UpdateTrace uses POST with ID in body (upsert behavior)
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/traces" {
			t.Errorf("expected /api/public/traces, got %s", r.URL.Path)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Check that ID is included in the body
		if body["id"] != "trace-123" {
			t.Errorf("expected id 'trace-123' in body, got %v", body["id"])
		}

		if body["name"] != "updated-trace" {
			t.Errorf("expected name 'updated-trace', got %v", body["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(TraceResponse{ID: "trace-123"})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	name := "updated-trace"
	trace, err := client.UpdateTrace(context.Background(), "trace-123", TraceUpdate{
		Name: &name,
	})
	if err != nil {
		t.Fatalf("UpdateTrace() error = %v", err)
	}
	if trace.ID != "trace-123" {
		t.Errorf("UpdateTrace() ID = %v, want trace-123", trace.ID)
	}
}
