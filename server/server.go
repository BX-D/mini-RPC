// Package server implements the RPC server with service registration, middleware chain,
// parallel request processing, and graceful shutdown.
//
// Request processing pipeline:
//
//	Accept conn → handleConn (single goroutine reads frames)
//	  → for each request: go handleRequest (parallel processing)
//	    → Codec.Decode → Middleware Chain → businessHandler (reflect.Call) → Codec.Encode → write response
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mini-rpc/codec"
	"mini-rpc/message"
	"mini-rpc/middleware"
	"mini-rpc/protocol"
	"mini-rpc/registry"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Server is the RPC server that registers services and handles incoming requests.
type Server struct {
	serviceMap    map[string]*service     // Registered services: "Arith" → *service
	listener      net.Listener            // TCP listener
	wg            sync.WaitGroup          // Tracks in-flight requests for graceful shutdown
	shutdown      atomic.Bool             // Set to true during shutdown to suppress Accept errors
	middlewares   []middleware.Middleware // Registered middlewares (applied in order)
	handler       middleware.HandlerFunc  // The final handler chain: middleware(middleware(...(businessHandler)))
	registry      registry.Registry       // Service registry (etcd), nil if not using discovery
	advertiseAddr string                  // Address registered in etcd (e.g., "127.0.0.1:8080")
	// Different from listen address (":8080") because etcd needs a routable IP
}

// NewServer creates a new RPC server with an empty service map.
func NewServer() *Server {
	s := new(Server)
	s.serviceMap = make(map[string]*service)
	return s
}

// Register registers a service receiver (e.g., &Arith{}) with the server.
// The struct's exported methods that match the RPC signature will be available for remote calls.
func (svr *Server) Register(rcvr any) error {
	svc, err := NewService(rcvr)
	if err != nil {
		return err
	}
	svr.serviceMap[svc.name] = svc
	return nil
}

// Serve starts the server: listens on the given address, optionally registers with etcd,
// and enters the Accept loop to handle incoming connections.
//
// Parameters:
//   - advertiseAddr: the address to register in etcd (e.g., "127.0.0.1:8080").
//     This differs from the listen address because ":8080" resolves to "[::]:8080" locally.
//   - reg: the registry implementation. Pass nil to skip service discovery.
func (svr *Server) Serve(network, address string, advertiseAddr string, reg registry.Registry) error {
	listener, err := net.Listen(network, address)
	svr.listener = listener

	// Build the middleware chain once at startup (not per-request)
	// Chain wraps middlewares in reverse order to create the onion model:
	//   Chain(A, B, C)(handler) → A(B(C(handler)))
	//   Execution order: A.before → B.before → C.before → handler → C.after → B.after → A.after
	svr.handler = middleware.Chain(svr.middlewares...)(svr.businessHandler)

	if err != nil {
		return err
	}

	// Register all services to etcd (if registry is provided)
	svr.advertiseAddr = advertiseAddr
	if reg != nil {
		svr.registry = reg
		for serviceName := range svr.serviceMap {
			svr.registry.Register(serviceName, registry.ServiceInstance{
				Addr: advertiseAddr,
			}, 10) // TTL = 10 seconds, KeepAlive renews automatically
		}
	}

	// Accept loop: one goroutine per connection
	for {
		conn, err := listener.Accept()
		if err != nil {
			// During shutdown, listener.Close() causes Accept to return an error.
			// Check the shutdown flag to distinguish intentional close from real errors.
			if svr.shutdown.Load() {
				return nil
			} else {
				return err
			}
		}
		go svr.handleConn(conn)
	}
}

// Use registers a middleware. Middlewares are applied in the order they are added.
func (svr *Server) Use(mw middleware.Middleware) {
	svr.middlewares = append(svr.middlewares, mw)
}

// handleConn processes a single TCP connection.
// It runs a read loop in a single goroutine (reads must be sequential to parse frame boundaries),
// but dispatches each request to its own goroutine for parallel processing.
//
// A per-connection write mutex (writeMu) is shared among all request goroutines on this connection.
// This prevents frame interleaving when multiple goroutines write responses concurrently.
func (svr *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	writeMu := &sync.Mutex{} // Per-connection write lock, shared by all requests on this conn
	for {
		// Read one complete frame (sequential — single reader per connection)
		header, body, err := protocol.Decode(conn)
		if err != nil {
			break // Connection closed or protocol error
		}

		// Skip heartbeat frames — they exist only to keep the connection alive
		if header.MsgType == protocol.MsgTypeHeartbeat {
			continue
		}

		// Dispatch request to a new goroutine for parallel processing.
		// This is critical for performance: without `go`, a slow handler on request 1
		// would block all subsequent requests on the same connection.
		go svr.handleRequest(header, body, conn, writeMu)
	}
}

// handleRequest processes a single RPC request: decode → middleware → business logic → encode → write.
//
// The protocol layer (codec encode/decode, frame write) is separated from the business layer
// (service lookup, reflection call) to allow middleware to wrap only the business logic.
func (svr *Server) handleRequest(header *protocol.Header, body []byte, conn net.Conn, writeMu *sync.Mutex) {
	// Track this request for graceful shutdown (wg.Wait ensures all in-flight requests complete)
	svr.wg.Add(1)
	defer svr.wg.Done()

	// Step 1: Decode the frame body into an RPCMessage using the appropriate codec
	c := codec.GetCodec(codec.CodecType(header.CodecType))
	msg := message.RPCMessage{}
	c.Decode(body, &msg)

	// Step 2: Run through the middleware chain → business handler
	// The handler returns an RPCMessage with the response payload (or error)
	rpcMessage := svr.handler(context.Background(), &msg)

	// Step 3: Encode and write the response (protected by per-connection write lock)
	writeMu.Lock()
	defer writeMu.Unlock()

	result, err := c.Encode(rpcMessage)
	if err != nil {
		log.Println("Failed to encode method result")
		return
	}

	// Build response header — preserve the same Seq so the client can match it
	replyHeader := protocol.Header{
		CodecType: header.CodecType,
		MsgType:   protocol.MsgTypeResponse,
		Seq:       header.Seq, // Same seq as request — this is how multiplexing works
		BodyLen:   uint32(len(result)),
	}
	err = protocol.Encode(conn, &replyHeader, result)
	if err != nil {
		log.Println("Failed to encode reply message")
	}
}

// Shutdown performs graceful shutdown:
//  1. Deregister all services from etcd (clients stop routing to this server)
//  2. Set shutdown flag (so Accept error is recognized as intentional)
//  3. Close the listener (stop accepting new connections)
//  4. Wait for in-flight requests to finish (with timeout)
func (svr *Server) Shutdown(timeout time.Duration) error {
	// Step 1: Deregister from etcd FIRST — so clients stop sending new requests
	for serviceName := range svr.serviceMap {
		if svr.registry != nil {
			svr.registry.Deregister(serviceName, svr.advertiseAddr)
		}
	}

	// Step 2: Set shutdown flag BEFORE closing listener
	// If we close first, the Accept error fires before the flag is set,
	// and Serve() would return a real error instead of nil
	svr.shutdown.Store(true)
	svr.listener.Close()

	// Step 3: Wait for in-flight requests with timeout
	done := make(chan struct{})
	go func() {
		svr.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil // All requests completed
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for ongoing requests to finish")
	}
}

// businessHandler is the core handler that dispatches RPC requests to registered services.
// It is wrapped by the middleware chain and has the HandlerFunc signature.
//
// Flow: parse "Service.Method" → find service → find method → reflect.New(args) →
// json.Unmarshal(payload, args) → reflect.Call → json.Marshal(reply) → return RPCMessage
func (svr *Server) businessHandler(ctx context.Context, req *message.RPCMessage) *message.RPCMessage {
	// Parse "ServiceName.MethodName"
	split := strings.Split(req.ServiceMethod, ".")
	if len(split) != 2 {
		return &message.RPCMessage{Error: "invalid service method format"}
	}
	serviceName := split[0]
	methodName := split[1]

	// Look up the service and method in the registry
	svc := svr.serviceMap[serviceName]
	method := svc.method[methodName]

	// Create new instances of args and reply types via reflection
	argv := reflect.New(method.ArgType)     // e.g., reflect.New(Args) → *Args
	replyv := reflect.New(method.ReplyType) // e.g., reflect.New(Reply) → *Reply

	// Deserialize the request payload into the args struct
	err := json.Unmarshal(req.Payload, argv.Interface())
	if err != nil {
		return &message.RPCMessage{Error: err.Error()}
	}

	// Invoke the method via reflection: receiver.Method(args, reply)
	methodErr := svc.Call(method, argv, replyv)

	// Serialize the reply struct to JSON
	replyMessage, err := json.Marshal(replyv.Interface())
	if err != nil {
		log.Println("Failed to marshal method result")
	}

	// Build the response RPCMessage
	rpcMessage := &message.RPCMessage{
		ServiceMethod: req.ServiceMethod,
		Payload:       replyMessage,
	}
	if methodErr != nil {
		rpcMessage.Error = methodErr.Error()
	}
	return rpcMessage
}
