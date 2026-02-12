package transport

import (
	"encoding/json"
	"mini-rpc/codec"
	"mini-rpc/message"
	"mini-rpc/protocol"
	"net"
	"sync"
)

type ClientTransport struct {
	conn    net.Conn
	codec   codec.CodecType
	seq     uint32
	pending sync.Map
	sending sync.Mutex
}

func NewClientTransport(conn net.Conn, codec codec.CodecType) *ClientTransport {
	transport := &ClientTransport{
		conn:  conn,
		codec: codec,
	}
	go transport.recvLoop()

	return transport
}

func (t *ClientTransport) Send(serviceMethod string, args any) (uint32, <-chan *message.RPCMessage, error) {
	// Increment sequence number
	t.sending.Lock()
	defer t.sending.Unlock()
	t.seq++
	seq := t.seq

	// Marshal args to json
	cdc := codec.GetCodec(t.codec)

	// Construct the RPCMessage
	payload, err := json.Marshal(args)

	if err != nil {
		return 0, nil, err
	}

	rpcMessage := message.RPCMessage{
		ServiceMethod: serviceMethod,
		Error:         "",
		Payload:       payload,
	}

	body, err := cdc.Encode(&rpcMessage)

	if err != nil {
		return 0, nil, err
	}

	// Construct the header
	header := protocol.Header{
		CodecType: byte(t.codec),
		MsgType:   protocol.MsgTypeRequest,
		Seq:       seq,
		BodyLen:   uint32(len(body)),
	}

	// Create a channel to receive the response
	respChan := make(chan *message.RPCMessage, 1)
	t.pending.Store(seq, respChan)

	// Send the request
	err = protocol.Encode(t.conn, &header, body)

	if err != nil {
		// Clean up the pending map if sending fails
		t.pending.Delete(seq)
		return 0, nil, err
	}

	return seq, respChan, nil
}

func (t *ClientTransport) recvLoop() {
	for {
		header, body, err := protocol.Decode(t.conn)
		if err != nil {
			t.closeAllPending(err)
			return
		}

		// Decode the response body to RPCMessage
		responseRPC := message.RPCMessage{}
		cdc := codec.GetCodec(codec.CodecType(header.CodecType))
		cdc.Decode(body, &responseRPC)

		// Get the channel from pending map and send the responseRPC to the channel
		if channel, ok := t.pending.LoadAndDelete(header.Seq); ok {
			channel.(chan *message.RPCMessage) <- &responseRPC
		}
	}
}

func (t *ClientTransport) closeAllPending(err error) {
	// Send error message to all pending channels
	t.pending.Range(func(key, value any) bool {
		channel := value.(chan *message.RPCMessage)
		rpcMessage := message.RPCMessage{
			Error: err.Error(),
		}
		channel <- &rpcMessage
		return true
	})
	// Delete all pending channels
	t.pending.Clear()
}
