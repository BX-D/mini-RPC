package client

import (
	"mini-rpc/codec"
	"mini-rpc/loadbalance"
	"mini-rpc/middleware"
	"mini-rpc/registry"
	"mini-rpc/server"
	"testing"
	"time"
)

// ---- 测试用的服务 ----

type Args struct {
	A, B int
}

type Reply struct {
	Result int
}

type Arith struct{}

func (a *Arith) Add(args *Args, reply *Reply) error {
	reply.Result = args.A + args.B
	return nil
}

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

// ---- 测试 ----

func TestClientWithRegistryAndLB(t *testing.T) {
	// 1. 启动 Server
	svr := server.NewServer()
	svr.Use(middleware.LoggingMiddleware())
	err := svr.Register(&Arith{})
	if err != nil {
		t.Fatal(err)
	}
	go svr.Serve("tcp", ":18080", "", nil)
	time.Sleep(100 * time.Millisecond)

	// 2. Mock Registry：手动注册服务实例
	reg := NewMockRegistry()
	reg.Register("Arith", registry.ServiceInstance{Addr: "127.0.0.1:18080", Weight: 1}, 10)

	// 3. 创建 Client
	bal := &loadbalance.RoundRobinBalancer{}
	client := NewClient(reg, bal, byte(codec.CodecTypeJSON), 4)

	// 4. 调用 Arith.Add(1, 2) = 3
	reply := &Reply{}
	err = client.Call("Arith.Add", &Args{A: 1, B: 2}, reply)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Result != 3 {
		t.Fatalf("expect 3, got %v", reply.Result)
	}

	// 5. 再调用一次 Add(10, 20) = 30
	reply2 := &Reply{}
	err = client.Call("Arith.Add", &Args{A: 10, B: 20}, reply2)
	if err != nil {
		t.Fatal(err)
	}
	if reply2.Result != 30 {
		t.Fatalf("expect 30, got %v", reply2.Result)
	}

	t.Log("All integration tests passed!")
}

func TestClientMultipleInstances(t *testing.T) {
	// 启动 2 个 Server 实例
	svr1 := server.NewServer()
	svr1.Register(&Arith{})
	go svr1.Serve("tcp", ":18081", "", nil)

	svr2 := server.NewServer()
	svr2.Register(&Arith{})
	go svr2.Serve("tcp", ":18082", "", nil)

	time.Sleep(100 * time.Millisecond)

	// 注册 2 个实例
	reg := NewMockRegistry()
	reg.Register("Arith", registry.ServiceInstance{Addr: "127.0.0.1:18081", Weight: 1}, 10)
	reg.Register("Arith", registry.ServiceInstance{Addr: "127.0.0.1:18082", Weight: 1}, 10)

	// RoundRobin 负载均衡
	bal := &loadbalance.RoundRobinBalancer{}
	client := NewClient(reg, bal, byte(codec.CodecTypeJSON), 4)

	// 发 10 个请求，RoundRobin 应该交替打到两个 server
	for i := 0; i < 10; i++ {
		reply := &Reply{}
		err := client.Call("Arith.Add", &Args{A: i, B: i}, reply)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		if reply.Result != i*2 {
			t.Fatalf("request %d: expect %d, got %d", i, i*2, reply.Result)
		}
	}

	t.Log("Multi-instance load balancing test passed!")
}
