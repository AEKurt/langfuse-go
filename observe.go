package langfuse

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

// ObserveOptions configures the observe wrapper behavior
type ObserveOptions struct {
	Name          string
	AsType        ObservationType
	CaptureInput  bool
	CaptureOutput bool
}

// Observe wraps a function to automatically capture inputs, outputs, timings, and errors
// This is the Go equivalent of the observe decorator pattern
func (c *Client) Observe(ctx context.Context, fn interface{}, opts *ObserveOptions) (interface{}, error) {
	if opts == nil {
		opts = &ObserveOptions{
			CaptureInput:  true,
			CaptureOutput: true,
		}
	}

	if opts.AsType == "" {
		opts.AsType = ObservationTypeSpan
	}

	if opts.Name == "" {
		opts.Name = getFunctionName(fn)
	}

	// Get function value and type
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		return nil, fmt.Errorf("observe: expected function, got %T", fn)
	}

	// Start observation
	var input interface{}
	if opts.CaptureInput {
		input = captureInput(fnType, fnValue)
	}

	obs, err := c.StartObservation(ctx, opts.AsType, opts.Name, input)
	if err != nil {
		return nil, fmt.Errorf("observe: failed to start observation: %w", err)
	}

	// Execute function
	startTime := time.Now()
	var result interface{}
	var fnErr error

	// Handle different function signatures
	numIn := fnType.NumIn()
	numOut := fnType.NumOut()

	// Prepare arguments (skip context if first param)
	args := make([]reflect.Value, 0, numIn)
	for i := 0; i < numIn; i++ {
		paramType := fnType.In(i)
		if paramType == reflect.TypeOf((*context.Context)(nil)).Elem() {
			args = append(args, reflect.ValueOf(obs.Context()))
		} else {
			// For simplicity, we'll require context as first param or no context
			args = append(args, reflect.Zero(paramType))
		}
	}

	// Call function
	results := fnValue.Call(args)

	// Extract results
	if numOut > 0 {
		result = results[0].Interface()
	}
	if numOut > 1 {
		errVal := results[1]
		errType := reflect.TypeOf((*error)(nil)).Elem()
		if errVal.Type().Implements(errType) && !errVal.IsNil() {
			fnErr = errVal.Interface().(error)
		}
	}

	// Update observation with output and end time
	endTime := time.Now()
	update := map[string]interface{}{
		"duration_ms": time.Since(startTime).Milliseconds(),
	}

	if opts.CaptureOutput && result != nil {
		update["output"] = result
	}

	if fnErr != nil {
		update["error"] = fnErr.Error()
		update["status"] = "error"
	} else {
		update["status"] = "success"
	}

	// Update based on observation type
	switch opts.AsType {
	case ObservationTypeSpan:
		spanUpdate := SpanUpdate{
			EndTime: &endTime,
			Output:  update,
		}
		if fnErr != nil {
			errMsg := fnErr.Error()
			spanUpdate.StatusMessage = &errMsg
		}
		_ = obs.Update(spanUpdate)
	case ObservationTypeGeneration:
		genUpdate := GenerationUpdate{
			EndTime: &endTime,
			Output:  update,
		}
		if fnErr != nil {
			errMsg := fnErr.Error()
			genUpdate.StatusMessage = &errMsg
		}
		_ = obs.Update(genUpdate)
	}

	return result, fnErr
}

// getFunctionName extracts a readable name from a function
func getFunctionName(fn interface{}) string {
	fnValue := reflect.ValueOf(fn)
	if fnValue.Kind() == reflect.Func {
		// Try to get function name from type
		return fmt.Sprintf("%T", fn)
	}
	return "unknown-function"
}

// captureInput attempts to capture function input parameters
func captureInput(fnType reflect.Type, fnValue reflect.Value) interface{} {
	// For now, return a simple representation
	// In a real implementation, you'd want to capture actual parameter values
	return map[string]interface{}{
		"function": getFunctionName(fnValue.Interface()),
	}
}
