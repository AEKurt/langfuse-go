package langfuse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	// SDKVersion is the version of this SDK
	SDKVersion = "0.1.4"
	// DefaultBaseURL is the default Langfuse API base URL
	DefaultBaseURL = "https://cloud.langfuse.com"
	// DefaultTimeout is the default HTTP client timeout
	DefaultTimeout = 30 * time.Second
)

// Client represents the Langfuse client
// WARNING: Do not log or expose the secretKey field as it contains sensitive credentials.
type Client struct {
	baseURL    string
	publicKey  string
	secretKey  string
	httpClient *http.Client
	logger     Logger
}

// Logger is an interface for logging requests and responses
type Logger interface {
	LogRequest(method, url string, body interface{})
	LogResponse(statusCode int, body []byte, err error)
}

// Config holds the configuration for the Langfuse client
type Config struct {
	PublicKey  string
	SecretKey  string
	BaseURL    string // Optional, defaults to https://cloud.langfuse.com
	HTTPClient *http.Client
	Logger     Logger // Optional logger for debugging
}

// NewClient creates a new Langfuse client
func NewClient(config Config) (*Client, error) {
	if config.PublicKey == "" {
		return nil, fmt.Errorf("public key is required")
	}
	if config.SecretKey == "" {
		return nil, fmt.Errorf("secret key is required")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	// Validate base URL
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: DefaultTimeout,
		}
	}

	return &Client{
		baseURL:    baseURL,
		publicKey:  config.PublicKey,
		secretKey:  config.SecretKey,
		httpClient: httpClient,
		logger:     config.Logger,
	}, nil
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	// Construct URL safely
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	// Remove leading slash from endpoint if present (url.JoinPath handles paths)
	if endpoint != "" && endpoint[0] == '/' {
		endpoint = endpoint[1:]
	}
	// Use url.JoinPath for safe path construction (Go 1.19+)
	joinedPath, err := url.JoinPath(base.Path, "api", "public", endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to join URL path: %w", err)
	}
	base.Path = joinedPath
	requestURL := base.String()

	var reqBody io.Reader
	var jsonData []byte
	if body != nil {
		var err error
		jsonData, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.publicKey, c.secretKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("langfuse-go/%s", SDKVersion))

	// Log request if logger is set
	if c.logger != nil {
		c.logger.LogRequest(method, requestURL, body)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.LogResponse(0, nil, err)
		}
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	// Log response if logger is set
	if c.logger != nil {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		c.logger.LogResponse(resp.StatusCode, bodyBytes, nil)
	}

	return resp, nil
}

// handleResponse handles HTTP response decoding and error checking
func (c *Client) handleResponse(resp *http.Response, v interface{}, allowCreated bool) error {
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && !(allowCreated && resp.StatusCode == http.StatusCreated) {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    "unexpected status code",
			Body:       string(body),
		}
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

// CreateTrace creates a new trace
func (c *Client) CreateTrace(ctx context.Context, trace Trace) (*TraceResponse, error) {
	// Auto-generate ID if not provided
	if trace.ID == "" {
		trace.ID = generateID()
	}
	resp, err := c.doRequest(ctx, "POST", "/traces", trace)
	if err != nil {
		return nil, err
	}

	var traceResp TraceResponse
	if err := c.handleResponse(resp, &traceResp, true); err != nil {
		return nil, err
	}

	// Return the ID we generated/used
	if traceResp.ID == "" {
		traceResp.ID = trace.ID
	}

	return &traceResp, nil
}

// CreateSpan creates a new span
func (c *Client) CreateSpan(ctx context.Context, span Span) (*SpanResponse, error) {
	// Auto-generate ID if not provided
	if span.ID == "" {
		span.ID = generateID()
	}
	resp, err := c.doRequest(ctx, "POST", "/spans", span)
	if err != nil {
		return nil, err
	}

	var spanResp SpanResponse
	if err := c.handleResponse(resp, &spanResp, true); err != nil {
		return nil, err
	}

	// Return the ID we generated/used
	if spanResp.ID == "" {
		spanResp.ID = span.ID
	}

	return &spanResp, nil
}

// CreateGeneration creates a new generation
func (c *Client) CreateGeneration(ctx context.Context, generation Generation) (*GenerationResponse, error) {
	// Auto-generate ID if not provided
	if generation.ID == "" {
		generation.ID = generateID()
	}
	resp, err := c.doRequest(ctx, "POST", "/generations", generation)
	if err != nil {
		return nil, err
	}

	var genResp GenerationResponse
	if err := c.handleResponse(resp, &genResp, true); err != nil {
		return nil, err
	}

	// Return the ID we generated/used
	if genResp.ID == "" {
		genResp.ID = generation.ID
	}

	return &genResp, nil
}

// CreateEvent creates a new event
func (c *Client) CreateEvent(ctx context.Context, event Event) (*EventResponse, error) {
	// Auto-generate ID if not provided
	if event.ID == "" {
		event.ID = generateID()
	}
	resp, err := c.doRequest(ctx, "POST", "/events", event)
	if err != nil {
		return nil, err
	}

	var eventResp EventResponse
	if err := c.handleResponse(resp, &eventResp, true); err != nil {
		return nil, err
	}

	// Return the ID we generated/used
	if eventResp.ID == "" {
		eventResp.ID = event.ID
	}

	return &eventResp, nil
}

// Score creates a score for a trace
func (c *Client) Score(ctx context.Context, score Score) (*ScoreResponse, error) {
	if score.Name == "" {
		return nil, fmt.Errorf("score name is required")
	}
	// Auto-generate ID if not provided
	if score.ID == "" {
		score.ID = generateID()
	}
	resp, err := c.doRequest(ctx, "POST", "/scores", score)
	if err != nil {
		return nil, err
	}

	var scoreResp ScoreResponse
	if err := c.handleResponse(resp, &scoreResp, true); err != nil {
		return nil, err
	}

	// Return the ID we generated/used
	if scoreResp.ID == "" {
		scoreResp.ID = score.ID
	}

	return &scoreResp, nil
}

// UpdateTrace updates an existing trace by posting with the same ID (upsert)
func (c *Client) UpdateTrace(ctx context.Context, traceID string, trace TraceUpdate) (*TraceResponse, error) {
	// Create a trace with the ID and update fields - Langfuse uses POST for upsert
	updateTrace := traceUpsert{
		ID:        traceID,
		Name:      trace.Name,
		UserID:    trace.UserID,
		SessionID: trace.SessionID,
		Version:   trace.Version,
		Release:   trace.Release,
		Metadata:  trace.Metadata,
		Tags:      trace.Tags,
		Input:     trace.Input,
		Output:    trace.Output,
		Public:    trace.Public,
		Level:     trace.Level,
	}

	resp, err := c.doRequest(ctx, "POST", "/traces", updateTrace)
	if err != nil {
		return nil, err
	}

	var traceResp TraceResponse
	if err := c.handleResponse(resp, &traceResp, true); err != nil {
		return nil, err
	}

	// Return the ID we used
	if traceResp.ID == "" {
		traceResp.ID = traceID
	}

	return &traceResp, nil
}

// UpdateSpan updates an existing span by posting with the same ID (upsert)
func (c *Client) UpdateSpan(ctx context.Context, spanID string, span SpanUpdate) (*SpanResponse, error) {
	// Create a span with the ID and update fields
	updateSpan := spanUpsert{
		ID:            spanID,
		Name:          span.Name,
		EndTime:       span.EndTime,
		Metadata:      span.Metadata,
		Input:         span.Input,
		Output:        span.Output,
		Level:         span.Level,
		StatusMessage: span.StatusMessage,
	}

	resp, err := c.doRequest(ctx, "POST", "/spans", updateSpan)
	if err != nil {
		return nil, err
	}

	var spanResp SpanResponse
	if err := c.handleResponse(resp, &spanResp, true); err != nil {
		return nil, err
	}

	if spanResp.ID == "" {
		spanResp.ID = spanID
	}

	return &spanResp, nil
}

// UpdateGeneration updates an existing generation by posting with the same ID (upsert)
func (c *Client) UpdateGeneration(ctx context.Context, generationID string, generation GenerationUpdate) (*GenerationResponse, error) {
	// Create a generation with the ID and update fields
	updateGen := generationUpsert{
		ID:              generationID,
		Name:            generation.Name,
		EndTime:         generation.EndTime,
		Model:           generation.Model,
		ModelParameters: generation.ModelParameters,
		Input:           generation.Input,
		Output:          generation.Output,
		Metadata:        generation.Metadata,
		Level:           generation.Level,
		StatusMessage:   generation.StatusMessage,
		Usage:           generation.Usage,
		Prompt:          generation.Prompt,
		Completion:      generation.Completion,
	}

	resp, err := c.doRequest(ctx, "POST", "/generations", updateGen)
	if err != nil {
		return nil, err
	}

	var genResp GenerationResponse
	if err := c.handleResponse(resp, &genResp, true); err != nil {
		return nil, err
	}

	if genResp.ID == "" {
		genResp.ID = generationID
	}

	return &genResp, nil
}

// Flush flushes any pending events (for async implementations)
func (c *Client) Flush() error {
	// This would be implemented if we add async/batching support
	return nil
}

// Shutdown gracefully shuts down the client
func (c *Client) Shutdown() error {
	return c.Flush()
}
