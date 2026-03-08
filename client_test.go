package langfuse

import (
	"context"
	"encoding/json"
	"math"
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

func TestClient_doRequest_ContextCancelled(t *testing.T) {
	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   "http://localhost:1", // won't actually connect
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = client.doRequest(ctx, "GET", "/test", nil)
	if err == nil {
		t.Error("doRequest() expected error for cancelled context, got nil")
	}
}

func TestClient_doRequest_MarshalError(t *testing.T) {
	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// math.Inf cannot be marshaled to JSON
	body := map[string]interface{}{
		"bad": math.Inf(1),
	}
	_, err = client.doRequest(context.Background(), "POST", "/test", body)
	if err == nil {
		t.Error("doRequest() expected marshal error, got nil")
	}
}

func TestClient_doRequest_LoggerOnHTTPError(t *testing.T) {
	// Create a server and immediately close it so connection fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	logger := &captureLogger{}
	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   serverURL,
		Logger:    logger,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.doRequest(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Error("doRequest() expected error for closed server, got nil")
	}
	// Logger should have been called with error (status code 0)
	if len(logger.responses) == 0 {
		t.Error("Logger.LogResponse was not called on HTTP error")
	}
	if logger.responses[0] != 0 {
		t.Errorf("Logger.LogResponse status = %d, want 0", logger.responses[0])
	}
}

func TestClient_handleResponse_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.doRequest(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("doRequest() error = %v", err)
	}

	var result map[string]string
	err = client.handleResponse(resp, &result, false)
	if err == nil {
		t.Error("handleResponse() expected decode error for malformed JSON, got nil")
	}
	// Should NOT be an APIError since status was 200
	if IsAPIError(err) {
		t.Error("handleResponse() error should not be APIError for decode failure")
	}
}

func TestClient_CreateTrace_AutoID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(TraceResponse{ID: ""})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.CreateTrace(context.Background(), Trace{Name: "auto-id"})
	if err != nil {
		t.Fatalf("CreateTrace() error = %v", err)
	}
	if resp.ID == "" {
		t.Error("CreateTrace() should use auto-generated ID when server returns empty")
	}
}

func TestClient_CreateSpan_AutoID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(SpanResponse{ID: ""})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.CreateSpan(context.Background(), Span{Name: "auto-id"})
	if err != nil {
		t.Fatalf("CreateSpan() error = %v", err)
	}
	if resp.ID == "" {
		t.Error("CreateSpan() should use auto-generated ID when server returns empty")
	}
}

func TestClient_CreateGeneration_AutoID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(GenerationResponse{ID: ""})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.CreateGeneration(context.Background(), Generation{Name: "auto-id"})
	if err != nil {
		t.Fatalf("CreateGeneration() error = %v", err)
	}
	if resp.ID == "" {
		t.Error("CreateGeneration() should use auto-generated ID when server returns empty")
	}
}

func TestClient_CreateEvent_AutoID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(EventResponse{ID: ""})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.CreateEvent(context.Background(), Event{Name: "auto-id"})
	if err != nil {
		t.Fatalf("CreateEvent() error = %v", err)
	}
	if resp.ID == "" {
		t.Error("CreateEvent() should use auto-generated ID when server returns empty")
	}
}

func TestClient_CreateEvent_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid"}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.CreateEvent(context.Background(), Event{Name: "test"})
	if err == nil {
		t.Error("CreateEvent() expected error, got nil")
	}
	if !IsAPIError(err) {
		t.Error("CreateEvent() expected APIError")
	}
}

func TestClient_Score_AutoID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ScoreResponse{ID: ""})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.Score(context.Background(), Score{Name: "test-score", TraceID: "t1", Value: 0.5})
	if err != nil {
		t.Fatalf("Score() error = %v", err)
	}
	if resp.ID == "" {
		t.Error("Score() should use auto-generated ID when server returns empty")
	}
}

func TestClient_Score_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal"}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Score(context.Background(), Score{Name: "test-score", TraceID: "t1", Value: 0.5})
	if err == nil {
		t.Error("Score() expected error, got nil")
	}
	if !IsAPIError(err) {
		t.Error("Score() expected APIError")
	}
}

func TestClient_UpdateTrace_AutoID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(TraceResponse{ID: ""})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	name := "updated"
	resp, err := client.UpdateTrace(context.Background(), "trace-abc", TraceUpdate{Name: &name})
	if err != nil {
		t.Fatalf("UpdateTrace() error = %v", err)
	}
	if resp.ID != "trace-abc" {
		t.Errorf("UpdateTrace() ID = %v, want trace-abc", resp.ID)
	}
}

func TestClient_UpdateSpan_AutoID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(SpanResponse{ID: ""})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.UpdateSpan(context.Background(), "span-abc", SpanUpdate{Output: "data"})
	if err != nil {
		t.Fatalf("UpdateSpan() error = %v", err)
	}
	if resp.ID != "span-abc" {
		t.Errorf("UpdateSpan() ID = %v, want span-abc", resp.ID)
	}
}

func TestClient_UpdateGeneration_AutoID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(GenerationResponse{ID: ""})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.UpdateGeneration(context.Background(), "gen-abc", GenerationUpdate{Output: "data"})
	if err != nil {
		t.Fatalf("UpdateGeneration() error = %v", err)
	}
	if resp.ID != "gen-abc" {
		t.Errorf("UpdateGeneration() ID = %v, want gen-abc", resp.ID)
	}
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
