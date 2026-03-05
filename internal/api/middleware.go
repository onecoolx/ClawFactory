package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/clawfactory/clawfactory/internal/model"
)

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
