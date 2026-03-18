package middleware

import (
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// RateLimit returns a per-IP token-bucket rate limiter middleware.
// rps is the sustained requests-per-second; burst is the burst cap.
func RateLimit(rps float64, burst int) func(http.Handler) http.Handler {
	type visitor struct {
		limiter *rate.Limiter
	}

	var mu sync.Mutex
	visitors := make(map[string]*visitor)

	getLimiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		v, ok := visitors[ip]
		if !ok {
			v = &visitor{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
			visitors[ip] = v
		}
		return v.limiter
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				ip = fwd
			}
			if !getLimiter(ip).Allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
