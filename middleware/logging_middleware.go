package middleware

import (
	"context"
	"log"
	"mini-rpc/message"
	"time"
)

// LoggingMiddleware records the service method, duration, and any errors for each RPC call.
// It captures the start time before calling next, and logs the elapsed time after next returns.
//
// Example output:
//
//	ServiceMethod: Arith.Add, Duration: 42μs
//	Error: division by zero
func LoggingMiddleware() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *message.RPCMessage) *message.RPCMessage {
			start := time.Now()

			// Call the next handler in the chain
			rpcMessage := next(ctx, req)

			// Post-processing: log duration and errors
			duration := time.Since(start)
			log.Printf("ServiceMethod: %s, Duration: %s", req.ServiceMethod, duration)
			if rpcMessage.Error != "" {
				log.Printf("Error: %s", rpcMessage.Error)
			}
			return rpcMessage
		}
	}
}
