package transport

import (
	"encoding/json"
	"mini-rpc/codec"
	"mini-rpc/server"
	"net"
	"sync"
	"testing"
	"time"
)

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

// 测试单连接上串行发送多个请求
func TestClientTransportSerial(t *testing.T) {
	svr := server.NewServer()
	if err := svr.Register(&Arith{}); err != nil {
		t.Fatal(err)
	}
	go svr.Serve("tcp", ":9001", "", nil)
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", ":9001")
	if err != nil {
		t.Fatal(err)
	}

	ct := NewClientTransport(conn, codec.CodecTypeJSON)

	// 串行发 3 个请求
	cases := []struct {
		a, b, expect int
	}{
		{1, 2, 3},
		{10, 20, 30},
		{100, 200, 300},
	}

	for _, tc := range cases {
		_, ch, err := ct.Send("Arith.Add", &Args{A: tc.a, B: tc.b})
		if err != nil {
			t.Fatal(err)
		}

		resp := <-ch
		if resp.Error != "" {
			t.Fatalf("server error: %s", resp.Error)
		}

		var reply Reply
		if err := json.Unmarshal(resp.Payload, &reply); err != nil {
			t.Fatal(err)
		}

		if reply.Result != tc.expect {
			t.Fatalf("expect %d, got %d", tc.expect, reply.Result)
		}
	}
}

// 测试单连接上并发发送多个请求（多路复用核心测试）
func TestClientTransportConcurrent(t *testing.T) {
	svr := server.NewServer()
	if err := svr.Register(&Arith{}); err != nil {
		t.Fatal(err)
	}
	go svr.Serve("tcp", ":9002", "", nil)
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", ":9002")
	if err != nil {
		t.Fatal(err)
	}

	ct := NewClientTransport(conn, codec.CodecTypeJSON)

	// 并发发 50 个请求
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			_, ch, err := ct.Send("Arith.Add", &Args{A: n, B: n})
			if err != nil {
				t.Errorf("send failed: %v", err)
				return
			}

			resp := <-ch
			if resp.Error != "" {
				t.Errorf("server error: %s", resp.Error)
				return
			}

			var reply Reply
			if err := json.Unmarshal(resp.Payload, &reply); err != nil {
				t.Errorf("unmarshal failed: %v", err)
				return
			}

			if reply.Result != n*2 {
				t.Errorf("expect %d, got %d", n*2, reply.Result)
			}
		}(i)
	}

	wg.Wait()
}
