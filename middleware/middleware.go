package middleware

import (
	"context"
	"mini-rpc/message"
)

type HandlerFunc func(ctx context.Context, req *message.RPCMessage) *message.RPCMessage

type Middleware func(next HandlerFunc) HandlerFunc

// Chain 将多个中间件组合成一个中间件
func Chain(middlewares ...Middleware) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}
