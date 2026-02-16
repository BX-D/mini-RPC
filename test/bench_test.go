package test

import (
	"mini-rpc/client"
	"mini-rpc/codec"
	"mini-rpc/loadbalance"
	"mini-rpc/message"
	"mini-rpc/registry"
	"mini-rpc/server"
	"testing"
	"time"
)

// ---- Mock Registry（不依赖 etcd）----

type MockRegistry struct {
	instances map[string][]registry.ServiceInstance
}

func NewMockRegistry() *MockRegistry {
	return &MockRegistry{instances: make(map[string][]registry.ServiceInstance)}
}

func (m *MockRegistry) Register(serviceName string, inst registry.ServiceInstance, ttl int64) error {
	m.instances[serviceName] = append(m.instances[serviceName], inst)
	return nil
}

func (m *MockRegistry) Deregister(serviceName string, addr string) error {
	insts := m.instances[serviceName]
	for i, inst := range insts {
		if inst.Addr == addr {
			m.instances[serviceName] = append(insts[:i], insts[i+1:]...)
			break
		}
	}
	return nil
}

func (m *MockRegistry) Discover(serviceName string) ([]registry.ServiceInstance, error) {
	return m.instances[serviceName], nil
}

func (m *MockRegistry) Watch(serviceName string) <-chan []registry.ServiceInstance {
	return nil
}

// ---- Setup 公共函数 ----

func setupServerAndClient(b *testing.B, addr string) (*server.Server, *client.Client) {
	svr := server.NewServer()
	if err := svr.Register(&Arith{}); err != nil {
		b.Fatal(err)
	}
	go svr.Serve("tcp", addr, "", nil)
	time.Sleep(100 * time.Millisecond)

	reg := NewMockRegistry()
	reg.Register("Arith", registry.ServiceInstance{Addr: addr}, 10)

	bal := &loadbalance.RoundRobinBalancer{}
	cli := client.NewClient(reg, bal, byte(codec.CodecTypeJSON), 8)

	return svr, cli
}

// ---- Benchmark ----

// 场景1: 单 goroutine 串行调用
func BenchmarkSerialCall(b *testing.B) {
	svr, cli := setupServerAndClient(b, "127.0.0.1:29090")
	b.Cleanup(func() { svr.Shutdown(3 * time.Second) })

	args := &Args{A: 1, B: 2}
	reply := &Reply{}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := cli.Call("Arith.Add", args, reply); err != nil {
			b.Fatal(err)
		}
	}
}

// 场景2: 多 goroutine 并发调用（体现多路复用优势）
func BenchmarkConcurrentCall(b *testing.B) {
	svr, cli := setupServerAndClient(b, "127.0.0.1:29091")
	b.Cleanup(func() { svr.Shutdown(3 * time.Second) })

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		args := &Args{A: 1, B: 2}
		reply := &Reply{}
		for pb.Next() {
			if err := cli.Call("Arith.Add", args, reply); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

// 场景3: JSON 编解码性能（不走网络，纯 codec）
func BenchmarkCodecJSON(b *testing.B) {
	cdc := codec.GetCodec(codec.CodecTypeJSON)
	msg := &message.RPCMessage{
		ServiceMethod: "Arith.Add",
		Payload:       []byte(`{"A":1,"B":2}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := cdc.Encode(msg)
		var out message.RPCMessage
		cdc.Decode(data, &out)
	}
}

// 场景4: Binary 编解码性能（不走网络，纯 codec）
func BenchmarkCodecBinary(b *testing.B) {
	cdc := codec.GetCodec(codec.CodecTypeBinary)
	msg := &message.RPCMessage{
		ServiceMethod: "Arith.Add",
		Payload:       []byte(`{"A":1,"B":2}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := cdc.Encode(msg)
		var out message.RPCMessage
		cdc.Decode(data, &out)
	}
}
