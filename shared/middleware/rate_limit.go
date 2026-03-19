package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// incrScript atomically increments a fixed-window counter and sets its TTL on
// first write. Returns the new counter value.
var incrScript = redis.NewScript(`
local key  = KEYS[1]
local ttl  = tonumber(ARGV[1])
local n    = redis.call("INCR", key)
if n == 1 then
    redis.call("EXPIRE", key, ttl)
end
return n
`)

// RateLimitRedis returns a Redis-backed fixed-window rate limiter middleware.
//
// keyPrefix distinguishes different rate-limit contexts (e.g. "auth:login").
// limit is the maximum number of requests allowed per window.
// window is the duration of each counting window.
//
// When the limit is exceeded the middleware responds with HTTP 429 and a
// Retry-After header indicating how many seconds remain in the current window.
// If Redis is unavailable the middleware fails open (requests are allowed
// through) to avoid a Redis outage bringing down the service.
func RateLimitRedis(rdb *redis.Client, keyPrefix string, limit int, window time.Duration) func(http.Handler) http.Handler {
	windowSecs := int64(window.Seconds())

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				ip = fwd
			}
			// Strip port — RemoteAddr is "host:port"; we key on IP only so all
			// connections from the same client share one counter.
			if host, _, err := net.SplitHostPort(ip); err == nil {
				ip = host
			}

			now := time.Now().Unix()
			bucket := now / windowSecs
			key := fmt.Sprintf("rl:%s:%s:%d", keyPrefix, ip, bucket)

			count, err := incrScript.Run(r.Context(), rdb, []string{key}, windowSecs).Int()
			if err != nil {
				// Fail open: a Redis error should not block legitimate requests.
				next.ServeHTTP(w, r)
				return
			}

			if count > limit {
				retryAfter := windowSecs - (now % windowSecs)
				w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit returns a per-IP in-memory token-bucket rate limiter middleware.
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
			if host, _, err := net.SplitHostPort(ip); err == nil {
				ip = host
			}
			if !getLimiter(ip).Allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
