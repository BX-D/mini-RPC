package middleware

import (
	"context"
	"mini-rpc/message"
	"time"
)

// TimeOutMiddleware enforces a maximum duration for each RPC call.
// If the handler doesn't complete within the timeout, it returns an error immediately.
//
// Implementation:
//  1. Create a context with timeout (ctx.Done() fires when timeout expires)
//  2. Run the next handler in a goroutine, sending its result to a channel
//  3. Select between the result channel and ctx.Done()
//
// Note: the handler goroutine is NOT cancelled — it continues running in the background.
// The timeout only controls when the caller gives up waiting. For true cancellation,
// the handler must check ctx.Done() internally.
func TimeOutMiddleware(timeout time.Duration) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *message.RPCMessage) *message.RPCMessage {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Run handler in a goroutine so we can race it against the timeout
			done := make(chan *message.RPCMessage, 1) // Buffered: prevent goroutine leak if timeout fires
			go func() {
				done <- next(ctx, req)
			}()

			select {
			case rpcMessage := <-done:
				return rpcMessage // Handler completed before timeout
			case <-ctx.Done():
				return &message.RPCMessage{
					Error: "request timed out",
				}
			}
		}
	}
}
