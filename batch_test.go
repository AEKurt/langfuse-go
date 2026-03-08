package langfuse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultBatchConfig(t *testing.T) {
	config := DefaultBatchConfig()

	if config.MaxBatchSize != 100 {
		t.Errorf("MaxBatchSize = %d, want 100", config.MaxBatchSize)
	}
	if config.FlushInterval != 5*time.Second {
		t.Errorf("FlushInterval = %v, want 5s", config.FlushInterval)
	}
	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", config.MaxRetries)
	}
	if config.RetryDelay != 1*time.Second {
		t.Errorf("RetryDelay = %v, want 1s", config.RetryDelay)
	}
	if config.QueueSize != 10000 {
		t.Errorf("QueueSize = %d, want 10000", config.QueueSize)
	}
	if config.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s", config.ShutdownTimeout)
	}
}

func TestNewBatchProcessor(t *testing.T) {
	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
	})

	bp := NewBatchProcessor(client, BatchConfig{})

	// Should use defaults for zero values
	if bp.config.MaxBatchSize != 100 {
		t.Errorf("MaxBatchSize = %d, want 100", bp.config.MaxBatchSize)
	}
}

func TestBatchProcessor_StartStop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BatchResponse{Successes: 1})
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})

	bp := NewBatchProcessor(client, BatchConfig{
		FlushInterval:   100 * time.Millisecond,
		ShutdownTimeout: 5 * time.Second,
	})

	bp.Start()

	// Should be able to enqueue
	err := bp.EnqueueTrace(Trace{Name: "test"})
	if err != nil {
		t.Errorf("EnqueueTrace() error = %v", err)
	}

	// Stop should flush and return
	err = bp.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// Enqueue after stop should fail
	err = bp.EnqueueTrace(Trace{Name: "test"})
	if err == nil {
		t.Error("EnqueueTrace() after Stop() should return error")
	}
}

func TestBatchProcessor_Enqueue(t *testing.T) {
	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
	})

	bp := NewBatchProcessor(client, BatchConfig{
		QueueSize: 10,
	})
	// Note: We intentionally do NOT start the processor here.
	// This allows us to test that events are properly enqueued to the channel.
	// If we called bp.Start(), the background goroutine would immediately
	// consume events from the channel into its local batch, making QueueLength() return 0.

	// We need to manually set running=true to allow enqueueing without starting the goroutine
	bp.mu.Lock()
	bp.running = true
	bp.mu.Unlock()

	// Enqueue various event types
	tests := []struct {
		name    string
		enqueue func() error
	}{
		{"Trace", func() error { return bp.EnqueueTrace(Trace{Name: "test"}) }},
		{"Span", func() error { return bp.EnqueueSpan(Span{Name: "test"}) }},
		{"SpanUpdate", func() error { return bp.EnqueueSpanUpdate("span-1", SpanUpdate{}) }},
		{"Generation", func() error { return bp.EnqueueGeneration(Generation{Name: "test"}) }},
		{"GenerationUpdate", func() error { return bp.EnqueueGenerationUpdate("gen-1", GenerationUpdate{}) }},
		{"Event", func() error { return bp.EnqueueEvent(Event{Name: "test"}) }},
		{"Score", func() error { return bp.EnqueueScore(Score{Name: "test", Value: 1.0}) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.enqueue()
			if err != nil {
				t.Errorf("Enqueue%s() error = %v", tt.name, err)
			}
		})
	}

	if bp.QueueLength() == 0 {
		t.Error("QueueLength() = 0, want > 0")
	}

	// Verify exact count
	if bp.QueueLength() != 7 {
		t.Errorf("QueueLength() = %d, want 7", bp.QueueLength())
	}
}

func TestBatchProcessor_FlushOnBatchSize(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		var req BatchRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BatchResponse{Successes: len(req.Batch)})
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})

	bp := NewBatchProcessor(client, BatchConfig{
		MaxBatchSize:  5,
		FlushInterval: 1 * time.Hour, // Long interval, should flush on batch size
	})
	bp.Start()

	// Enqueue exactly MaxBatchSize events
	for i := 0; i < 5; i++ {
		_ = bp.EnqueueTrace(Trace{Name: "test"})
	}

	// Wait for batch to be sent
	time.Sleep(100 * time.Millisecond)

	_ = bp.Stop()

	if atomic.LoadInt32(&requestCount) < 1 {
		t.Error("Expected at least one batch request")
	}
}

func TestBatchProcessor_FlushOnInterval(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BatchResponse{Successes: 1})
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})

	bp := NewBatchProcessor(client, BatchConfig{
		MaxBatchSize:  100, // High batch size
		FlushInterval: 50 * time.Millisecond,
	})
	bp.Start()

	// Enqueue one event
	_ = bp.EnqueueTrace(Trace{Name: "test"})

	// Wait for interval flush
	time.Sleep(200 * time.Millisecond)

	_ = bp.Stop()

	if atomic.LoadInt32(&requestCount) < 1 {
		t.Error("Expected at least one batch request from interval flush")
	}
}

func TestBatchProcessor_ManualFlush(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BatchResponse{Successes: 1})
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})

	bp := NewBatchProcessor(client, BatchConfig{
		MaxBatchSize:  100,
		FlushInterval: 1 * time.Hour,
	})
	bp.Start()
	defer func() { _ = bp.Stop() }()

	// Enqueue events
	_ = bp.EnqueueTrace(Trace{Name: "test1"})
	_ = bp.EnqueueTrace(Trace{Name: "test2"})

	// Manual flush
	err := bp.Flush()
	if err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	if atomic.LoadInt32(&requestCount) < 1 {
		t.Error("Expected batch request from manual flush")
	}
}

func TestBatchProcessor_RetryOnError(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count < 3 {
			// Fail first 2 requests
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BatchResponse{Successes: 1})
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})

	bp := NewBatchProcessor(client, BatchConfig{
		MaxBatchSize:  1,
		FlushInterval: 50 * time.Millisecond,
		MaxRetries:    3,
		RetryDelay:    10 * time.Millisecond,
	})
	bp.Start()

	_ = bp.EnqueueTrace(Trace{Name: "test"})

	// Wait for retries
	time.Sleep(500 * time.Millisecond)

	_ = bp.Stop()

	if atomic.LoadInt32(&requestCount) < 3 {
		t.Errorf("Expected at least 3 requests (with retries), got %d", requestCount)
	}
}

func TestBatchProcessor_OnErrorCallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest) // 400 - won't retry
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})

	var errorCalled bool
	var errorMu sync.Mutex

	bp := NewBatchProcessor(client, BatchConfig{
		MaxBatchSize:  1,
		FlushInterval: 50 * time.Millisecond,
		MaxRetries:    1,
		RetryDelay:    10 * time.Millisecond,
		OnError: func(err error, events []BatchEvent) {
			errorMu.Lock()
			errorCalled = true
			errorMu.Unlock()
		},
	})
	bp.Start()

	_ = bp.EnqueueTrace(Trace{Name: "test"})

	time.Sleep(200 * time.Millisecond)

	_ = bp.Stop()

	errorMu.Lock()
	if !errorCalled {
		t.Error("OnError callback was not called")
	}
	errorMu.Unlock()
}

func TestBatchProcessor_QueueFull(t *testing.T) {
	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
	})

	bp := NewBatchProcessor(client, BatchConfig{
		QueueSize:     2,
		FlushInterval: 1 * time.Hour, // Don't flush
	})
	bp.Start()

	// Fill the queue
	_ = bp.EnqueueTrace(Trace{Name: "test1"})
	_ = bp.EnqueueTrace(Trace{Name: "test2"})

	// This should fail
	err := bp.EnqueueTrace(Trace{Name: "test3"})
	if err == nil {
		t.Error("Expected error when queue is full")
	}

	_ = bp.Stop()
}

func TestBatchProcessor_sendBatch_EmptyEvents(t *testing.T) {
	client, _ := NewClient(Config{PublicKey: "pk-test", SecretKey: "sk-test"})
	bp := NewBatchProcessor(client, BatchConfig{})

	err := bp.sendBatch([]BatchEvent{})
	if err != nil {
		t.Errorf("sendBatch(empty) error = %v, want nil", err)
	}
}

func TestBatchProcessor_sendBatch_WithLogger(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BatchResponse{Successes: 1})
	}))
	defer server.Close()

	logger := &captureLogger{}
	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
		Logger:    logger,
	})

	bp := NewBatchProcessor(client, BatchConfig{})
	err := bp.sendBatch([]BatchEvent{
		{ID: "e1", Type: BatchEventTypeTrace, Timestamp: time.Now(), Body: Trace{Name: "test"}},
	})
	if err != nil {
		t.Fatalf("sendBatch() error = %v", err)
	}
	if len(logger.requests) == 0 {
		t.Error("Logger.LogRequest was not called for batch request")
	}
	if len(logger.responses) == 0 {
		t.Error("Logger.LogResponse was not called for batch response")
	}
}

func TestBatchProcessor_sendBatch_LoggerOnHTTPError(t *testing.T) {
	// Closed server to trigger HTTP error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	logger := &captureLogger{}
	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   serverURL,
		Logger:    logger,
	})

	bp := NewBatchProcessor(client, BatchConfig{})
	err := bp.sendBatch([]BatchEvent{
		{ID: "e1", Type: BatchEventTypeTrace, Timestamp: time.Now(), Body: Trace{Name: "test"}},
	})
	if err == nil {
		t.Error("sendBatch() expected error for closed server, got nil")
	}
	if len(logger.responses) == 0 {
		t.Error("Logger.LogResponse was not called on HTTP error")
	}
	if logger.responses[0] != 0 {
		t.Errorf("Logger.LogResponse status = %d, want 0", logger.responses[0])
	}
}

func TestBatchProcessor_RetryOn429(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests) // 429
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BatchResponse{Successes: 1})
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})

	bp := NewBatchProcessor(client, BatchConfig{
		MaxRetries: 3,
		RetryDelay: 1 * time.Millisecond,
	})

	bp.sendBatchWithRetry([]BatchEvent{
		{ID: "e1", Type: BatchEventTypeTrace, Timestamp: time.Now(), Body: Trace{Name: "test"}},
	})

	if atomic.LoadInt32(&requestCount) < 2 {
		t.Errorf("Expected at least 2 requests (retry on 429), got %d", requestCount)
	}
}

func TestBatchProcessor_NoRetryOnClientError(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusBadRequest) // 400 — should not retry
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})

	var errorCalled bool
	bp := NewBatchProcessor(client, BatchConfig{
		MaxRetries: 3,
		RetryDelay: 1 * time.Millisecond,
		OnError: func(err error, events []BatchEvent) {
			errorCalled = true
		},
	})

	bp.sendBatchWithRetry([]BatchEvent{
		{ID: "e1", Type: BatchEventTypeTrace, Timestamp: time.Now(), Body: Trace{Name: "test"}},
	})

	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("Expected exactly 1 request (no retry on 400), got %d", requestCount)
	}
	if !errorCalled {
		t.Error("OnError callback was not called after non-retryable error")
	}
}

func TestBatchProcessor_AllRetriesFail_NoCallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // always 500
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})

	bp := NewBatchProcessor(client, BatchConfig{
		MaxRetries: 1,
		RetryDelay: 1 * time.Millisecond,
		// OnError is nil — should not panic
	})

	// Should not panic
	bp.sendBatchWithRetry([]BatchEvent{
		{ID: "e1", Type: BatchEventTypeTrace, Timestamp: time.Now(), Body: Trace{Name: "test"}},
	})
}

func TestBatchProcessor_sendBatch_MultiStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus) // 207
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})

	bp := NewBatchProcessor(client, BatchConfig{})
	err := bp.sendBatch([]BatchEvent{
		{ID: "e1", Type: BatchEventTypeTrace, Timestamp: time.Now(), Body: Trace{Name: "test"}},
	})
	if err != nil {
		t.Errorf("sendBatch() with 207 status error = %v, want nil", err)
	}
}

func TestAsyncClient(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BatchResponse{Successes: 1})
	}))
	defer server.Close()

	client, err := NewAsyncClient(
		Config{
			PublicKey: "pk-test",
			SecretKey: "sk-test",
			BaseURL:   server.URL,
		},
		BatchConfig{
			MaxBatchSize:  10,
			FlushInterval: 100 * time.Millisecond,
		},
	)
	if err != nil {
		t.Fatalf("NewAsyncClient() error = %v", err)
	}

	// Create trace
	traceID, err := client.CreateTraceAsync(Trace{Name: "test-trace"})
	if err != nil {
		t.Errorf("CreateTraceAsync() error = %v", err)
	}
	if traceID == "" {
		t.Error("CreateTraceAsync() returned empty ID")
	}

	// Create span
	spanID, err := client.CreateSpanAsync(Span{Name: "test-span", TraceID: traceID})
	if err != nil {
		t.Errorf("CreateSpanAsync() error = %v", err)
	}
	if spanID == "" {
		t.Error("CreateSpanAsync() returned empty ID")
	}

	// Update span
	err = client.UpdateSpanAsync(spanID, SpanUpdate{Output: "result"})
	if err != nil {
		t.Errorf("UpdateSpanAsync() error = %v", err)
	}

	// Create generation
	genID, err := client.CreateGenerationAsync(Generation{Name: "test-gen", TraceID: traceID})
	if err != nil {
		t.Errorf("CreateGenerationAsync() error = %v", err)
	}
	if genID == "" {
		t.Error("CreateGenerationAsync() returned empty ID")
	}

	// Update generation
	err = client.UpdateGenerationAsync(genID, GenerationUpdate{Output: "result"})
	if err != nil {
		t.Errorf("UpdateGenerationAsync() error = %v", err)
	}

	// Create event
	eventID, err := client.CreateEventAsync(Event{Name: "test-event", TraceID: traceID})
	if err != nil {
		t.Errorf("CreateEventAsync() error = %v", err)
	}
	if eventID == "" {
		t.Error("CreateEventAsync() returned empty ID")
	}

	// Create score
	scoreID, err := client.ScoreAsync(Score{Name: "test-score", TraceID: traceID, Value: 0.9})
	if err != nil {
		t.Errorf("ScoreAsync() error = %v", err)
	}
	if scoreID == "" {
		t.Error("ScoreAsync() returned empty ID")
	}

	// Check queue length (may have already flushed)
	_ = client.QueueLength()

	// Flush
	err = client.Flush()
	if err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	// Shutdown
	err = client.Shutdown()
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}

	// Verify requests were made
	if atomic.LoadInt32(&requestCount) == 0 {
		t.Error("Expected at least one batch request")
	}
}

func TestBatchProcessor_StartAlreadyRunning(t *testing.T) {
	client, _ := NewClient(Config{PublicKey: "pk-test", SecretKey: "sk-test"})
	bp := NewBatchProcessor(client, BatchConfig{FlushInterval: 1 * time.Hour})

	bp.Start()
	bp.Start() // second call should be a no-op

	_ = bp.Stop()
}

func TestBatchProcessor_StopAlreadyStopped(t *testing.T) {
	client, _ := NewClient(Config{PublicKey: "pk-test", SecretKey: "sk-test"})
	bp := NewBatchProcessor(client, BatchConfig{FlushInterval: 1 * time.Hour})

	// Never started — Stop should return nil immediately
	err := bp.Stop()
	if err != nil {
		t.Errorf("Stop() on unstarted processor error = %v", err)
	}
}

func TestBatchProcessor_FlushEmpty(t *testing.T) {
	client, _ := NewClient(Config{PublicKey: "pk-test", SecretKey: "sk-test"})
	bp := NewBatchProcessor(client, BatchConfig{})
	bp.Start()
	defer func() { _ = bp.Stop() }()

	// Flushing an empty queue should be a no-op
	err := bp.Flush()
	if err != nil {
		t.Errorf("Flush() on empty queue error = %v", err)
	}
}

func TestNewAsyncClient_InvalidConfig(t *testing.T) {
	_, err := NewAsyncClient(Config{}, BatchConfig{})
	if err == nil {
		t.Error("NewAsyncClient() with empty Config should return error")
	}
}

func TestAsyncClient_BatchProcessorAccessor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BatchResponse{Successes: 1})
	}))
	defer server.Close()

	ac, err := NewAsyncClient(
		Config{PublicKey: "pk-test", SecretKey: "sk-test", BaseURL: server.URL},
		BatchConfig{},
	)
	if err != nil {
		t.Fatalf("NewAsyncClient() error = %v", err)
	}
	defer func() { _ = ac.Shutdown() }()

	bp := ac.BatchProcessor()
	if bp == nil {
		t.Error("BatchProcessor() should return non-nil BatchProcessor")
	}
}

func TestAsyncClient_ConcurrentWrites(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		var req BatchRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BatchResponse{Successes: len(req.Batch)})
	}))
	defer server.Close()

	client, err := NewAsyncClient(
		Config{
			PublicKey: "pk-test",
			SecretKey: "sk-test",
			BaseURL:   server.URL,
		},
		BatchConfig{
			MaxBatchSize:  50,
			FlushInterval: 100 * time.Millisecond,
			QueueSize:     1000,
		},
	)
	if err != nil {
		t.Fatalf("NewAsyncClient() error = %v", err)
	}

	// Concurrent writes
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = client.CreateTraceAsync(Trace{Name: "concurrent-trace"})
		}(i)
	}
	wg.Wait()

	// Flush and shutdown
	_ = client.Flush()
	_ = client.Shutdown()

	// Should have processed all events
	if atomic.LoadInt32(&requestCount) == 0 {
		t.Error("Expected batch requests from concurrent writes")
	}
}
