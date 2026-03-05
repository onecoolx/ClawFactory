package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pgregory.net/rapid"
)

// Property 8: Token authentication consistency
// **Validates: Requirements 4.1, 4.3**
func TestProperty8_TokenAuthConsistency(t *testing.T) {
	validTokens := []string{"valid-token-1", "valid-token-2"}
	middleware := TokenAuthMiddleware(validTokens)

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
