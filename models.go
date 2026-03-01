package langfuse

import (
	"time"
)

// Level represents the log level for traces and observations
type Level string

const (
	// LevelDefault is the default level
	LevelDefault Level = "DEFAULT"
	// LevelDebug is for debug messages
	LevelDebug Level = "DEBUG"
	// LevelInfo is for informational messages
	LevelInfo Level = "INFO"
	// LevelWarning is for warning messages
	LevelWarning Level = "WARNING"
	// LevelError is for error messages
	LevelError Level = "ERROR"
)

// Trace represents a trace in Langfuse
type Trace struct {
	ID         string                 `json:"id,omitempty"`
	Name       string                 `json:"name,omitempty"`
	UserID     string                 `json:"userId,omitempty"`
	SessionID  string                 `json:"sessionId,omitempty"`
	Version    string                 `json:"version,omitempty"`
	Release    string                 `json:"release,omitempty"`
	Input      interface{}            `json:"input,omitempty"`
	Output     interface{}            `json:"output,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	Public     bool                   `json:"public,omitempty"`
	Timestamp  *time.Time             `json:"timestamp,omitempty"`
	ExternalID string                 `json:"externalId,omitempty"`
	Level      Level                  `json:"level,omitempty"`
}

// TraceUpdate represents an update to a trace
type TraceUpdate struct {
	Name      *string                `json:"name,omitempty"`
	UserID    *string                `json:"userId,omitempty"`
	SessionID *string                `json:"sessionId,omitempty"`
	Version   *string                `json:"version,omitempty"`
	Release   *string                `json:"release,omitempty"`
	Input     interface{}            `json:"input,omitempty"`
	Output    interface{}            `json:"output,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
	Public    *bool                  `json:"public,omitempty"`
	Level     *Level                 `json:"level,omitempty"`
}

// TraceResponse represents the response from creating/updating a trace
type TraceResponse struct {
	ID string `json:"id"`
}

// Span represents a span in Langfuse
type Span struct {
	ID                  string                 `json:"id,omitempty"`
	TraceID             string                 `json:"traceId,omitempty"`
	Name                string                 `json:"name,omitempty"`
	StartTime           *time.Time             `json:"startTime,omitempty"`
	EndTime             *time.Time             `json:"endTime,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	Input               interface{}            `json:"input,omitempty"`
	Output              interface{}            `json:"output,omitempty"`
	Level               Level                  `json:"level,omitempty"`
	StatusMessage       string                 `json:"statusMessage,omitempty"`
	ParentObservationID string                 `json:"parentObservationId,omitempty"`
}

// SpanUpdate represents an update to a span
type SpanUpdate struct {
	Name          *string                `json:"name,omitempty"`
	EndTime       *time.Time             `json:"endTime,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Input         interface{}            `json:"input,omitempty"`
	Output        interface{}            `json:"output,omitempty"`
	Level         *Level                 `json:"level,omitempty"`
	StatusMessage *string                `json:"statusMessage,omitempty"`
}

// SpanResponse represents the response from creating/updating a span
type SpanResponse struct {
	ID string `json:"id"`
}

// Generation represents a generation (LLM call) in Langfuse
type Generation struct {
	ID                  string                 `json:"id,omitempty"`
	TraceID             string                 `json:"traceId,omitempty"`
	Name                string                 `json:"name,omitempty"`
	StartTime           *time.Time             `json:"startTime,omitempty"`
	EndTime             *time.Time             `json:"endTime,omitempty"`
	Model               string                 `json:"model,omitempty"`
	ModelParameters     map[string]interface{} `json:"modelParameters,omitempty"`
	Input               interface{}            `json:"input,omitempty"`
	Output              interface{}            `json:"output,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	Level               Level                  `json:"level,omitempty"`
	StatusMessage       string                 `json:"statusMessage,omitempty"`
	ParentObservationID string                 `json:"parentObservationId,omitempty"`
	Usage               *Usage                 `json:"usage,omitempty"`
	Prompt              *Prompt                `json:"prompt,omitempty"`
	Completion          *Completion            `json:"completion,omitempty"`
}

// GenerationUpdate represents an update to a generation
type GenerationUpdate struct {
	Name            *string                `json:"name,omitempty"`
	EndTime         *time.Time             `json:"endTime,omitempty"`
	Model           *string                `json:"model,omitempty"`
	ModelParameters map[string]interface{} `json:"modelParameters,omitempty"`
	Input           interface{}            `json:"input,omitempty"`
	Output          interface{}            `json:"output,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	Level           *Level                 `json:"level,omitempty"`
	StatusMessage   *string                `json:"statusMessage,omitempty"`
	Usage           *Usage                 `json:"usage,omitempty"`
	Prompt          *Prompt                `json:"prompt,omitempty"`
	Completion      *Completion            `json:"completion,omitempty"`
}

// GenerationResponse represents the response from creating/updating a generation
type GenerationResponse struct {
	ID string `json:"id"`
}

// Event represents an event in Langfuse
type Event struct {
	ID                  string                 `json:"id,omitempty"`
	TraceID             string                 `json:"traceId,omitempty"`
	Name                string                 `json:"name,omitempty"`
	StartTime           *time.Time             `json:"startTime,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	Input               interface{}            `json:"input,omitempty"`
	Output              interface{}            `json:"output,omitempty"`
	Level               Level                  `json:"level,omitempty"`
	ParentObservationID string                 `json:"parentObservationId,omitempty"`
}

// EventResponse represents the response from creating an event
type EventResponse struct {
	ID string `json:"id"`
}

// Score represents a score in Langfuse
type Score struct {
	ID            string  `json:"id,omitempty"`
	TraceID       string  `json:"traceId,omitempty"`
	Name          string  `json:"name"`
	Value         float64 `json:"value"`
	ObservationID string  `json:"observationId,omitempty"`
	Comment       string  `json:"comment,omitempty"`
}

// ScoreResponse represents the response from creating a score
type ScoreResponse struct {
	ID string `json:"id"`
}

// Usage represents token usage information
type Usage struct {
	Input  int    `json:"input,omitempty"`
	Output int    `json:"output,omitempty"`
	Total  int    `json:"total,omitempty"`
	Unit   string `json:"unit,omitempty"` // TOKENS, CHARACTERS, etc.
}

// Prompt represents prompt information
type Prompt struct {
	Raw      string                   `json:"raw,omitempty"`
	Messages []map[string]interface{} `json:"messages,omitempty"`
}

// Completion represents completion information
type Completion struct {
	Raw      string                   `json:"raw,omitempty"`
	Messages []map[string]interface{} `json:"messages,omitempty"`
}

// traceUpsert is used internally for trace create/update (upsert) operations.
type traceUpsert struct {
	ID        string                 `json:"id"`
	Name      *string                `json:"name,omitempty"`
	UserID    *string                `json:"userId,omitempty"`
	SessionID *string                `json:"sessionId,omitempty"`
	Version   *string                `json:"version,omitempty"`
	Release   *string                `json:"release,omitempty"`
	Input     interface{}            `json:"input,omitempty"`
	Output    interface{}            `json:"output,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
	Public    *bool                  `json:"public,omitempty"`
	Level     *Level                 `json:"level,omitempty"`
}

// spanUpsert is used internally for span create/update (upsert) operations.
type spanUpsert struct {
	ID            string                 `json:"id"`
	Name          *string                `json:"name,omitempty"`
	EndTime       *time.Time             `json:"endTime,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Input         interface{}            `json:"input,omitempty"`
	Output        interface{}            `json:"output,omitempty"`
	Level         *Level                 `json:"level,omitempty"`
	StatusMessage *string                `json:"statusMessage,omitempty"`
}

// generationUpsert is used internally for generation create/update (upsert) operations.
type generationUpsert struct {
	ID              string                 `json:"id"`
	Name            *string                `json:"name,omitempty"`
	EndTime         *time.Time             `json:"endTime,omitempty"`
	Model           *string                `json:"model,omitempty"`
	ModelParameters map[string]interface{} `json:"modelParameters,omitempty"`
	Input           interface{}            `json:"input,omitempty"`
	Output          interface{}            `json:"output,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	Level           *Level                 `json:"level,omitempty"`
	StatusMessage   *string                `json:"statusMessage,omitempty"`
	Usage           *Usage                 `json:"usage,omitempty"`
	Prompt          *Prompt                `json:"prompt,omitempty"`
	Completion      *Completion            `json:"completion,omitempty"`
}
