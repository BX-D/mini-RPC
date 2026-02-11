package client

import (
	"mini-rpc/codec"
	"mini-rpc/server"
	"net"
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

func TestClientCall(t *testing.T) {
	// Start server
	svr := server.NewServer()
	err := svr.Register(&Arith{})
	if err != nil {
		t.Fatal(err)
	}
	go svr.Serve("tcp", ":8889")
	time.Sleep(100 * time.Millisecond)

	// Create client
	conn, err := net.Dial("tcp", ":8889")
	if err != nil {
		t.Fatal(err)
	}

	client := NewClient(conn, byte(codec.CodecTypeJSON))

	// Call Arith.Add(1, 2) = 3
	reply := &Reply{}
	err = client.Call("Arith.Add", &Args{A: 1, B: 2}, reply)
	if err != nil {
		t.Fatal(err)
	}

	if reply.Result != 3 {
		t.Fatalf("expect 3, got %v", reply.Result)
	}

	// Call again: Add(10, 20) = 30
	reply2 := &Reply{}
	err = client.Call("Arith.Add", &Args{A: 10, B: 20}, reply2)
	if err != nil {
		t.Fatal(err)
	}

	if reply2.Result != 30 {
		t.Fatalf("expect 30, got %v", reply2.Result)
	}
}

func TestClientCallWithBinaryCodec(t *testing.T) {
	svr := server.NewServer()
	err := svr.Register(&Arith{})
	if err != nil {
		t.Fatal(err)
	}
	go svr.Serve("tcp", ":8890")
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", ":8890")
	if err != nil {
		t.Fatal(err)
	}

	client := NewClient(conn, byte(codec.CodecTypeBinary))

	reply := &Reply{}
	err = client.Call("Arith.Add", &Args{A: 5, B: 7}, reply)
	if err != nil {
		t.Fatal(err)
	}

	if reply.Result != 12 {
		t.Fatalf("expect 12, got %v", reply.Result)
	}
}
