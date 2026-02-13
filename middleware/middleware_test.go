package middleware

import (
	"context"
	"mini-rpc/message"
	"testing"
	"time"
)

// 模拟一个简单的 handler：直接返回成功响应
func echoHandler(ctx context.Context, req *message.RPCMessage) *message.RPCMessage {
	return &message.RPCMessage{
		ServiceMethod: req.ServiceMethod,
		Payload:       []byte("ok"),
	}
}

// 模拟一个慢 handler：睡 200ms
func slowHandler(ctx context.Context, req *message.RPCMessage) *message.RPCMessage {
	time.Sleep(200 * time.Millisecond)
	return &message.RPCMessage{
		ServiceMethod: req.ServiceMethod,
		Payload:       []byte("ok"),
	}
}

func TestLogging(t *testing.T) {
	handler := LoggingMiddleware()(echoHandler)

	req := &message.RPCMessage{ServiceMethod: "Arith.Add"}
	resp := handler(context.Background(), req)

	if resp == nil {
		t.Fatal("expect non-nil response")
	}
	if string(resp.Payload) != "ok" {
		t.Fatalf("expect payload 'ok', got '%s'", string(resp.Payload))
	}
}

func TestTimeoutPass(t *testing.T) {
	// 超时 500ms，handler 很快，应该正常返回
	handler := TimeOutMiddleware(500 * time.Millisecond)(echoHandler)

	req := &message.RPCMessage{ServiceMethod: "Arith.Add"}
	resp := handler(context.Background(), req)

	if resp.Error != "" {
		t.Fatalf("expect no error, got '%s'", resp.Error)
	}
}

func TestTimeoutExceeded(t *testing.T) {
	// 超时 50ms，handler 需要 200ms，应该超时
	handler := TimeOutMiddleware(50 * time.Millisecond)(slowHandler)

	req := &message.RPCMessage{ServiceMethod: "Arith.Add"}
	resp := handler(context.Background(), req)

	if resp.Error != "request timed out" {
		t.Fatalf("expect timeout error, got '%s'", resp.Error)
	}
}

func TestRateLimit(t *testing.T) {
	// rate=1 per second, burst=2 → 前 2 个立刻放行，第 3 个被拒
	handler := RateLimitMiddleware(1, 2)(echoHandler)
	req := &message.RPCMessage{ServiceMethod: "Arith.Add"}

	// 前 2 个应该通过（burst=2）
	for i := 0; i < 2; i++ {
		resp := handler(context.Background(), req)
		if resp.Error != "" {
			t.Fatalf("request %d should pass, got error: %s", i, resp.Error)
		}
	}

	// 第 3 个应该被限流
	resp := handler(context.Background(), req)
	if resp.Error != "rate limit exceeded" {
		t.Fatalf("request 3 should be rate limited, got: '%s'", resp.Error)
	}
}

func TestChain(t *testing.T) {
	// 用 Chain 组合 Logging + Timeout，验证请求能正常穿过
	chained := Chain(LoggingMiddleware(), TimeOutMiddleware(500*time.Millisecond))
	handler := chained(echoHandler)

	req := &message.RPCMessage{ServiceMethod: "Arith.Add"}
	resp := handler(context.Background(), req)

	if resp == nil {
		t.Fatal("expect non-nil response")
	}
	if resp.Error != "" {
		t.Fatalf("expect no error, got '%s'", resp.Error)
	}
}
