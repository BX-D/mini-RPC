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

type Server struct {
	serviceMap  map[string]*service
	listener    net.Listener
	wg          sync.WaitGroup
	shutdown    atomic.Bool
	middlewares []middleware.Middleware
	handler     middleware.HandlerFunc
}

func NewServer() *Server {
	s := new(Server)
	s.serviceMap = make(map[string]*service)
	return s
}

func (svr *Server) Register(rcvr any) error {
	svc, err := NewService(rcvr)
	if err != nil {
		return err
	}
	// Store svc to map
	svr.serviceMap[svc.name] = svc
	return nil
}

func (svr *Server) Serve(network, address string) error {
	// Create a listener and bind to a port
	listener, err := net.Listen(network, address)
	svr.listener = listener

	// Construct business handler with middlewares
	svr.handler = middleware.Chain(svr.middlewares...)(svr.businessHandler)

	if err != nil {
		return err
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			if svr.shutdown.Load() {
				return nil
			} else {
				return err
			}
		}
		go svr.handleConn(conn)
	}
}

func (svr *Server) Use(mw middleware.Middleware) {
	svr.middlewares = append(svr.middlewares, mw)
}

func (svr *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	for {
		header, body, err := protocol.Decode(conn)
		if err != nil {
			break
		}
		svr.handleRequest(header, body, conn)
	}
}

func (svr *Server) handleRequest(header *protocol.Header, body []byte, conn net.Conn) {
	// Wg add 1 for each request
	svr.wg.Add(1)
	defer svr.wg.Done()

	// Get formatting codec
	c := codec.GetCodec(codec.CodecType(header.CodecType))
	msg := message.RPCMessage{}
	// Decode body to msg
	c.Decode(body, &msg)

	// Call handler to get reply message
	rpcMessage := svr.handler(context.Background(), &msg)
	// Encode body
	result, err := c.Encode(rpcMessage)

	if err != nil {
		log.Println("Failed to encode method result")
		return
	}
	// Construct Header
	replyHeader := protocol.Header{
		CodecType: header.CodecType,
		MsgType:   protocol.MsgTypeResponse,
		Seq:       header.Seq,
		BodyLen:   uint32(len(result)),
	}
	err = protocol.Encode(conn, &replyHeader, result)

	if err != nil {
		log.Println("Failed to encode reply message")
	}
}

// Shutdown the server
func (svr *Server) Shutdown(registry registry.Registry, timeout time.Duration) error {
	// Derigistry from etcd
	for serviceName := range svr.serviceMap {
		registry.Deregister(serviceName, svr.listener.Addr().String())
	}
	// Close the listener
	svr.shutdown.Store(true)
	svr.listener.Close()

	done := make(chan struct{})
	go func() {
		svr.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for ongoing requests to finish")
	}
}

func (svr *Server) businessHandler(ctx context.Context, req *message.RPCMessage) *message.RPCMessage {
	// Get service name and method name
	split := strings.Split(req.ServiceMethod, ".")

	if len(split) != 2 {
		errorMessage := &message.RPCMessage{
			Error: "invalid service method format",
		}
		return errorMessage
	}
	serviceName := split[0]
	methodName := split[1]

	// Find service
	svc := svr.serviceMap[serviceName]

	// Call require 3 parameters: *methodType, argv and replayv(both are reflect.Value)
	// Construct parameters
	method := svc.method[methodName]
	argv := reflect.New(method.ArgType)
	// Deserialize payload to argv
	replyv := reflect.New(method.ReplyType)

	err := json.Unmarshal(req.Payload, argv.Interface())

	if err != nil {
		errorMessage := &message.RPCMessage{
			Error: err.Error(),
		}
		return errorMessage
	}
	// Call the 'Call' method
	methodErr := svc.Call(method, argv, replyv)

	replyMessage, err := json.Marshal(replyv.Interface())
	if err != nil {
		log.Println("Failed to marshal method result")
	}
	// Constuct reply RPCMessage
	rpcMessage := &message.RPCMessage{
		ServiceMethod: req.ServiceMethod,
		Payload:       replyMessage,
	}
	if methodErr != nil {
		rpcMessage.Error = methodErr.Error()
	}
	return rpcMessage
}
