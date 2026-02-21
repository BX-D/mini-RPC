// Package transport implements the client-side transport layer with multiplexing and heartbeat.
//
// ClientTransport enables multiple concurrent RPC calls over a single TCP connection.
// The key insight: each request gets a unique sequence ID, and a background goroutine (recvLoop)
// continuously reads responses and routes them to the correct caller via pending channels.
//
//	goroutine-1 ──Send(seq=1)──┐
//	goroutine-2 ──Send(seq=2)──┼──→ single TCP conn ──→ Server
//	goroutine-3 ──Send(seq=3)──┘
//
//	recvLoop:  ←── response(seq=2) → pending[2] chan ← response → goroutine-2 wakes up
package transport

import (
	"encoding/json"
	"mini-rpc/codec"
	"mini-rpc/message"
	"mini-rpc/protocol"
	"net"
	"sync"
	"time"
)

// ClientTransport manages a single multiplexed TCP connection.
type ClientTransport struct {
	conn    net.Conn        // Underlying TCP connection
	codec   codec.CodecType // Serialization format for this transport
	seq     uint32          // Monotonically increasing sequence number (protected by sending mutex)
	pending sync.Map        // map[uint32]chan *message.RPCMessage — each request waits on its own channel
	sending sync.Mutex      // Write lock — multiple goroutines share one conn, writes must be serialized
	//                        to prevent frame interleaving (req A's header + req B's body = corruption)
}

// NewClientTransport creates a transport for the given connection and starts two background goroutines:
//   - recvLoop: continuously reads responses from the connection and dispatches to pending callers
//   - heartbeatLoop: sends periodic heartbeat frames to detect dead connections
func NewClientTransport(conn net.Conn, codec codec.CodecType) *ClientTransport {
	transport := &ClientTransport{
		conn:  conn,
		codec: codec,
	}
	go transport.recvLoop()
	go transport.heartbeatLoop(30 * time.Second)
	return transport
}

// Send serializes and sends an RPC request over the connection.
// Returns the sequence number and a channel that will receive the response.
//
// Thread safety: the sending mutex ensures that the entire frame (header + body)
// is written atomically. Without this lock, concurrent writes would interleave
// bytes from different requests, corrupting the TCP stream.
func (t *ClientTransport) Send(serviceMethod string, args any) (uint32, <-chan *message.RPCMessage, error) {
	t.sending.Lock()
	defer t.sending.Unlock()

	// Assign a unique sequence number for this request (protected by sending mutex)
	t.seq++
	seq := t.seq

	// Step 1: Serialize args to JSON bytes
	payload, err := json.Marshal(args)
	if err != nil {
		return 0, nil, err
	}

	// Step 2: Wrap in RPCMessage and encode with the configured codec
	rpcMessage := message.RPCMessage{
		ServiceMethod: serviceMethod,
		Error:         "",
		Payload:       payload,
	}
	cdc := codec.GetCodec(t.codec)
	body, err := cdc.Encode(&rpcMessage)
	if err != nil {
		return 0, nil, err
	}

	// Step 3: Build the protocol frame header
	header := protocol.Header{
		CodecType: byte(t.codec),
		MsgType:   protocol.MsgTypeRequest,
		Seq:       seq,
		BodyLen:   uint32(len(body)),
	}

	// Step 4: Register a response channel BEFORE sending (avoid race with recvLoop)
	respChan := make(chan *message.RPCMessage, 1) // Buffered to prevent recvLoop from blocking
	t.pending.Store(seq, respChan)

	// Step 5: Write the frame to the TCP connection
	err = protocol.Encode(t.conn, &header, body)
	if err != nil {
		t.pending.Delete(seq) // Clean up on failure
		return 0, nil, err
	}

	return seq, respChan, nil
}

// recvLoop runs in a dedicated goroutine, continuously reading responses from the connection.
// For each response, it looks up the sequence number in the pending map, finds the caller's
// channel, and sends the response. This is the core of multiplexing — responses can arrive
// in any order, and each one is routed to the correct waiting goroutine.
//
// Why a single goroutine for reading? TCP is a byte stream — reads must be sequential
// to correctly parse frame boundaries. Multiple readers would corrupt the stream.
func (t *ClientTransport) recvLoop() {
	for {
		// Read one complete frame from the connection
		header, body, err := protocol.Decode(t.conn)
		if err != nil {
			// Connection broken — notify all pending callers
			t.closeAllPending(err)
			return
		}

		// Deserialize the response body
		responseRPC := message.RPCMessage{}
		cdc := codec.GetCodec(codec.CodecType(header.CodecType))
		cdc.Decode(body, &responseRPC)

		// Route the response to the correct caller using the sequence number
		if channel, ok := t.pending.LoadAndDelete(header.Seq); ok {
			channel.(chan *message.RPCMessage) <- &responseRPC
		}
	}
}

// closeAllPending is called when the connection breaks. It sends an error message
// to every pending caller so they don't block forever waiting for a response.
func (t *ClientTransport) closeAllPending(err error) {
	t.pending.Range(func(key, value any) bool {
		channel := value.(chan *message.RPCMessage)
		channel <- &message.RPCMessage{Error: err.Error()}
		return true
	})
	t.pending.Clear()
}

// Conn returns the underlying TCP connection.
func (t *ClientTransport) Conn() net.Conn {
	return t.conn
}

// heartbeatLoop sends periodic heartbeat frames to keep the connection alive.
// If the server doesn't receive any data for a long time, it may close the connection.
// Heartbeat frames have MsgType=Heartbeat and no body, so they're very lightweight.
func (t *ClientTransport) heartbeatLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		header := &protocol.Header{
			MsgType: protocol.MsgTypeHeartbeat,
			BodyLen: 0,
		}
		// Heartbeat writes also need the sending lock to avoid frame interleaving
		t.sending.Lock()
		err := protocol.Encode(t.conn, header, nil)
		t.sending.Unlock()
		if err != nil {
			return // Connection broken, exit heartbeat loop
		}
	}
}
