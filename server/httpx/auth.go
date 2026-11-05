package httpx

import (
	"net/http"
	"strings"

	"github.com/digitizables/lattice/server/auth"
)

// WithOperatorAuth gates OperatorService RPCs behind a bearer token.
// Login and /healthz stay open. When auth is disabled, all requests pass.
func WithOperatorAuth(next http.Handler, a *auth.Auth) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a == nil || a.Disabled() {
			next.ServeHTTP(w, r)
			return
		}
		path := r.URL.Path
		if path == "/healthz" || strings.HasSuffix(path, "/Login") {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.Contains(path, "OperatorService") {
			next.ServeHTTP(w, r)
			return
		}
		tok := auth.TokenFromRequest(r)
		if !a.Valid(tok) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
