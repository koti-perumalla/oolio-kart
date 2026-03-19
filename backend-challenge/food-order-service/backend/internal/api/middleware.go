package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"os"
)

// requireAPIKey enforces the api_key header using a constant-time SHA-256 comparison
// to prevent timing attacks. Set the API_KEY environment variable to the expected key.
func requireAPIKey(next http.HandlerFunc) http.HandlerFunc {
	expected := sha256.Sum256([]byte(os.Getenv("API_KEY")))
	return func(w http.ResponseWriter, r *http.Request) {
		incoming := sha256.Sum256([]byte(r.Header.Get("api_key")))
		if subtle.ConstantTimeCompare(incoming[:], expected[:]) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
