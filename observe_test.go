package langfuse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newObserveServer creates a minimal httptest server that accepts any POST and
// returns a valid JSON ID response, used for Observe wrapper tests.
func newObserveServer(t *testing.T) (*httptest.Server, *Client) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"obs-id"}`))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return server, client
}

func TestObserve_BasicSpan(t *testing.T) {
	_, client := newObserveServer(t)

	fn := func(ctx context.Context) (string, error) {
		return "hello", nil
	}

	result, err := client.Observe(context.Background(), fn, &ObserveOptions{
		Name:          "test-fn",
		AsType:        ObservationTypeSpan,
		CaptureInput:  true,
		CaptureOutput: true,
	})
	if err != nil {
		t.Errorf("Observe() error = %v", err)
	}
	if result != "hello" {
		t.Errorf("Observe() result = %v, want hello", result)
	}
}

func TestObserve_WithError(t *testing.T) {
	_, client := newObserveServer(t)

	fn := func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("something failed")
	}

	_, err := client.Observe(context.Background(), fn, &ObserveOptions{
		Name:   "failing-fn",
		AsType: ObservationTypeSpan,
	})
	if err == nil {
		t.Error("Observe() expected error from wrapped function, got nil")
	}
	if err.Error() != "something failed" {
		t.Errorf("Observe() error = %v, want 'something failed'", err)
	}
}

func TestObserve_DefaultOptions(t *testing.T) {
	_, client := newObserveServer(t)

	fn := func(ctx context.Context) (string, error) {
		return "ok", nil
	}

	result, err := client.Observe(context.Background(), fn, nil)
	if err != nil {
		t.Errorf("Observe() error = %v", err)
	}
	if result != "ok" {
		t.Errorf("Observe() result = %v, want ok", result)
	}
}

func TestObserve_DefaultName(t *testing.T) {
	_, client := newObserveServer(t)

	fn := func(ctx context.Context) (string, error) {
		return "ok", nil
	}

	// No name set — should auto-derive from function type
	result, err := client.Observe(context.Background(), fn, &ObserveOptions{
		AsType: ObservationTypeSpan,
	})
	if err != nil {
		t.Errorf("Observe() error = %v", err)
	}
	if result != "ok" {
		t.Errorf("Observe() result = %v, want ok", result)
	}
}

func TestObserve_GenerationType(t *testing.T) {
	_, client := newObserveServer(t)

	fn := func(ctx context.Context) (string, error) {
		return "generated text", nil
	}

	result, err := client.Observe(context.Background(), fn, &ObserveOptions{
		Name:   "llm-call",
		AsType: ObservationTypeGeneration,
	})
	if err != nil {
		t.Errorf("Observe() error = %v", err)
	}
	if result != "generated text" {
		t.Errorf("Observe() result = %v, want 'generated text'", result)
	}
}

func TestObserve_NotAFunction(t *testing.T) {
	_, client := newObserveServer(t)

	_, err := client.Observe(context.Background(), "not-a-function", nil)
	if err == nil {
		t.Error("Observe() expected error for non-function input, got nil")
	}
}

func TestObserve_NoReturnValues(t *testing.T) {
	_, client := newObserveServer(t)

	fn := func(ctx context.Context) {}

	result, err := client.Observe(context.Background(), fn, &ObserveOptions{
		Name:   "no-return",
		AsType: ObservationTypeSpan,
	})
	if err != nil {
		t.Errorf("Observe() error = %v", err)
	}
	if result != nil {
		t.Errorf("Observe() result = %v, want nil", result)
	}
}

func TestGetFunctionName_Func(t *testing.T) {
	fn := func() {}
	name := getFunctionName(fn)
	if name == "" {
		t.Error("getFunctionName() should return non-empty string for a function")
	}
}

func TestGetFunctionName_NonFunc(t *testing.T) {
	name := getFunctionName("not-a-func")
	if name != "unknown-function" {
		t.Errorf("getFunctionName() for non-func = %v, want unknown-function", name)
	}
}

func TestObserve_GenerationWithError(t *testing.T) {
	_, client := newObserveServer(t)

	fn := func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("generation failed")
	}

	_, err := client.Observe(context.Background(), fn, &ObserveOptions{
		Name:          "gen-with-error",
		AsType:        ObservationTypeGeneration,
		CaptureInput:  true,
		CaptureOutput: true,
	})
	if err == nil {
		t.Error("Observe() expected error from wrapped function, got nil")
	}
	if err.Error() != "generation failed" {
		t.Errorf("Observe() error = %v, want 'generation failed'", err)
	}
}

func TestObserve_NoCaptureInput(t *testing.T) {
	_, client := newObserveServer(t)

	fn := func(ctx context.Context) (string, error) {
		return "result", nil
	}

	result, err := client.Observe(context.Background(), fn, &ObserveOptions{
		Name:          "no-input-capture",
		AsType:        ObservationTypeSpan,
		CaptureInput:  false,
		CaptureOutput: true,
	})
	if err != nil {
		t.Errorf("Observe() error = %v", err)
	}
	if result != "result" {
		t.Errorf("Observe() result = %v, want result", result)
	}
}

func TestObserve_NoCaptureOutput(t *testing.T) {
	_, client := newObserveServer(t)

	fn := func(ctx context.Context) (string, error) {
		return "result", nil
	}

	result, err := client.Observe(context.Background(), fn, &ObserveOptions{
		Name:          "no-output-capture",
		AsType:        ObservationTypeSpan,
		CaptureInput:  true,
		CaptureOutput: false,
	})
	if err != nil {
		t.Errorf("Observe() error = %v", err)
	}
	if result != "result" {
		t.Errorf("Observe() result = %v, want result", result)
	}
}
