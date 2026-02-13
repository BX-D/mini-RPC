package middleware

import (
	"context"
	"mini-rpc/message"
	"time"
)

func TimeOutMiddleware(timeout time.Duration) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *message.RPCMessage) *message.RPCMessage {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			done := make(chan *message.RPCMessage, 1)
			go func() {
				done <- next(ctx, req)
			}()

			select {
			case rpcMessage := <-done:
				return rpcMessage
			case <-ctx.Done():
				return &message.RPCMessage{
					Error: "request timed out",
				}
			}
		}
	}
}
