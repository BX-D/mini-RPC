package server

import (
	"encoding/json"
	"fmt"
	"mini-rpc/codec"
	"mini-rpc/message"
	"mini-rpc/protocol"
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

func TestServer(t *testing.T) {
	// Start a server
	svr := NewServer()

	go svr.Serve("tcp", ":8888", "", nil)

	err := svr.Register(&Arith{})

	if err != nil {
		t.Fatalf("Failed to regist method")
	}

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", ":8888")

	if err != nil {
		t.Error(err)
	}

	payload, err := json.Marshal(&Args{1, 2})
	if err != nil {
		t.Error(err)
	}

	rpcMessage := message.RPCMessage{
		ServiceMethod: "Arith.Add",
		Error:         "",
		Payload:       payload,
	}

	cdc := codec.GetCodec(codec.CodecType(protocol.CodecTypeJSON))

	body, err := cdc.Encode(&rpcMessage)

	if err != nil {
		t.Error(err)
	}

	header := protocol.Header{
		CodecType: protocol.CodecTypeJSON,
		MsgType:   protocol.MsgTypeRequest,
		Seq:       uint32(123),
		BodyLen:   uint32(len(body)),
	}

	err = protocol.Encode(conn, &header, body)

	if err != nil {
		t.Error(err)
	}

	replyHeader, responseBody, err := protocol.Decode(conn)

	if replyHeader.Seq != header.Seq {
		t.Fatalf("Expect replyHeader with seq: %v, get %v", header.Seq, replyHeader.Seq)
	}

	if replyHeader.CodecType != header.CodecType {
		t.Fatalf("Expect replyHeader with CodecType: %v, get %v", header.CodecType, replyHeader.CodecType)
	}

	if replyHeader.MsgType != protocol.MsgTypeResponse {
		t.Fatalf("Expect replyHeader with MsgType: %v, get %v", header.MsgType, replyHeader.MsgType)
	}

	responseRPC := message.RPCMessage{}

	err = cdc.Decode(responseBody, &responseRPC)

	if err != nil {
		t.Error(err)
	}

	var reply Reply

	err = json.Unmarshal(responseRPC.Payload, &reply)

	if err != nil {
		t.Error(err)
	}

	if reply.Result != 3 {
		t.Fatalf("Expect get result = 3, get %v", reply.Result)
		t.Fail()
	}

	fmt.Println("Pass all the test!")
}
