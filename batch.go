package langfuse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// BatchConfig holds configuration for the batch processor
type BatchConfig struct {
	// MaxBatchSize is the maximum number of events to batch together (default: 100)
	MaxBatchSize int
	// FlushInterval is how often to flush the batch (default: 5 seconds)
	FlushInterval time.Duration
	// MaxRetries is the maximum number of retries for failed requests (default: 3)
	MaxRetries int
	// RetryDelay is the initial delay between retries (default: 1 second, exponential backoff)
	RetryDelay time.Duration
	// QueueSize is the size of the event queue (default: 10000)
	QueueSize int
	// OnError is called when an error occurs during batch processing
	OnError func(err error, events []BatchEvent)
	// ShutdownTimeout is the maximum time to wait for pending events during shutdown (default: 30 seconds)
	ShutdownTimeout time.Duration
}

// DefaultBatchConfig returns the default batch configuration
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		MaxBatchSize:    100,
		FlushInterval:   5 * time.Second,
		MaxRetries:      3,
		RetryDelay:      1 * time.Second,
		QueueSize:       10000,
		ShutdownTimeout: 30 * time.Second,
	}
}

// BatchEventType represents the type of event in a batch
type BatchEventType string

const (
	BatchEventTypeTrace      BatchEventType = "trace-create"
	BatchEventTypeSpan       BatchEventType = "span-create"
	BatchEventTypeSpanUpdate BatchEventType = "span-update"
	BatchEventTypeGeneration BatchEventType = "generation-create"
	BatchEventTypeGenerationUpdate BatchEventType = "generation-update"
	BatchEventTypeEvent      BatchEventType = "event-create"
	BatchEventTypeScore      BatchEventType = "score-create"
)

// BatchEvent represents a single event in the batch
type BatchEvent struct {
	ID        string         `json:"id"`
	Type      BatchEventType `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Body      interface{}    `json:"body"`
}

// BatchRequest represents the batch API request
type BatchRequest struct {
	Batch    []BatchEvent      `json:"batch"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// BatchResponse represents the batch API response
type BatchResponse struct {
	Successes int      `json:"successes"`
	Errors    []string `json:"errors,omitempty"`
}

// BatchProcessor handles async batching of Langfuse events
type BatchProcessor struct {
	client     *Client
	config     BatchConfig
	eventQueue chan BatchEvent
	stopChan   chan struct{}
	doneChan   chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	running    bool
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(client *Client, config BatchConfig) *BatchProcessor {
	if config.MaxBatchSize <= 0 {
		config.MaxBatchSize = DefaultBatchConfig().MaxBatchSize
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = DefaultBatchConfig().FlushInterval
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = DefaultBatchConfig().MaxRetries
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = DefaultBatchConfig().RetryDelay
	}
	if config.QueueSize <= 0 {
		config.QueueSize = DefaultBatchConfig().QueueSize
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = DefaultBatchConfig().ShutdownTimeout
	}

	return &BatchProcessor{
		client:     client,
		config:     config,
		eventQueue: make(chan BatchEvent, config.QueueSize),
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
	}
}

// Start starts the batch processor
func (bp *BatchProcessor) Start() {
	bp.mu.Lock()
	if bp.running {
		bp.mu.Unlock()
		return
	}
	bp.running = true
	bp.mu.Unlock()

	bp.wg.Add(1)
	go bp.processLoop()
}

// Stop stops the batch processor and waits for pending events to be flushed
func (bp *BatchProcessor) Stop() error {
	bp.mu.Lock()
	if !bp.running {
		bp.mu.Unlock()
		return nil
	}
	bp.running = false
	bp.mu.Unlock()

	close(bp.stopChan)

	// Wait for processor to finish with timeout
	select {
	case <-bp.doneChan:
		return nil
	case <-time.After(bp.config.ShutdownTimeout):
		return fmt.Errorf("shutdown timeout: some events may not have been sent")
	}
}

// Enqueue adds an event to the batch queue
func (bp *BatchProcessor) Enqueue(event BatchEvent) error {
	bp.mu.Lock()
	if !bp.running {
		bp.mu.Unlock()
		return fmt.Errorf("batch processor is not running")
	}
	bp.mu.Unlock()

	if event.ID == "" {
		event.ID = generateID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	select {
	case bp.eventQueue <- event:
		return nil
	default:
		return fmt.Errorf("event queue is full")
	}
}

// EnqueueTrace enqueues a trace creation event
func (bp *BatchProcessor) EnqueueTrace(trace Trace) error {
	if trace.ID == "" {
		trace.ID = generateID()
	}
	return bp.Enqueue(BatchEvent{
		ID:   trace.ID,
		Type: BatchEventTypeTrace,
		Body: trace,
	})
}

// EnqueueSpan enqueues a span creation event
func (bp *BatchProcessor) EnqueueSpan(span Span) error {
	if span.ID == "" {
		span.ID = generateID()
	}
	return bp.Enqueue(BatchEvent{
		ID:   span.ID,
		Type: BatchEventTypeSpan,
		Body: span,
	})
}

// EnqueueSpanUpdate enqueues a span update event
func (bp *BatchProcessor) EnqueueSpanUpdate(spanID string, update SpanUpdate) error {
	body := spanUpsert{
		ID:            spanID,
		Name:          update.Name,
		EndTime:       update.EndTime,
		Metadata:      update.Metadata,
		Input:         update.Input,
		Output:        update.Output,
		Level:         update.Level,
		StatusMessage: update.StatusMessage,
	}
	return bp.Enqueue(BatchEvent{
		ID:   spanID,
		Type: BatchEventTypeSpanUpdate,
		Body: body,
	})
}

// EnqueueGeneration enqueues a generation creation event
func (bp *BatchProcessor) EnqueueGeneration(gen Generation) error {
	if gen.ID == "" {
		gen.ID = generateID()
	}
	return bp.Enqueue(BatchEvent{
		ID:   gen.ID,
		Type: BatchEventTypeGeneration,
		Body: gen,
	})
}

// EnqueueGenerationUpdate enqueues a generation update event
func (bp *BatchProcessor) EnqueueGenerationUpdate(genID string, update GenerationUpdate) error {
	body := generationUpsert{
		ID:              genID,
		Name:            update.Name,
		EndTime:         update.EndTime,
		Model:           update.Model,
		ModelParameters: update.ModelParameters,
		Input:           update.Input,
		Output:          update.Output,
		Metadata:        update.Metadata,
		Level:           update.Level,
		StatusMessage:   update.StatusMessage,
		Usage:           update.Usage,
		Prompt:          update.Prompt,
		Completion:      update.Completion,
	}
	return bp.Enqueue(BatchEvent{
		ID:   genID,
		Type: BatchEventTypeGenerationUpdate,
		Body: body,
	})
}

// EnqueueEvent enqueues an event creation
func (bp *BatchProcessor) EnqueueEvent(event Event) error {
	if event.ID == "" {
		event.ID = generateID()
	}
	return bp.Enqueue(BatchEvent{
		ID:   event.ID,
		Type: BatchEventTypeEvent,
		Body: event,
	})
}

// EnqueueScore enqueues a score creation event
func (bp *BatchProcessor) EnqueueScore(score Score) error {
	if score.ID == "" {
		score.ID = generateID()
	}
	return bp.Enqueue(BatchEvent{
		ID:   score.ID,
		Type: BatchEventTypeScore,
		Body: score,
	})
}

// QueueLength returns the current number of events in the queue
func (bp *BatchProcessor) QueueLength() int {
	return len(bp.eventQueue)
}

// Flush forces an immediate flush of events currently in the queue channel.
// Note: events already consumed by the background processLoop into its local
// batch buffer are not captured here. For a guaranteed full flush, use Stop().
func (bp *BatchProcessor) Flush() error {
	events := bp.drainQueue()
	if len(events) == 0 {
		return nil
	}
	return bp.sendBatch(events)
}

// processLoop is the main processing loop
func (bp *BatchProcessor) processLoop() {
	defer bp.wg.Done()
	defer close(bp.doneChan)

	ticker := time.NewTicker(bp.config.FlushInterval)
	defer ticker.Stop()

	batch := make([]BatchEvent, 0, bp.config.MaxBatchSize)

	for {
		select {
		case event := <-bp.eventQueue:
			batch = append(batch, event)
			if len(batch) >= bp.config.MaxBatchSize {
				bp.sendBatchWithRetry(batch)
				batch = make([]BatchEvent, 0, bp.config.MaxBatchSize)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				bp.sendBatchWithRetry(batch)
				batch = make([]BatchEvent, 0, bp.config.MaxBatchSize)
			}

		case <-bp.stopChan:
			// Drain remaining events from queue
			for {
				select {
				case event := <-bp.eventQueue:
					batch = append(batch, event)
					if len(batch) >= bp.config.MaxBatchSize {
						bp.sendBatchWithRetry(batch)
						batch = make([]BatchEvent, 0, bp.config.MaxBatchSize)
					}
				default:
					// No more events in queue
					if len(batch) > 0 {
						bp.sendBatchWithRetry(batch)
					}
					return
				}
			}
		}
	}
}

// drainQueue drains all events from the queue
func (bp *BatchProcessor) drainQueue() []BatchEvent {
	events := make([]BatchEvent, 0)
	for {
		select {
		case event := <-bp.eventQueue:
			events = append(events, event)
		default:
			return events
		}
	}
}

// sendBatchWithRetry sends a batch with retries
func (bp *BatchProcessor) sendBatchWithRetry(events []BatchEvent) {
	var err error
	delay := bp.config.RetryDelay

	for attempt := 0; attempt <= bp.config.MaxRetries; attempt++ {
		err = bp.sendBatch(events)
		if err == nil {
			return
		}

		// Check if error is retryable
		if apiErr, ok := err.(*APIError); ok {
			if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 && apiErr.StatusCode != 429 {
				// Client error (except rate limit), don't retry
				break
			}
		}

		if attempt < bp.config.MaxRetries {
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
		}
	}

	// All retries failed
	if bp.config.OnError != nil {
		bp.config.OnError(err, events)
	}
}

// sendBatch sends a batch of events to the API
func (bp *BatchProcessor) sendBatch(events []BatchEvent) error {
	if len(events) == 0 {
		return nil
	}

	request := BatchRequest{
		Batch: events,
		Metadata: map[string]string{
			"sdk_name":    "langfuse-go",
			"sdk_version": SDKVersion,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal batch request: %w", err)
	}

	// Construct URL
	base, err := url.Parse(bp.client.baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	joinedPath, err := url.JoinPath(base.Path, "api", "public", "ingestion")
	if err != nil {
		return fmt.Errorf("failed to join URL path: %w", err)
	}
	base.Path = joinedPath
	requestURL := base.String()

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(bp.client.publicKey, bp.client.secretKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("langfuse-go/%s", SDKVersion))

	// Log request if logger is set
	if bp.client.logger != nil {
		bp.client.logger.LogRequest("POST", requestURL, request)
	}

	resp, err := bp.client.httpClient.Do(req)
	if err != nil {
		if bp.client.logger != nil {
			bp.client.logger.LogResponse(0, nil, err)
		}
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Log response if logger is set
	if bp.client.logger != nil {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		bp.client.logger.LogResponse(resp.StatusCode, bodyBytes, nil)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusMultiStatus {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    "batch request failed",
			Body:       string(body),
		}
	}

	return nil
}

// AsyncClient wraps a Client with async batch processing
type AsyncClient struct {
	*Client
	batch *BatchProcessor
}

// NewAsyncClient creates a new async client with batch processing
func NewAsyncClient(config Config, batchConfig BatchConfig) (*AsyncClient, error) {
	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}

	batch := NewBatchProcessor(client, batchConfig)
	batch.Start()

	return &AsyncClient{
		Client: client,
		batch:  batch,
	}, nil
}

// CreateTraceAsync creates a trace asynchronously
func (ac *AsyncClient) CreateTraceAsync(trace Trace) (string, error) {
	if trace.ID == "" {
		trace.ID = generateID()
	}
	err := ac.batch.EnqueueTrace(trace)
	return trace.ID, err
}

// CreateSpanAsync creates a span asynchronously
func (ac *AsyncClient) CreateSpanAsync(span Span) (string, error) {
	if span.ID == "" {
		span.ID = generateID()
	}
	err := ac.batch.EnqueueSpan(span)
	return span.ID, err
}

// UpdateSpanAsync updates a span asynchronously
func (ac *AsyncClient) UpdateSpanAsync(spanID string, update SpanUpdate) error {
	return ac.batch.EnqueueSpanUpdate(spanID, update)
}

// CreateGenerationAsync creates a generation asynchronously
func (ac *AsyncClient) CreateGenerationAsync(gen Generation) (string, error) {
	if gen.ID == "" {
		gen.ID = generateID()
	}
	err := ac.batch.EnqueueGeneration(gen)
	return gen.ID, err
}

// UpdateGenerationAsync updates a generation asynchronously
func (ac *AsyncClient) UpdateGenerationAsync(genID string, update GenerationUpdate) error {
	return ac.batch.EnqueueGenerationUpdate(genID, update)
}

// CreateEventAsync creates an event asynchronously
func (ac *AsyncClient) CreateEventAsync(event Event) (string, error) {
	if event.ID == "" {
		event.ID = generateID()
	}
	err := ac.batch.EnqueueEvent(event)
	return event.ID, err
}

// ScoreAsync creates a score asynchronously
func (ac *AsyncClient) ScoreAsync(score Score) (string, error) {
	if score.ID == "" {
		score.ID = generateID()
	}
	err := ac.batch.EnqueueScore(score)
	return score.ID, err
}

// Flush flushes all pending events synchronously
func (ac *AsyncClient) Flush() error {
	return ac.batch.Flush()
}

// Shutdown gracefully shuts down the async client
func (ac *AsyncClient) Shutdown() error {
	return ac.batch.Stop()
}

// QueueLength returns the number of pending events
func (ac *AsyncClient) QueueLength() int {
	return ac.batch.QueueLength()
}

// BatchProcessor returns the underlying batch processor
func (ac *AsyncClient) BatchProcessor() *BatchProcessor {
	return ac.batch
}
