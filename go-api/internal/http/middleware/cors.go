package middleware

import (
	"net/http"
	"strings"
)

const (
	headerOrigin                      = "Origin"
	headerVary                        = "Vary"
	headerACAO                        = "Access-Control-Allow-Origin"
	headerACAM                        = "Access-Control-Allow-Methods"
	headerACAH                        = "Access-Control-Allow-Headers"
	headerACAC                        = "Access-Control-Allow-Credentials"
	headerAccessControlRequestMethod  = "Access-Control-Request-Method"
	headerAccessControlRequestHeaders = "Access-Control-Request-Headers"
)

const defaultAllowedHeaders = "Authorization, Content-Type, Idempotency-Key, X-Request-ID"
const defaultAllowedMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"

func CORS(next http.Handler, allowedOrigins []string) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(headerVary, headerOrigin)
		w.Header().Add(headerVary, headerAccessControlRequestMethod)
		w.Header().Add(headerVary, headerAccessControlRequestHeaders)

		origin := strings.TrimSpace(r.Header.Get(headerOrigin))
		_, originAllowed := allowed[origin]
		if originAllowed {
			w.Header().Set(headerACAO, origin)
			w.Header().Set(headerACAM, defaultAllowedMethods)
			w.Header().Set(headerACAH, defaultAllowedHeaders)
			w.Header().Set(headerACAC, "true")
		}

		if r.Method == http.MethodOptions && origin != "" {
			if originAllowed {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
