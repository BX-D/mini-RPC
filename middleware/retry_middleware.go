package middleware

import (
	"context"
	"log"
	"mini-rpc/message"
	"strings"
	"time"
)

func RetryMiddleware(maxRetries int, baseDelay time.Duration) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *message.RPCMessage) *message.RPCMessage {
			rpcMessage := next(ctx, req)
			for i := 0; i < maxRetries; i++ {
				if rpcMessage.Error == "" {
					return rpcMessage // Success, return response
				}
				if strings.Contains(rpcMessage.Error, "timeout") || strings.Contains(rpcMessage.Error, "connection refused") {
					// Log the retry attempt
					log.Printf("Retry attempt %d for %s due to error: %s", i+1, req.ServiceMethod, rpcMessage.Error)
					time.Sleep(baseDelay * time.Duration(1<<i)) // Exponential backoff
					rpcMessage = next(ctx, req)                 // Retry the request
				} else {
					return rpcMessage // Non-retryable error, return immediately
				}
			}
			return rpcMessage // Return last response after retries
		}
	}
}
