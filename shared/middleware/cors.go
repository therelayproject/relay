package middleware

import (
	"net/http"
	"strings"
)

// CORS returns a middleware that sets cross-origin resource sharing headers.
//
// allowedOrigins is the list of permitted origins. Pass []string{"*"} to allow
// any origin (suitable for development). Otherwise only requests whose Origin
// header exactly matches one of the entries receive permissive headers; others
// receive the appropriate restrictive response.
//
// When allowedOrigins is empty the middleware is a no-op — all requests pass
// through without CORS headers, which is the safe default until origins are
// explicitly configured via CORS_ALLOWED_ORIGINS.
//
// The middleware handles OPTIONS preflight requests directly (204 No Content)
// so callers do not need to register OPTIONS routes. It also sets the standard
// CORS headers on every response so that WebSocket upgrade requests from a
// browser are accepted correctly.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	wildcard := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"

	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o = strings.TrimSpace(o); o != "" {
			allowed[o] = struct{}{}
		}
	}

	setHeaders := func(w http.ResponseWriter, origin string) {
		h := w.Header()
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Set("Access-Control-Max-Age", "86400")
		if wildcard {
			// When using wildcard origin credentials cannot be combined with *.
			// Use the reflected origin instead so browser credentials work.
			if origin != "" {
				h.Set("Access-Control-Allow-Origin", origin)
			} else {
				h.Set("Access-Control-Allow-Origin", "*")
				h.Set("Access-Control-Allow-Credentials", "false")
			}
		} else {
			h.Set("Access-Control-Allow-Origin", origin)
		}
		h.Add("Vary", "Origin")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Non-CORS request (no Origin header) — pass straight through.
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Origin check for non-wildcard configs.
			if !wildcard {
				if _, ok := allowed[origin]; !ok {
					// Origin not permitted — serve request without CORS headers.
					next.ServeHTTP(w, r)
					return
				}
			}

			setHeaders(w, origin)

			// Preflight — respond immediately.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ParseCORSOrigins splits a comma-separated list of allowed origins read from
// an environment variable. Whitespace around each entry is trimmed and empty
// entries are dropped. Returns nil when raw is empty, which causes the CORS
// middleware to become a no-op.
//
// Example: ParseCORSOrigins("https://app.relay.im, https://relay.im")
func ParseCORSOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
