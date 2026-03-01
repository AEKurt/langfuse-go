# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.6] - 2026-03-01

### Fixed
- `UpdateTrace` was silently dropping `Version`, `Release`, and `Level` fields
- `observe.go`: guard `IsNil()` with error interface type check to prevent panic on non-nilable second return values

### Changed
- Extract `traceUpsert`, `spanUpsert`, `generationUpsert` as internal named types, eliminating 4 duplicated anonymous struct definitions across `client.go` and `batch.go`
- Simplify `handleResponse`: collapse duplicated if/else into a single condition
- Rename `BatchEventTypeGenUpdate` to `BatchEventTypeGenerationUpdate` for naming consistency
- Replace hardcoded `30s` timeout in `sendBatch` with the `DefaultTimeout` constant
- Clarify `BatchProcessor.Flush()` doc comment: it only drains the channel queue; use `Stop()` for a guaranteed full flush

### Added
- Test coverage increased from 64.7% to 88.2%:
  - New `trace_context_test.go`: `CreateTraceID`/`ObservationID` (seeded & random), `GetCurrentTraceID`/`ObservationID`, `WithPropagatedAttributes`, `GetPropagatedAttributes`, `MergePropagatedAttributes`
  - New `observe_test.go`: `Observe` wrapper (success, error, no-return, generation type, non-function), `getFunctionName`
  - Additional tests for `Context()`, `UpdateCurrentSpan`, `StartAsCurrentSpan`, `StartAsCurrentGeneration`, `APIError.Unwrap`, `Flush`, `Shutdown`, logger path, batch processor edge cases

### Docs
- Add `Logger` field to Config API reference in README
- Add Observe Wrapper section to Advanced Features
- Add Propagated Attributes section to Advanced Features
- Add `ShutdownTimeout` to `BatchConfig` example
- Fix error-ignored patterns in async code examples

## [0.1.5] - 2025-12-22

### Added
- Added GitHub Actions CI/CD workflow for automated testing and coverage
- Added [go-test-coverage](https://github.com/vladopajic/go-test-coverage) integration for coverage reporting
- Added coverage badge to README
- Added Makefile with helpful development commands:
  - `make test` - run tests
  - `make test-race` - run tests with race detector
  - `make coverage` - generate HTML coverage report
  - `make check-coverage` - verify coverage thresholds
  - `make lint` - run golangci-lint

### Changed
- Enhanced test suite with comprehensive input/output data for all observation types
- Updated README with development instructions and badges

## [0.1.4] - 2025-12-22

### Added
- Added async/batch processing for production performance:
  - `BatchProcessor` - background batch processing with configurable settings
  - `AsyncClient` - high-level async client wrapping batch processor
  - Configurable batch size, flush interval, retries, and queue size
  - Automatic retry with exponential backoff for failed requests
  - Graceful shutdown with flush of pending events
- Async methods on `AsyncClient`:
  - `CreateTraceAsync`, `CreateSpanAsync`, `UpdateSpanAsync`
  - `CreateGenerationAsync`, `UpdateGenerationAsync`
  - `CreateEventAsync`, `ScoreAsync`
  - `Flush()` - manual flush of pending events
  - `Shutdown()` - graceful shutdown

## [0.1.3] - 2025-12-22

### Added
- Added specialized observation types for better categorization:
  - `ObservationTypeAgent` - reasoning blocks using LLM guidance
  - `ObservationTypeTool` - external tool calls (e.g., APIs)
  - `ObservationTypeChain` - connecting LLM application steps
  - `ObservationTypeRetriever` - data retrieval (e.g., vector stores)
  - `ObservationTypeEmbedding` - LLM embedding calls
  - `ObservationTypeEvaluator` - assessing LLM outputs
  - `ObservationTypeGuardrail` - protection against jailbreaks
- Added convenience methods for starting specialized observations:
  - `StartAgent`, `StartTool`, `StartChain`, `StartRetriever`
  - `StartEmbedding`, `StartEvaluator`, `StartGuardrail`
- Added context-aware variants:
  - `StartAsCurrentAgent`, `StartAsCurrentTool`, `StartAsCurrentChain`
  - `StartAsCurrentRetriever`, `StartAsCurrentEmbedding`
  - `StartAsCurrentEvaluator`, `StartAsCurrentGuardrail`

## [0.1.2] - 2025-12-22

### Added
- Added `Input` and `Output` fields to `Trace` and `TraceUpdate` structs
  - Traces now properly support input/output as per [Langfuse FAQ](https://langfuse.com/faq/all/empty-trace-input-and-output)
- Added `PropagatedAttributes` for automatic attribute propagation
  - `WithPropagatedAttributes(ctx, attrs)` - set attributes for all child spans
  - `GetPropagatedAttributes(ctx)` - get propagated attributes
  - `MergePropagatedAttributes(ctx, attrs)` - merge with existing attributes
- Added `UpdateCurrentSpan(ctx, output, metadata)` - update current span from context
- Added `StartAsCurrentSpan(ctx, name, input)` - start span and store in context
- Added `StartAsCurrentGeneration(ctx, name, model, input)` - start generation and store in context
- Added `GetCurrentObservation(ctx)` - get current observation from context
- Added `WithCurrentObservation(ctx, obs)` - store observation in context

### Fixed
- Auto-generate IDs for Trace, Span, Generation, Event, and Score when not provided
- Fixed UpdateSpan and UpdateGeneration to use POST with ID (upsert) instead of PATCH
- API now correctly sends required `id` field for all entities

## [0.1.1] - 2025-12-22

### Fixed
- Added missing `fmt` import in `observation.go`
- Removed unused `err` variable in `observation.go` StartObservation function
- Removed unused `expectedStatus` variable in `client.go` handleResponse function
- Fixed integration test missing context parameter in CreateTrace call
- Fixed examples compilation errors:
  - `getting_started`: Added missing StartTime field to Generation
  - `error_handling`: Fixed taking address of constant (LevelError)
  - `observe_wrapper`: Fixed generic type mismatch in observeSpan function
- Fixed redundant newline warnings in example files

## [0.1.0] - 2025-12-22

### Added
- Initial release of Langfuse Go SDK
- Client initialization with configurable base URL and HTTP client
- Trace creation and updates
- Span creation and updates
- Generation creation and updates
- Event creation
- Score creation
- Helper functions for creating entities with auto-generated IDs
- Comprehensive error handling with APIError type
- W3C Trace Context support with deterministic trace ID generation
- Context manager pattern with StartObservation and StartChildObservation
- Trace context propagation across services
- Logger interface for debugging API requests/responses
- Example code demonstrating:
  - Basic usage
  - Advanced features (nested observations, context propagation)
  - Observe wrapper pattern
  - Error handling
  - Logger integration
  - Context propagation across services

