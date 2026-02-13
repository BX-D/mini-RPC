package middleware

import (
	"context"
	"golang.org/x/time/rate"
	"mini-rpc/message"
)

// RateLimitMiddleware 创建一个基于令牌桶算法的限流中间件
func RateLimitMiddleware(r float64, burst int) Middleware {
	limiter := rate.NewLimiter(rate.Limit(r), burst)
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *message.RPCMessage) *message.RPCMessage {
			if !limiter.Allow() {
				return &message.RPCMessage{
					Error: "rate limit exceeded",
				}
			}
			return next(ctx, req)
		}
	}

}
