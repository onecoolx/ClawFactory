package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/clawfactory/clawfactory/internal/metrics"
	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/go-chi/chi/v5"
)

// traceIDKey is a private context key type for storing TraceID.
type traceIDKey struct{}

// generateUUID generates a UUID v4 string using crypto/rand.
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// TraceIDMiddleware generates a unique TraceID for each request,
// stores it in the context, and sets the X-Trace-ID response header.
func TraceIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := generateUUID()
		ctx := context.WithValue(r.Context(), traceIDKey{}, traceID)
		w.Header().Set("X-Trace-ID", traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TraceIDFromContext extracts the TraceID from the request context.
// Returns an empty string if no TraceID is present.
func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey{}).(string); ok {
		return v
	}
	return ""
}

// responseWriterWrapper wraps http.ResponseWriter to capture the status code.
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *responseWriterWrapper) WriteHeader(code int) {
	if !w.written {
		w.statusCode = code
		w.written = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	if !w.written {
		w.statusCode = http.StatusOK
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}

// MetricsMiddleware records HTTP request duration and count using the MetricsCollector.
// It uses chi.RouteContext to get the route pattern to avoid high-cardinality labels.
func MetricsMiddleware(mc metrics.MetricsCollector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(ww, r)

			duration := time.Since(start).Seconds()
			// Use route pattern from chi to avoid high-cardinality labels
			pattern := r.URL.Path
			if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
				pattern = rctx.RoutePattern()
			}
			mc.IncHTTPRequestTotal(r.Method, pattern, ww.statusCode)
			mc.ObserveHTTPRequestDuration(r.Method, pattern, duration)
		})
	}
}

// TokenAuthMiddleware validates API tokens.
func TokenAuthMiddleware(validTokens []string) func(http.Handler) http.Handler {
	tokenSet := make(map[string]bool)
	for _, t := range validTokens {
		tokenSet[t] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			token := strings.TrimPrefix(auth, "Bearer ")
			if token == "" || !tokenSet[token] {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or missing API token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// writeError writes a unified error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(model.ErrorResponse{
		Error: model.ErrorDetail{Code: code, Message: message},
	})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
