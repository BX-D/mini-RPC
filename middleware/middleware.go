// Package middleware implements the onion model middleware chain for mini-RPC.
//
// Middleware wraps the business handler to add cross-cutting concerns
// (logging, timeout, rate limiting) without modifying the handler itself.
//
// Onion model execution order:
//
//	Chain(A, B, C)(handler)  →  A(B(C(handler)))
//
//	Request:   A.before → B.before → C.before → handler
//	Response:  handler → C.after → B.after → A.after
//
// Each middleware can:
//   - Do pre-processing (before calling next)
//   - Call next(ctx, req) to pass to the next layer
//   - Do post-processing (after next returns)
//   - Short-circuit by returning early without calling next (e.g., rate limiting)
package middleware

import (
	"context"
	"mini-rpc/message"
)

// HandlerFunc is the function signature for request handlers.
// Both the business handler and middleware-wrapped handlers share this signature.
type HandlerFunc func(ctx context.Context, req *message.RPCMessage) *message.RPCMessage

// Middleware takes a handler and returns a new handler that wraps it.
// This is the decorator pattern — each middleware adds behavior around the next handler.
type Middleware func(next HandlerFunc) HandlerFunc

// Chain composes multiple middlewares into a single middleware.
// It builds the chain from right to left so that the first middleware in the list
// is the outermost layer (executed first on request, last on response).
//
// Example:
//
//	chain := Chain(Logging, Timeout, RateLimit)
//	handler := chain(businessHandler)
//	// Execution: Logging → Timeout → RateLimit → businessHandler → RateLimit → Timeout → Logging
func Chain(middlewares ...Middleware) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		// Build from right to left: wrap innermost first
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}
