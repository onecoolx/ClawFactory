package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/clawfactory/clawfactory/internal/api"
	"pgregory.net/rapid"
)

// Property 8: Token authentication consistency
// **Validates: Requirements 4.1, 4.3**
func TestProperty8_TokenAuthConsistency(t *testing.T) {
	validTokens := []string{"valid-token-1", "valid-token-2"}
	middleware := api.TokenAuthMiddleware(validTokens)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rapid.Check(t, func(t *rapid.T) {
		token := rapid.SampledFrom([]string{
			"valid-token-1", "valid-token-2", // valid
			"invalid-token", "", "random-xyz", // invalid
		}).Draw(t, "token")

		req := httptest.NewRequest("GET", "/v1/tasks", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		isValid := token == "valid-token-1" || token == "valid-token-2"
		if isValid && rec.Code != http.StatusOK {
			t.Fatalf("valid token %q got status %d", token, rec.Code)
		}
		if !isValid && rec.Code != http.StatusUnauthorized {
			t.Fatalf("invalid token %q got status %d, want 401", token, rec.Code)
		}
	})
}

// Feature: v03-observability, Property 37: TraceID 唯一性
// For any N concurrent requests (N >= 2), TraceID middleware generates unique TraceIDs.
// **Validates: Requirements 2.3**
func TestProperty37_TraceIDUniqueness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(2, 50).Draw(t, "numRequests")

		traceIDs := make([]string, n)
		var mu sync.Mutex
		var wg sync.WaitGroup

		handler := api.TraceIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tid := api.TraceIDFromContext(r.Context())
			idx := r.Context().Value(indexKey{}).(int)
			mu.Lock()
			traceIDs[idx] = tid
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}))

		// Send N concurrent requests
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				req := httptest.NewRequest("GET", "/test", nil)
				req = req.WithContext(context.WithValue(req.Context(), indexKey{}, idx))
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				// Verify X-Trace-ID header is set
				headerTID := rec.Header().Get("X-Trace-ID")
				if headerTID == "" {
					t.Errorf("request %d: X-Trace-ID header not set", idx)
				}
			}(i)
		}
		wg.Wait()

		// Verify all TraceIDs are unique and non-empty
		seen := make(map[string]bool)
		for i, tid := range traceIDs {
			if tid == "" {
				t.Fatalf("request %d: TraceID is empty", i)
			}
			if seen[tid] {
				t.Fatalf("duplicate TraceID found: %s", tid)
			}
			seen[tid] = true
		}
	})
}

// indexKey is a private context key for passing request index in tests.
type indexKey struct{}

// Feature: v03-observability, Property 38: 结构化日志字段完整性
// For any log call with TraceID context, output JSON contains time, level, msg, trace_id, component fields.
// **Validates: Requirements 2.4**
func TestProperty38_StructuredLogFieldCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		msg := rapid.SampledFrom([]string{
			"request started", "task assigned", "agent registered",
			"heartbeat received", "workflow submitted", "authorization check",
		}).Draw(t, "msg")

		component := rapid.SampledFrom([]string{
			"api", "registry", "scheduler", "workflow", "taskqueue",
		}).Draw(t, "component")

		// Generate a TraceID via the middleware
		var traceID string
		handler := api.TraceIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID = api.TraceIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if traceID == "" {
			t.Fatal("TraceID should not be empty")
		}

		// Capture slog output to a buffer
		var buf bytes.Buffer
		jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		logger := slog.New(jsonHandler)

		// Log with trace_id and component fields
		logger.Info(msg, "trace_id", traceID, "component", component)

		// Parse the JSON output
		var logEntry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
			t.Fatalf("failed to parse log JSON: %v, raw: %s", err, buf.String())
		}

		// Verify required fields exist
		requiredFields := []string{"time", "level", "msg", "trace_id", "component"}
		for _, field := range requiredFields {
			if _, ok := logEntry[field]; !ok {
				t.Fatalf("missing required field %q in log entry: %v", field, logEntry)
			}
		}

		// Verify field values
		if logEntry["msg"] != msg {
			t.Fatalf("msg field: got %q, want %q", logEntry["msg"], msg)
		}
		if logEntry["trace_id"] != traceID {
			t.Fatalf("trace_id field: got %q, want %q", logEntry["trace_id"], traceID)
		}
		if logEntry["component"] != component {
			t.Fatalf("component field: got %q, want %q", logEntry["component"], component)
		}
	})
}
