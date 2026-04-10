package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

const internalSecretHeader = "X-Internal-Secret"

// RequireInternalSecret protects internal-only endpoints with a shared secret.
// The secret is read from INTERNAL_API_SECRET.
func RequireInternalSecret(next http.Handler) http.Handler {
	secret := strings.TrimSpace(os.Getenv("INTERNAL_API_SECRET"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret == "" {
			http.Error(w, `{"error":"internal secret is not configured"}`, http.StatusInternalServerError)
			return
		}
		given := strings.TrimSpace(r.Header.Get(internalSecretHeader))
		if given == "" || subtle.ConstantTimeCompare([]byte(given), []byte(secret)) != 1 {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

