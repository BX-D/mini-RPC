package middleware

import (
	"context"
	"log"
	"mini-rpc/message"
	"time"
)

func LoggingMiddleware() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *message.RPCMessage) *message.RPCMessage {
			// Log the incoming request
			start := time.Now()
			rpcMessage := next(ctx, req)
			// Print the service method and the time taken to process the request and error if any
			duration := time.Since(start)
			log.Printf("ServiceMethod: %s, Duration: %s", req.ServiceMethod, duration)
			if rpcMessage.Error != "" {
				log.Printf("Error: %s", rpcMessage.Error)
			}
			return rpcMessage
		}
	}
}
