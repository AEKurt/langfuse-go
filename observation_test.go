package langfuse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestObservationTypes(t *testing.T) {
	// Verify all observation types are defined correctly
	types := []ObservationType{
		ObservationTypeSpan,
		ObservationTypeGeneration,
		ObservationTypeEvent,
		ObservationTypeAgent,
		ObservationTypeTool,
		ObservationTypeChain,
		ObservationTypeRetriever,
		ObservationTypeEmbedding,
		ObservationTypeEvaluator,
		ObservationTypeGuardrail,
	}

	expectedValues := map[ObservationType]string{
		ObservationTypeSpan:       "span",
		ObservationTypeGeneration: "generation",
		ObservationTypeEvent:      "event",
		ObservationTypeAgent:      "agent",
		ObservationTypeTool:       "tool",
		ObservationTypeChain:      "chain",
		ObservationTypeRetriever:  "retriever",
		ObservationTypeEmbedding:  "embedding",
		ObservationTypeEvaluator:  "evaluator",
		ObservationTypeGuardrail:  "guardrail",
	}

	for _, obsType := range types {
		expected, ok := expectedValues[obsType]
		if !ok {
			t.Errorf("unexpected observation type: %s", obsType)
			continue
		}
		if string(obsType) != expected {
			t.Errorf("observation type %s has wrong value: got %s, want %s", obsType, string(obsType), expected)
		}
	}
}

func createTestServer(t *testing.T) (*httptest.Server, *Client) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode request body to get ID
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)

		id := "generated-id"
		if bodyID, ok := body["id"].(string); ok && bodyID != "" {
			id = bodyID
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
	}))

	client, err := NewClient(Config{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	return server, client
}

func TestStartObservation_AllTypes(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	tests := []struct {
		name    string
		obsType ObservationType
		obsName string
		input   map[string]interface{}
	}{
		{
			"span", ObservationTypeSpan, "database-query-span",
			map[string]interface{}{"query": "SELECT * FROM users WHERE active = true", "database": "postgres"},
		},
		{
			"generation", ObservationTypeGeneration, "chat-response-generation",
			map[string]interface{}{"messages": []string{"Hello, how can I help?"}, "model": "gpt-4"},
		},
		{
			"event", ObservationTypeEvent, "user-signup-event",
			map[string]interface{}{"user_id": "user-456", "source": "organic", "plan": "premium"},
		},
		{
			"agent", ObservationTypeAgent, "customer-service-agent",
			map[string]interface{}{"query": "I want to cancel my subscription", "sentiment": "negative"},
		},
		{
			"tool", ObservationTypeTool, "email-sender-tool",
			map[string]interface{}{"to": "user@example.com", "subject": "Order Confirmation", "template": "order_confirm"},
		},
		{
			"chain", ObservationTypeChain, "document-qa-chain",
			map[string]interface{}{"question": "What is the refund policy?", "context_docs": 5},
		},
		{
			"retriever", ObservationTypeRetriever, "faq-retriever",
			map[string]interface{}{"query": "How to reset password", "collection": "faq_docs", "top_k": 3},
		},
		{
			"embedding", ObservationTypeEmbedding, "product-embedding",
			map[string]interface{}{"text": "Wireless Bluetooth Headphones with Noise Cancellation", "purpose": "product_search"},
		},
		{
			"evaluator", ObservationTypeEvaluator, "response-relevance-evaluator",
			map[string]interface{}{"query": "Best restaurants nearby", "response": "Here are top-rated restaurants...", "metric": "relevance"},
		},
		{
			"guardrail", ObservationTypeGuardrail, "content-moderation-guardrail",
			map[string]interface{}{"content": "This is a normal message about cooking recipes", "checks": []string{"toxicity", "spam"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs, err := client.StartObservation(context.Background(), tt.obsType, tt.obsName, tt.input)
			if err != nil {
				t.Fatalf("StartObservation(%s) error = %v", tt.obsType, err)
			}
			if obs == nil {
				t.Fatalf("StartObservation(%s) returned nil", tt.obsType)
			}
			if obs.Type != tt.obsType {
				t.Errorf("StartObservation(%s) Type = %s, want %s", tt.obsType, obs.Type, tt.obsType)
			}
			if obs.ID == "" {
				t.Errorf("StartObservation(%s) ID is empty", tt.obsType)
			}
			if obs.TraceID == "" {
				t.Errorf("StartObservation(%s) TraceID is empty", tt.obsType)
			}
		})
	}
}

func TestStartAgent(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	obs, err := client.StartAgent(context.Background(), "restaurant-finder-agent", map[string]interface{}{
		"task":        "Find best Italian restaurant in Manhattan",
		"user_intent": "restaurant_search",
		"constraints": map[string]interface{}{
			"cuisine":  "Italian",
			"location": "Manhattan, NYC",
			"budget":   "$$-$$$",
		},
	})
	if err != nil {
		t.Fatalf("StartAgent() error = %v", err)
	}
	if obs.Type != ObservationTypeAgent {
		t.Errorf("StartAgent() Type = %s, want %s", obs.Type, ObservationTypeAgent)
	}
}

func TestStartTool(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	obs, err := client.StartTool(context.Background(), "weather-api", map[string]interface{}{
		"action":     "get_current_weather",
		"city":       "New York City",
		"country":    "USA",
		"units":      "metric",
		"parameters": []string{"temperature", "humidity", "wind_speed"},
	})
	if err != nil {
		t.Fatalf("StartTool() error = %v", err)
	}
	if obs.Type != ObservationTypeTool {
		t.Errorf("StartTool() Type = %s, want %s", obs.Type, ObservationTypeTool)
	}

	// Update with output
	err = obs.Update(SpanUpdate{
		Output: map[string]interface{}{
			"temperature": 22.5,
			"humidity":    65,
			"wind_speed":  12.3,
			"conditions":  "Partly Cloudy",
			"last_update": "2024-12-22T10:30:00Z",
		},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestStartChain(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	obs, err := client.StartChain(context.Background(), "rag-chain", map[string]interface{}{
		"query":           "How do I implement semantic search with embeddings?",
		"chain_type":      "retrieval_qa",
		"max_tokens":      2048,
		"temperature":     0.7,
		"retrieval_top_k": 5,
	})
	if err != nil {
		t.Fatalf("StartChain() error = %v", err)
	}
	if obs.Type != ObservationTypeChain {
		t.Errorf("StartChain() Type = %s, want %s", obs.Type, ObservationTypeChain)
	}

	// Update with output
	err = obs.Update(SpanUpdate{
		Output: map[string]interface{}{
			"answer": "To implement semantic search: 1) Choose an embedding model (e.g., OpenAI text-embedding-3), 2) Embed your documents, 3) Store vectors in a database like Pinecone or Weaviate, 4) Embed user queries and find similar vectors using cosine similarity.",
			"sources": []map[string]interface{}{
				{"title": "Embedding Models Guide", "relevance": 0.95},
				{"title": "Vector Database Comparison", "relevance": 0.88},
			},
			"confidence": 0.92,
		},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestStartRetriever(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	obs, err := client.StartRetriever(context.Background(), "pinecone-vector-search", map[string]interface{}{
		"query":      "Best practices for building RAG applications",
		"top_k":      10,
		"namespace":  "documentation",
		"provider":   "pinecone",
		"index_name": "knowledge-base-prod",
		"filters": map[string]interface{}{
			"category": "tutorials",
			"language": "english",
		},
	})
	if err != nil {
		t.Fatalf("StartRetriever() error = %v", err)
	}
	if obs.Type != ObservationTypeRetriever {
		t.Errorf("StartRetriever() Type = %s, want %s", obs.Type, ObservationTypeRetriever)
	}

	// Update with output
	err = obs.Update(SpanUpdate{
		Output: map[string]interface{}{
			"documents": []map[string]interface{}{
				{"id": "doc-001", "title": "RAG Architecture Guide", "score": 0.95, "chunk": "RAG combines retrieval with generation..."},
				{"id": "doc-002", "title": "Embedding Best Practices", "score": 0.91, "chunk": "Choose your embedding model based on..."},
				{"id": "doc-003", "title": "Vector Database Selection", "score": 0.87, "chunk": "When selecting a vector database..."},
			},
			"total_results":  3,
			"search_time_ms": 45,
		},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestStartEmbedding(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	obs, err := client.StartEmbedding(context.Background(), "document-embedding", "text-embedding-3-large", map[string]interface{}{
		"text":       "Semantic search enables finding content based on meaning rather than keywords. It uses vector embeddings to represent text numerically.",
		"chunk_size": 512,
		"overlap":    50,
		"purpose":    "document_indexing",
	})
	if err != nil {
		t.Fatalf("StartEmbedding() error = %v", err)
	}
	if obs.Type != ObservationTypeEmbedding {
		t.Errorf("StartEmbedding() Type = %s, want %s", obs.Type, ObservationTypeEmbedding)
	}

	// Update with output
	model := "text-embedding-3-large"
	err = obs.Update(GenerationUpdate{
		Model: &model,
		Output: map[string]interface{}{
			"dimensions":    3072,
			"vector_sample": []float64{0.0123, -0.0456, 0.0789, -0.0321, 0.0654},
			"normalized":    true,
		},
		Usage: &Usage{Input: 28, Total: 28, Unit: "TOKENS"},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestStartEvaluator(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	obs, err := client.StartEvaluator(context.Background(), "response-quality-evaluator", map[string]interface{}{
		"response":  "Based on the retrieved documents, I recommend using OpenAI's text-embedding-3-large model for semantic search. It offers 3072 dimensions and excellent performance on MTEB benchmarks.",
		"query":     "What embedding model should I use for semantic search?",
		"criteria":  []string{"relevance", "accuracy", "helpfulness", "conciseness"},
		"evaluator": "gpt-4-judge",
	})
	if err != nil {
		t.Fatalf("StartEvaluator() error = %v", err)
	}
	if obs.Type != ObservationTypeEvaluator {
		t.Errorf("StartEvaluator() Type = %s, want %s", obs.Type, ObservationTypeEvaluator)
	}

	// Update with output
	err = obs.Update(SpanUpdate{
		Output: map[string]interface{}{
			"scores": map[string]float64{
				"relevance":   0.95,
				"accuracy":    0.92,
				"helpfulness": 0.88,
				"conciseness": 0.85,
			},
			"overall_score": 0.90,
			"reasoning":     "The response directly addresses the query with a specific recommendation backed by evidence. Minor deduction for not mentioning alternatives.",
			"passed":        true,
		},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestStartGuardrail(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	obs, err := client.StartGuardrail(context.Background(), "input-safety-filter", map[string]interface{}{
		"content": "Please help me write a professional cover letter for a software engineering position at Google.",
		"checks": []string{
			"jailbreak_detection",
			"pii_detection",
			"toxicity_check",
			"prompt_injection",
		},
		"thresholds": map[string]float64{
			"toxicity_max":   0.3,
			"jailbreak_max":  0.1,
			"pii_confidence": 0.8,
		},
	})
	if err != nil {
		t.Fatalf("StartGuardrail() error = %v", err)
	}
	if obs.Type != ObservationTypeGuardrail {
		t.Errorf("StartGuardrail() Type = %s, want %s", obs.Type, ObservationTypeGuardrail)
	}

	// Update with output
	err = obs.Update(SpanUpdate{
		Output: map[string]interface{}{
			"passed": true,
			"checks": map[string]interface{}{
				"jailbreak_detection": map[string]interface{}{
					"detected": false, "score": 0.02, "passed": true,
				},
				"pii_detection": map[string]interface{}{
					"detected": false, "entities": []string{}, "passed": true,
				},
				"toxicity_check": map[string]interface{}{
					"score": 0.01, "passed": true,
				},
				"prompt_injection": map[string]interface{}{
					"detected": false, "score": 0.03, "passed": true,
				},
			},
			"processing_time_ms": 23,
			"action":             "allow",
		},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestStartAsCurrentAgent(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	ctx, obs, err := client.StartAsCurrentAgent(context.Background(), "customer-support-agent", map[string]interface{}{
		"user_query":  "I need help resetting my password",
		"customer_id": "cust-12345",
		"priority":    "medium",
		"channel":     "web_chat",
	})
	if err != nil {
		t.Fatalf("StartAsCurrentAgent() error = %v", err)
	}
	if obs.Type != ObservationTypeAgent {
		t.Errorf("StartAsCurrentAgent() Type = %s, want %s", obs.Type, ObservationTypeAgent)
	}

	// Verify observation is stored in context
	retrieved, ok := GetCurrentObservation(ctx)
	if !ok {
		t.Error("StartAsCurrentAgent() observation not stored in context")
	}
	if retrieved.ID != obs.ID {
		t.Errorf("StartAsCurrentAgent() retrieved ID = %s, want %s", retrieved.ID, obs.ID)
	}

	// Update with output
	err = obs.Update(SpanUpdate{
		Output: map[string]interface{}{
			"resolution":      "Password reset link sent to registered email",
			"actions_taken":   []string{"verified_identity", "sent_reset_email", "logged_ticket"},
			"satisfaction":    0.95,
			"escalated":       false,
			"resolution_time": "2m 34s",
		},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestStartAsCurrentTool(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	ctx, obs, err := client.StartAsCurrentTool(context.Background(), "database-query-tool", map[string]interface{}{
		"action":     "execute_query",
		"database":   "postgres",
		"table":      "users",
		"query_type": "SELECT",
		"filters":    map[string]interface{}{"status": "active", "created_after": "2024-01-01"},
	})
	if err != nil {
		t.Fatalf("StartAsCurrentTool() error = %v", err)
	}
	if obs.Type != ObservationTypeTool {
		t.Errorf("StartAsCurrentTool() Type = %s, want %s", obs.Type, ObservationTypeTool)
	}

	retrieved, ok := GetCurrentObservation(ctx)
	if !ok {
		t.Error("StartAsCurrentTool() observation not stored in context")
	}
	if retrieved.ID != obs.ID {
		t.Errorf("StartAsCurrentTool() retrieved ID = %s, want %s", retrieved.ID, obs.ID)
	}

	// Update with output
	err = obs.Update(SpanUpdate{
		Output: map[string]interface{}{
			"rows_returned":     150,
			"execution_time_ms": 45,
			"sample_data": []map[string]interface{}{
				{"id": 1, "name": "John Doe", "email": "john@example.com"},
				{"id": 2, "name": "Jane Smith", "email": "jane@example.com"},
			},
		},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestStartAsCurrentChain(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	ctx, obs, err := client.StartAsCurrentChain(context.Background(), "summarization-chain", map[string]interface{}{
		"document":    "Long article about machine learning advances in 2024...",
		"max_length":  500,
		"style":       "executive_summary",
		"chain_steps": []string{"extract_key_points", "generate_summary", "refine_output"},
	})
	if err != nil {
		t.Fatalf("StartAsCurrentChain() error = %v", err)
	}
	if obs.Type != ObservationTypeChain {
		t.Errorf("StartAsCurrentChain() Type = %s, want %s", obs.Type, ObservationTypeChain)
	}

	retrieved, ok := GetCurrentObservation(ctx)
	if !ok {
		t.Error("StartAsCurrentChain() observation not stored in context")
	}
	if retrieved.ID != obs.ID {
		t.Errorf("StartAsCurrentChain() retrieved ID = %s, want %s", retrieved.ID, obs.ID)
	}

	// Update with output
	err = obs.Update(SpanUpdate{
		Output: map[string]interface{}{
			"summary": "In 2024, ML advances focused on multimodal models, efficient fine-tuning, and AI safety. Key developments include GPT-4V, Gemini, and open-source alternatives.",
			"key_points": []string{
				"Multimodal AI became mainstream",
				"Fine-tuning efficiency improved 10x",
				"AI safety research accelerated",
			},
			"word_count":        42,
			"compression_ratio": 0.08,
		},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestStartAsCurrentRetriever(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	ctx, obs, err := client.StartAsCurrentRetriever(context.Background(), "hybrid-search-retriever", map[string]interface{}{
		"query":         "Python async programming best practices",
		"search_type":   "hybrid",
		"vector_weight": 0.7,
		"bm25_weight":   0.3,
		"top_k":         5,
		"rerank":        true,
	})
	if err != nil {
		t.Fatalf("StartAsCurrentRetriever() error = %v", err)
	}
	if obs.Type != ObservationTypeRetriever {
		t.Errorf("StartAsCurrentRetriever() Type = %s, want %s", obs.Type, ObservationTypeRetriever)
	}

	retrieved, ok := GetCurrentObservation(ctx)
	if !ok {
		t.Error("StartAsCurrentRetriever() observation not stored in context")
	}
	if retrieved.ID != obs.ID {
		t.Errorf("StartAsCurrentRetriever() retrieved ID = %s, want %s", retrieved.ID, obs.ID)
	}

	// Update with output
	err = obs.Update(SpanUpdate{
		Output: map[string]interface{}{
			"documents": []map[string]interface{}{
				{"id": "doc-async-101", "title": "AsyncIO Tutorial", "score": 0.94, "rerank_score": 0.97},
				{"id": "doc-async-patterns", "title": "Async Patterns in Python", "score": 0.89, "rerank_score": 0.91},
			},
			"total_candidates": 150,
			"filtered_count":   5,
			"latency_ms":       32,
		},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestStartAsCurrentEmbedding(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	ctx, obs, err := client.StartAsCurrentEmbedding(context.Background(), "batch-document-embedding", "text-embedding-3-small", map[string]interface{}{
		"documents": []string{
			"First document about machine learning fundamentals",
			"Second document covering neural network architectures",
			"Third document on transformer models",
		},
		"batch_size": 100,
		"normalize":  true,
		"truncation": "end",
		"max_tokens": 8192,
	})
	if err != nil {
		t.Fatalf("StartAsCurrentEmbedding() error = %v", err)
	}
	if obs.Type != ObservationTypeEmbedding {
		t.Errorf("StartAsCurrentEmbedding() Type = %s, want %s", obs.Type, ObservationTypeEmbedding)
	}

	retrieved, ok := GetCurrentObservation(ctx)
	if !ok {
		t.Error("StartAsCurrentEmbedding() observation not stored in context")
	}
	if retrieved.ID != obs.ID {
		t.Errorf("StartAsCurrentEmbedding() retrieved ID = %s, want %s", retrieved.ID, obs.ID)
	}

	// Update with output
	model := "text-embedding-3-small"
	err = obs.Update(GenerationUpdate{
		Model: &model,
		Output: map[string]interface{}{
			"embeddings_count":   3,
			"dimensions":         1536,
			"total_tokens":       45,
			"processing_time_ms": 120,
		},
		Usage: &Usage{Input: 45, Total: 45, Unit: "TOKENS"},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestStartAsCurrentEvaluator(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	ctx, obs, err := client.StartAsCurrentEvaluator(context.Background(), "factual-consistency-evaluator", map[string]interface{}{
		"response":  "The Eiffel Tower is 330 meters tall and was completed in 1889.",
		"reference": "The Eiffel Tower, located in Paris, stands at 330 meters and was finished in 1889 for the World's Fair.",
		"metrics":   []string{"factual_consistency", "completeness", "faithfulness"},
		"model":     "gpt-4-judge",
	})
	if err != nil {
		t.Fatalf("StartAsCurrentEvaluator() error = %v", err)
	}
	if obs.Type != ObservationTypeEvaluator {
		t.Errorf("StartAsCurrentEvaluator() Type = %s, want %s", obs.Type, ObservationTypeEvaluator)
	}

	retrieved, ok := GetCurrentObservation(ctx)
	if !ok {
		t.Error("StartAsCurrentEvaluator() observation not stored in context")
	}
	if retrieved.ID != obs.ID {
		t.Errorf("StartAsCurrentEvaluator() retrieved ID = %s, want %s", retrieved.ID, obs.ID)
	}

	// Update with output
	err = obs.Update(SpanUpdate{
		Output: map[string]interface{}{
			"scores": map[string]float64{
				"factual_consistency": 1.0,
				"completeness":        0.85,
				"faithfulness":        0.95,
			},
			"overall":     0.93,
			"verdict":     "pass",
			"explanation": "Response is factually correct. Minor completeness deduction for omitting the World's Fair context.",
		},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestStartAsCurrentGuardrail(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	ctx, obs, err := client.StartAsCurrentGuardrail(context.Background(), "output-content-filter", map[string]interface{}{
		"content": "Here's how to write effective Python code: use meaningful variable names, follow PEP 8 style guide, and write comprehensive tests.",
		"policy":  "general_content_safety",
		"checks": []string{
			"harmful_content",
			"bias_detection",
			"copyright_violation",
			"misinformation",
		},
		"severity_threshold": "medium",
	})
	if err != nil {
		t.Fatalf("StartAsCurrentGuardrail() error = %v", err)
	}
	if obs.Type != ObservationTypeGuardrail {
		t.Errorf("StartAsCurrentGuardrail() Type = %s, want %s", obs.Type, ObservationTypeGuardrail)
	}

	retrieved, ok := GetCurrentObservation(ctx)
	if !ok {
		t.Error("StartAsCurrentGuardrail() observation not stored in context")
	}
	if retrieved.ID != obs.ID {
		t.Errorf("StartAsCurrentGuardrail() retrieved ID = %s, want %s", retrieved.ID, obs.ID)
	}

	// Update with output
	err = obs.Update(SpanUpdate{
		Output: map[string]interface{}{
			"passed": true,
			"results": map[string]interface{}{
				"harmful_content":     map[string]interface{}{"detected": false, "confidence": 0.99},
				"bias_detection":      map[string]interface{}{"detected": false, "confidence": 0.97},
				"copyright_violation": map[string]interface{}{"detected": false, "confidence": 0.98},
				"misinformation":      map[string]interface{}{"detected": false, "confidence": 0.95},
			},
			"action":             "allow",
			"processing_time_ms": 18,
			"model_version":      "guardrail-v2.1",
		},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestObservation_Update_SpanLikeTypes(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	testCases := []struct {
		obsType ObservationType
		name    string
		input   map[string]interface{}
		output  map[string]interface{}
	}{
		{
			ObservationTypeSpan, "api-call-span",
			map[string]interface{}{"endpoint": "/api/v1/users", "method": "GET"},
			map[string]interface{}{"status": 200, "response_time_ms": 45, "users_count": 10},
		},
		{
			ObservationTypeAgent, "planning-agent",
			map[string]interface{}{"goal": "Book flight to Paris", "constraints": []string{"budget < $500", "direct flight"}},
			map[string]interface{}{"plan": []string{"search_flights", "compare_prices", "book_best_option"}, "confidence": 0.92},
		},
		{
			ObservationTypeTool, "calendar-tool",
			map[string]interface{}{"action": "create_event", "title": "Team meeting", "date": "2024-12-25"},
			map[string]interface{}{"event_id": "evt-123", "created": true, "calendar": "work"},
		},
		{
			ObservationTypeChain, "qa-chain",
			map[string]interface{}{"question": "What is the capital of France?", "context_docs": 3},
			map[string]interface{}{"answer": "Paris", "confidence": 0.99, "sources": []string{"doc-1", "doc-2"}},
		},
		{
			ObservationTypeRetriever, "semantic-retriever",
			map[string]interface{}{"query": "machine learning tutorials", "top_k": 5},
			map[string]interface{}{"documents": []string{"doc-ml-101", "doc-ml-102"}, "latency_ms": 28},
		},
		{
			ObservationTypeEvaluator, "coherence-evaluator",
			map[string]interface{}{"text": "The model performed well on all benchmarks.", "criteria": "coherence"},
			map[string]interface{}{"score": 0.95, "reasoning": "Clear and logical structure"},
		},
		{
			ObservationTypeGuardrail, "pii-filter",
			map[string]interface{}{"text": "Contact John at john@email.com", "check": "pii"},
			map[string]interface{}{"pii_detected": true, "entities": []string{"email"}, "action": "redact"},
		},
	}

	for _, tc := range testCases {
		t.Run(string(tc.obsType), func(t *testing.T) {
			obs, err := client.StartObservation(context.Background(), tc.obsType, tc.name, tc.input)
			if err != nil {
				t.Fatalf("StartObservation() error = %v", err)
			}

			err = obs.Update(SpanUpdate{
				Output: tc.output,
			})
			if err != nil {
				t.Errorf("Update() error = %v", err)
			}
		})
	}
}

func TestObservation_Update_GenerationLikeTypes(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	testCases := []struct {
		obsType ObservationType
		name    string
		model   string
		input   map[string]interface{}
		output  map[string]interface{}
		usage   *Usage
	}{
		{
			ObservationTypeGeneration,
			"chat-completion",
			"gpt-4-turbo",
			map[string]interface{}{
				"messages": []map[string]string{
					{"role": "system", "content": "You are a helpful coding assistant."},
					{"role": "user", "content": "Write a Python function to calculate factorial."},
				},
				"temperature": 0.7,
				"max_tokens":  500,
			},
			map[string]interface{}{
				"response":      "def factorial(n):\n    if n <= 1:\n        return 1\n    return n * factorial(n - 1)",
				"finish_reason": "stop",
			},
			&Usage{Input: 45, Output: 28, Total: 73, Unit: "TOKENS"},
		},
		{
			ObservationTypeEmbedding,
			"text-embedding",
			"text-embedding-3-large",
			map[string]interface{}{
				"text":       "Semantic search enables finding content based on meaning.",
				"dimensions": 3072,
			},
			map[string]interface{}{
				"embedding_sample": []float64{0.0123, -0.0456, 0.0789},
				"normalized":       true,
			},
			&Usage{Input: 10, Total: 10, Unit: "TOKENS"},
		},
	}

	for _, tc := range testCases {
		t.Run(string(tc.obsType), func(t *testing.T) {
			obs, err := client.StartObservation(context.Background(), tc.obsType, tc.name, tc.input)
			if err != nil {
				t.Fatalf("StartObservation() error = %v", err)
			}

			model := tc.model
			err = obs.Update(GenerationUpdate{
				Model:  &model,
				Output: tc.output,
				Usage:  tc.usage,
			})
			if err != nil {
				t.Errorf("Update() error = %v", err)
			}
		})
	}
}

func TestObservation_End_AllTypes(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	testCases := []struct {
		obsType ObservationType
		name    string
		input   map[string]interface{}
	}{
		{ObservationTypeSpan, "http-request", map[string]interface{}{"url": "https://api.example.com/data", "method": "POST"}},
		{ObservationTypeGeneration, "llm-call", map[string]interface{}{"prompt": "Summarize this article", "model": "gpt-4"}},
		{ObservationTypeEvent, "user-click", map[string]interface{}{"element": "submit-button", "page": "/checkout"}},
		{ObservationTypeAgent, "research-agent", map[string]interface{}{"task": "Research latest AI trends", "depth": "comprehensive"}},
		{ObservationTypeTool, "search-api", map[string]interface{}{"query": "langfuse documentation", "engine": "google"}},
		{ObservationTypeChain, "translate-chain", map[string]interface{}{"text": "Hello world", "source": "en", "target": "es"}},
		{ObservationTypeRetriever, "doc-retriever", map[string]interface{}{"query": "API authentication", "collection": "docs"}},
		{ObservationTypeEmbedding, "query-embed", map[string]interface{}{"text": "How to use embeddings?", "model": "ada-002"}},
		{ObservationTypeEvaluator, "grammar-check", map[string]interface{}{"text": "This are a test.", "check": "grammar"}},
		{ObservationTypeGuardrail, "rate-limiter", map[string]interface{}{"user_id": "user-123", "action": "api_call", "window": "1m"}},
	}

	for _, tc := range testCases {
		t.Run(string(tc.obsType), func(t *testing.T) {
			obs, err := client.StartObservation(context.Background(), tc.obsType, tc.name, tc.input)
			if err != nil {
				t.Fatalf("StartObservation() error = %v", err)
			}

			err = obs.End()
			if err != nil {
				t.Errorf("End() error = %v", err)
			}
		})
	}
}

func TestObservation_StartChildObservation(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	// Parent agent orchestrating multiple tools
	parent, err := client.StartAgent(context.Background(), "travel-booking-agent", map[string]interface{}{
		"task":     "Book a round-trip flight from NYC to Paris",
		"budget":   1500,
		"currency": "USD",
		"dates": map[string]string{
			"departure": "2024-06-15",
			"return":    "2024-06-22",
		},
	})
	if err != nil {
		t.Fatalf("StartAgent() error = %v", err)
	}

	// Child tool for flight search
	child, err := parent.StartChildObservation(ObservationTypeTool, "flight-search-api", map[string]interface{}{
		"origin":      "JFK",
		"destination": "CDG",
		"date":        "2024-06-15",
		"passengers":  1,
		"class":       "economy",
	})
	if err != nil {
		t.Fatalf("StartChildObservation() error = %v", err)
	}

	if child.Type != ObservationTypeTool {
		t.Errorf("StartChildObservation() Type = %s, want %s", child.Type, ObservationTypeTool)
	}
	if child.TraceID != parent.TraceID {
		t.Errorf("StartChildObservation() TraceID = %s, want %s", child.TraceID, parent.TraceID)
	}
	if child.ParentSpanID != parent.ID {
		t.Errorf("StartChildObservation() ParentSpanID = %s, want %s", child.ParentSpanID, parent.ID)
	}

	// Update child with output
	err = child.Update(SpanUpdate{
		Output: map[string]interface{}{
			"flights": []map[string]interface{}{
				{"airline": "Air France", "price": 850, "departure": "08:00", "arrival": "21:30"},
				{"airline": "Delta", "price": 920, "departure": "10:30", "arrival": "23:45"},
			},
			"cheapest": 850,
			"count":    2,
		},
	})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestObservation_InvalidUpdateType(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	// Span-like type (Agent) with GenerationUpdate should fail
	spanObs, _ := client.StartAgent(context.Background(), "data-analysis-agent", map[string]interface{}{
		"task":    "Analyze sales data for Q4",
		"dataset": "sales_2024_q4.csv",
	})
	model := "gpt-4"
	err := spanObs.Update(GenerationUpdate{
		Model: &model,
		Output: map[string]interface{}{
			"analysis": "Sales increased by 15%",
		},
	})
	if err == nil {
		t.Error("Update() with GenerationUpdate on span-like type should return error")
	}

	// Generation-like type with SpanUpdate should fail
	genObs, _ := client.StartObservation(context.Background(), ObservationTypeGeneration, "text-generation", map[string]interface{}{
		"prompt":      "Write a haiku about programming",
		"max_tokens":  50,
		"temperature": 0.9,
	})
	err = genObs.Update(SpanUpdate{
		Output: map[string]interface{}{
			"haiku": "Code flows like water\nBugs emerge from the shadows\nDebug, test, repeat",
		},
	})
	if err == nil {
		t.Error("Update() with SpanUpdate on generation-like type should return error")
	}
}

func TestStartObservation_UnknownType(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	_, err := client.StartObservation(context.Background(), ObservationType("unknown_observation_type"), "mystery-observation", map[string]interface{}{
		"description": "This observation type does not exist",
		"should_fail": true,
	})
	if err == nil {
		t.Error("StartObservation() with unknown type should return error")
	}
}

func TestObservation_Context(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	obs, err := client.StartObservation(context.Background(), ObservationTypeSpan, "ctx-test", nil)
	if err != nil {
		t.Fatalf("StartObservation() error = %v", err)
	}

	ctx := obs.Context()
	if ctx == nil {
		t.Fatal("Observation.Context() returned nil")
	}
	// The context should carry the trace info
	traceCtx, ok := GetTraceContext(ctx)
	if !ok {
		t.Error("Observation.Context() should embed trace context")
	}
	if traceCtx.TraceID == "" {
		t.Error("Observation.Context() trace context should have a TraceID")
	}
}

func TestUpdateCurrentSpan_NoContext(t *testing.T) {
	client, err := NewClient(Config{PublicKey: "pk-test", SecretKey: "sk-test"})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = client.UpdateCurrentSpan(context.Background(), "output", nil)
	if err == nil {
		t.Error("UpdateCurrentSpan() expected error when no observation in context, got nil")
	}
}

func TestUpdateCurrentSpan_ViaTraceContext(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	// Put a TraceContext (with SpanID) in context but no Observation
	ctx := WithTraceContext(context.Background(), TraceContext{
		TraceID: "trace-abc",
		SpanID:  "span-xyz",
	})

	err := client.UpdateCurrentSpan(ctx, "output-value", map[string]interface{}{"key": "val"})
	if err != nil {
		t.Errorf("UpdateCurrentSpan() via trace context error = %v", err)
	}
}

func TestUpdateCurrentSpan_WithSpanObservation(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	obs, err := client.StartObservation(context.Background(), ObservationTypeSpan, "span-obs", nil)
	if err != nil {
		t.Fatalf("StartObservation() error = %v", err)
	}

	ctx := WithCurrentObservation(obs.Context(), obs)
	err = client.UpdateCurrentSpan(ctx, "span-output", nil)
	if err != nil {
		t.Errorf("UpdateCurrentSpan() with span observation error = %v", err)
	}
}

func TestUpdateCurrentSpan_WithGenerationObservation(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	obs, err := client.StartObservation(context.Background(), ObservationTypeGeneration, "gen-obs", nil)
	if err != nil {
		t.Fatalf("StartObservation() error = %v", err)
	}

	ctx := WithCurrentObservation(obs.Context(), obs)
	err = client.UpdateCurrentSpan(ctx, "gen-output", nil)
	if err != nil {
		t.Errorf("UpdateCurrentSpan() with generation observation error = %v", err)
	}
}

func TestStartAsCurrentSpan(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	ctx, obs, err := client.StartAsCurrentSpan(context.Background(), "current-span", map[string]interface{}{"input": "data"})
	if err != nil {
		t.Fatalf("StartAsCurrentSpan() error = %v", err)
	}
	if obs.Type != ObservationTypeSpan {
		t.Errorf("StartAsCurrentSpan() Type = %s, want span", obs.Type)
	}

	retrieved, ok := GetCurrentObservation(ctx)
	if !ok {
		t.Error("StartAsCurrentSpan() observation not stored in context")
	}
	if retrieved.ID != obs.ID {
		t.Errorf("StartAsCurrentSpan() retrieved ID = %s, want %s", retrieved.ID, obs.ID)
	}
}

func TestStartAsCurrentSpan_WithPropagatedAttributes(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	// Set propagated attributes in the context before starting
	ctx := WithPropagatedAttributes(context.Background(), PropagatedAttributes{
		SessionID: "session-1",
		Metadata:  map[string]interface{}{"env": "test"},
	})

	_, obs, err := client.StartAsCurrentSpan(ctx, "span-with-attrs", nil)
	if err != nil {
		t.Fatalf("StartAsCurrentSpan() with propagated attrs error = %v", err)
	}
	if obs == nil {
		t.Error("StartAsCurrentSpan() returned nil observation")
	}
}

func TestStartAsCurrentGeneration(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	ctx, obs, err := client.StartAsCurrentGeneration(context.Background(), "current-gen", "gpt-4", map[string]interface{}{"prompt": "hello"})
	if err != nil {
		t.Fatalf("StartAsCurrentGeneration() error = %v", err)
	}
	if obs.Type != ObservationTypeGeneration {
		t.Errorf("StartAsCurrentGeneration() Type = %s, want generation", obs.Type)
	}

	retrieved, ok := GetCurrentObservation(ctx)
	if !ok {
		t.Error("StartAsCurrentGeneration() observation not stored in context")
	}
	if retrieved.ID != obs.ID {
		t.Errorf("StartAsCurrentGeneration() retrieved ID = %s, want %s", retrieved.ID, obs.ID)
	}
}

func TestObservation_Update_UnknownType(t *testing.T) {
	obs := &Observation{
		Type: ObservationType("unknown"),
		ID:   "obs-1",
	}

	err := obs.Update(SpanUpdate{Output: "data"})
	if err == nil {
		t.Error("Update() with unknown type should return error")
	}
}

func TestObservation_End_UnknownType(t *testing.T) {
	obs := &Observation{
		Type: ObservationType("unknown"),
		ID:   "obs-1",
	}

	err := obs.End()
	if err == nil {
		t.Error("End() with unknown type should return error")
	}
}

func TestUpdateCurrentSpan_WithEventObservation(t *testing.T) {
	server, client := createTestServer(t)
	defer server.Close()

	obs, err := client.StartObservation(context.Background(), ObservationTypeEvent, "event-obs", nil)
	if err != nil {
		t.Fatalf("StartObservation() error = %v", err)
	}

	ctx := WithCurrentObservation(obs.Context(), obs)
	err = client.UpdateCurrentSpan(ctx, "event-output", nil)
	if err != nil {
		t.Errorf("UpdateCurrentSpan() with event observation should return nil, got error = %v", err)
	}
}
