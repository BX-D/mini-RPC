package middleware

import (
	"context"
	"golang.org/x/time/rate"
	"mini-rpc/message"
)

// RateLimitMiddleware creates a rate limiter using the token bucket algorithm.
//
// Token bucket: tokens are added at rate r per second, up to a burst size.
// Each request consumes one token. If the bucket is empty, the request is rejected.
// Unlike a leaky bucket (constant drain rate), token bucket allows short bursts
// of traffic — more suitable for RPC workloads with bursty patterns.
//
// CRITICAL: the limiter is created in the OUTER closure (once per middleware creation),
// NOT in the inner handler function. If created per-request, every request would get
// a fresh full bucket, defeating the entire purpose of rate limiting.
//
// Parameters:
//   - r: token refill rate (tokens per second)
//   - burst: maximum bucket size (allows this many requests in a burst)
func RateLimitMiddleware(r float64, burst int) Middleware {
	limiter := rate.NewLimiter(rate.Limit(r), burst) // Shared across all requests
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *message.RPCMessage) *message.RPCMessage {
			if !limiter.Allow() {
				// No tokens available — reject immediately (short-circuit, don't call next)
				return &message.RPCMessage{
					Error: "rate limit exceeded",
				}
			}
			return next(ctx, req)
		}
	}
}
