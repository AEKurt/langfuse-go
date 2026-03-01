package langfuse

import (
	"context"
	"testing"
)

func TestCreateTraceID_Random(t *testing.T) {
	id1 := CreateTraceID()
	id2 := CreateTraceID()
	if id1 == id2 {
		t.Error("CreateTraceID() should return unique IDs")
	}
	if len(id1) != 32 {
		t.Errorf("CreateTraceID() length = %d, want 32", len(id1))
	}
}

func TestCreateTraceID_Seeded(t *testing.T) {
	id1 := CreateTraceID("my-seed")
	id2 := CreateTraceID("my-seed")
	if id1 != id2 {
		t.Error("CreateTraceID() with same seed should return same ID")
	}
	id3 := CreateTraceID("other-seed")
	if id1 == id3 {
		t.Error("CreateTraceID() with different seeds should return different IDs")
	}
	if len(id1) != 32 {
		t.Errorf("CreateTraceID() seeded length = %d, want 32", len(id1))
	}
}

func TestCreateTraceID_EmptySeed(t *testing.T) {
	// Empty seed should fall back to random
	id1 := CreateTraceID("")
	id2 := CreateTraceID("")
	if id1 == id2 {
		t.Error("CreateTraceID('') should return unique IDs (falls back to random)")
	}
}

func TestCreateObservationID_Random(t *testing.T) {
	id1 := CreateObservationID()
	id2 := CreateObservationID()
	if id1 == id2 {
		t.Error("CreateObservationID() should return unique IDs")
	}
	if len(id1) != 16 {
		t.Errorf("CreateObservationID() length = %d, want 16", len(id1))
	}
}

func TestCreateObservationID_Seeded(t *testing.T) {
	id1 := CreateObservationID("seed")
	id2 := CreateObservationID("seed")
	if id1 != id2 {
		t.Error("CreateObservationID() with same seed should return same ID")
	}
	id3 := CreateObservationID("other-seed")
	if id1 == id3 {
		t.Error("CreateObservationID() with different seeds should return different IDs")
	}
	if len(id1) != 16 {
		t.Errorf("CreateObservationID() seeded length = %d, want 16", len(id1))
	}
}

func TestGetCurrentTraceID_Empty(t *testing.T) {
	ctx := context.Background()
	_, ok := GetCurrentTraceID(ctx)
	if ok {
		t.Error("GetCurrentTraceID() should return false for empty context")
	}
}

func TestGetCurrentTraceID_Set(t *testing.T) {
	ctx := WithTraceContext(context.Background(), TraceContext{TraceID: "trace-abc"})
	id, ok := GetCurrentTraceID(ctx)
	if !ok {
		t.Error("GetCurrentTraceID() should return true when trace context is set")
	}
	if id != "trace-abc" {
		t.Errorf("GetCurrentTraceID() = %v, want trace-abc", id)
	}
}

func TestGetCurrentTraceID_EmptyTraceID(t *testing.T) {
	// TraceContext exists but TraceID is empty
	ctx := WithTraceContext(context.Background(), TraceContext{SpanID: "span-xyz"})
	_, ok := GetCurrentTraceID(ctx)
	if ok {
		t.Error("GetCurrentTraceID() should return false when TraceID is empty")
	}
}

func TestGetCurrentObservationID_Empty(t *testing.T) {
	ctx := context.Background()
	_, ok := GetCurrentObservationID(ctx)
	if ok {
		t.Error("GetCurrentObservationID() should return false for empty context")
	}
}

func TestGetCurrentObservationID_Set(t *testing.T) {
	ctx := WithTraceContext(context.Background(), TraceContext{TraceID: "trace-abc", SpanID: "span-xyz"})
	id, ok := GetCurrentObservationID(ctx)
	if !ok {
		t.Error("GetCurrentObservationID() should return true when SpanID is set")
	}
	if id != "span-xyz" {
		t.Errorf("GetCurrentObservationID() = %v, want span-xyz", id)
	}
}

func TestGetCurrentObservationID_EmptySpanID(t *testing.T) {
	ctx := WithTraceContext(context.Background(), TraceContext{TraceID: "trace-abc"})
	_, ok := GetCurrentObservationID(ctx)
	if ok {
		t.Error("GetCurrentObservationID() should return false when SpanID is empty")
	}
}

func TestWithAndGetPropagatedAttributes(t *testing.T) {
	ctx := context.Background()

	_, ok := GetPropagatedAttributes(ctx)
	if ok {
		t.Error("GetPropagatedAttributes() should return false for empty context")
	}

	attrs := PropagatedAttributes{
		SessionID: "session-1",
		UserID:    "user-1",
		Tags:      []string{"tag1", "tag2"},
		Metadata:  map[string]interface{}{"key": "value"},
	}
	ctx = WithPropagatedAttributes(ctx, attrs)

	got, ok := GetPropagatedAttributes(ctx)
	if !ok {
		t.Error("GetPropagatedAttributes() should return true when attributes are set")
	}
	if got.SessionID != "session-1" {
		t.Errorf("SessionID = %v, want session-1", got.SessionID)
	}
	if got.UserID != "user-1" {
		t.Errorf("UserID = %v, want user-1", got.UserID)
	}
	if len(got.Tags) != 2 {
		t.Errorf("Tags length = %d, want 2", len(got.Tags))
	}
	if got.Metadata["key"] != "value" {
		t.Errorf("Metadata[key] = %v, want value", got.Metadata["key"])
	}
}

func TestMergePropagatedAttributes_IntoEmpty(t *testing.T) {
	ctx := context.Background()
	ctx = MergePropagatedAttributes(ctx, PropagatedAttributes{
		SessionID: "session-1",
		UserID:    "user-1",
		Tags:      []string{"tag1"},
		Metadata:  map[string]interface{}{"key1": "val1"},
	})

	got, ok := GetPropagatedAttributes(ctx)
	if !ok {
		t.Error("GetPropagatedAttributes() should return true after merge into empty context")
	}
	if got.SessionID != "session-1" {
		t.Errorf("SessionID = %v, want session-1", got.SessionID)
	}
	if got.UserID != "user-1" {
		t.Errorf("UserID = %v, want user-1", got.UserID)
	}
}

func TestMergePropagatedAttributes_OverrideAndMerge(t *testing.T) {
	ctx := context.Background()
	ctx = WithPropagatedAttributes(ctx, PropagatedAttributes{
		SessionID: "session-1",
		UserID:    "user-1",
		Tags:      []string{"tag1"},
		Metadata:  map[string]interface{}{"key1": "val1"},
	})

	// Merge: new UserID overrides, SessionID kept, tags/metadata merged
	ctx = MergePropagatedAttributes(ctx, PropagatedAttributes{
		UserID:   "user-2",
		Tags:     []string{"tag2"},
		Metadata: map[string]interface{}{"key2": "val2"},
	})

	got, _ := GetPropagatedAttributes(ctx)
	if got.SessionID != "session-1" {
		t.Errorf("SessionID = %v, want session-1 (kept from existing)", got.SessionID)
	}
	if got.UserID != "user-2" {
		t.Errorf("UserID = %v, want user-2 (overridden by new)", got.UserID)
	}
	if got.Metadata["key1"] == nil {
		t.Error("Metadata should still contain key1 from existing attrs")
	}
	if got.Metadata["key2"] == nil {
		t.Error("Metadata should contain key2 from new attrs")
	}
	// Both tags should be present
	tagSet := make(map[string]bool)
	for _, tag := range got.Tags {
		tagSet[tag] = true
	}
	if !tagSet["tag1"] || !tagSet["tag2"] {
		t.Errorf("Tags = %v, want both tag1 and tag2", got.Tags)
	}
}

func TestMergePropagatedAttributes_KeepExistingWhenNewEmpty(t *testing.T) {
	ctx := WithPropagatedAttributes(context.Background(), PropagatedAttributes{
		SessionID: "session-original",
		UserID:    "user-original",
	})

	// Merge empty new attrs — existing values should be preserved
	ctx = MergePropagatedAttributes(ctx, PropagatedAttributes{})

	got, _ := GetPropagatedAttributes(ctx)
	if got.SessionID != "session-original" {
		t.Errorf("SessionID = %v, want session-original", got.SessionID)
	}
	if got.UserID != "user-original" {
		t.Errorf("UserID = %v, want user-original", got.UserID)
	}
}
