package test

import (
	"mini-rpc/client"
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

func (a *Arith) Multiply(args *Args, reply *Reply) error {
	reply.Result = args.A * args.B
	return nil
}

// TestFullIntegrationWithEtcd 完整端到端测试
// 链路: Client → Registry(etcd) → LB → ConnPool → Protocol → Codec → Middleware → Server → 反射调用
func TestFullIntegrationWithEtcd(t *testing.T) {
	// 1. 连接 etcd
	reg, err := registry.NewEtcdRegistry([]string{"127.0.0.1:2379"})
	if err != nil {
		t.Fatalf("failed to connect etcd: %v", err)
	}

	// 2. 启动 Server，挂载中间件
	svr := server.NewServer()
	svr.Use(middleware.LoggingMiddleware())
	err = svr.Register(&Arith{})
	if err != nil {
		t.Fatal(err)
	}
	go svr.Serve("tcp", ":19090")
	time.Sleep(100 * time.Millisecond)

	// 3. 向 etcd 注册服务实例
	err = reg.Register("Arith", registry.ServiceInstance{
		Addr:   "127.0.0.1:19090",
		Weight: 10,
	}, 10)
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	// 4. 创建 Client（用同一个 registry 做服务发现）
	bal := &loadbalance.RoundRobinbalancer{}
	cli := client.NewClient(reg, bal, byte(codec.CodecTypeJSON))

	// 5. 测试 Add
	reply := &Reply{}
	err = cli.Call("Arith.Add", &Args{A: 3, B: 5}, reply)
	if err != nil {
		t.Fatalf("Call Add failed: %v", err)
	}
	if reply.Result != 8 {
		t.Fatalf("Add: expect 8, got %d", reply.Result)
	}

	// 6. 测试 Multiply
	reply2 := &Reply{}
	err = cli.Call("Arith.Multiply", &Args{A: 4, B: 6}, reply2)
	if err != nil {
		t.Fatalf("Call Multiply failed: %v", err)
	}
	if reply2.Result != 24 {
		t.Fatalf("Multiply: expect 24, got %d", reply2.Result)
	}

	t.Log("Full integration test with etcd passed!")

	// 7. 清理：注销 + 关闭 server
	svr.Shutdown(reg, 3*time.Second)
}

// TestMultiServerWithEtcd 多实例 + 负载均衡 + etcd
func TestMultiServerWithEtcd(t *testing.T) {
	// 1. 连接 etcd
	reg, err := registry.NewEtcdRegistry([]string{"127.0.0.1:2379"})
	if err != nil {
		t.Fatalf("failed to connect etcd: %v", err)
	}

	// 清理上个测试残留的 etcd 数据
	reg.Deregister("Arith", "127.0.0.1:19090")

	// 2. 启动 2 个 Server
	svr1 := server.NewServer()
	svr1.Register(&Arith{})
	go svr1.Serve("tcp", ":19091")

	svr2 := server.NewServer()
	svr2.Register(&Arith{})
	go svr2.Serve("tcp", ":19092")

	time.Sleep(100 * time.Millisecond)

	// 3. 注册 2 个实例到 etcd
	reg.Register("Arith", registry.ServiceInstance{Addr: "127.0.0.1:19091", Weight: 10}, 10)
	reg.Register("Arith", registry.ServiceInstance{Addr: "127.0.0.1:19092", Weight: 10}, 10)

	// 4. 创建 Client
	bal := &loadbalance.RoundRobinbalancer{}
	cli := client.NewClient(reg, bal, byte(codec.CodecTypeJSON))

	// 5. 发 10 个请求，验证全部正确
	for i := 1; i <= 10; i++ {
		reply := &Reply{}
		err := cli.Call("Arith.Add", &Args{A: i, B: i * 10}, reply)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		expected := i + i*10
		if reply.Result != expected {
			t.Fatalf("request %d: expect %d, got %d", i, expected, reply.Result)
		}
	}

	t.Log("Multi-server integration test with etcd passed!")

	// 6. 清理
	svr1.Shutdown(reg, 3*time.Second)
	svr2.Shutdown(reg, 3*time.Second)
}
