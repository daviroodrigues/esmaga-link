package http

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"esmaga-link/internal/service"
	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client *redis.Client
}

func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

func (rl *RateLimiter) Limit(scope string, maxRequests int64, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identifier := getRealIP(r)
			if claims, ok := r.Context().Value(UserClaimsKey).(*service.TokenClaims); ok && claims != nil {
				identifier = "user:" + claims.UserID.String()
			}

			windowBucket := time.Now().Unix() / int64(window.Seconds())
			key := fmt.Sprintf("esmaga:ratelimit:%s:%s:%d", scope, identifier, windowBucket)

			ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
			defer cancel()

			pipe := rl.client.Pipeline()
			incrCmd := pipe.Incr(ctx, key)
			pipe.Expire(ctx, key, window*2)
			_, err := pipe.Exec(ctx)

			if err != nil {

				next.ServeHTTP(w, r)
				return
			}

			currentCount := incrCmd.Val()
			remaining := maxRequests - currentCount
			if remaining < 0 {
				remaining = 0
			}

			resetSeconds := int64(window.Seconds()) - (time.Now().Unix() % int64(window.Seconds()))

			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(maxRequests, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Unix()+resetSeconds, 10))

			if currentCount > maxRequests {
				w.Header().Set("Retry-After", strconv.FormatInt(resetSeconds, 10))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error": "muitas requisições, por favor tente novamente mais tarde"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
