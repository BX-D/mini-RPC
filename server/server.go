package server

import (
	"encoding/json"
	"log"
	"mini-rpc/codec"
	"mini-rpc/message"
	"mini-rpc/protocol"
	"net"
	"reflect"
	"strings"
)

type Server struct {
	serviceMap map[string]*service
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

	if err != nil {
		return err
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		go svr.handleConn(conn)
	}

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
	// Get formatting codec
	codec := codec.GetCodec(codec.CodecType(header.CodecType))
	msg := message.RPCMessage{}
	// Decode body to msg
	codec.Decode(body, &msg)
	// Get service name and method name
	split := strings.Split(msg.ServiceMethod, ".")
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

	err := json.Unmarshal(msg.Payload, argv.Interface())

	if err != nil {
		errorMessage := message.RPCMessage{
			Error: err.Error(),
		}
		encodedErrorBody, err := codec.Encode(&errorMessage)
		if err != nil {
			return
		}
		replyHeader := protocol.Header{
			CodecType: header.CodecType,
			MsgType:   protocol.MsgTypeResponse,
			Seq:       header.Seq,
			BodyLen:   uint32(len(encodedErrorBody)),
		}

		err = protocol.Encode(conn, &replyHeader, encodedErrorBody)
		if err != nil {
			return
		}
		return
	}
	// Call the 'Call' method
	methodErr := svc.Call(method, argv, replyv)

	replyMessage, err := json.Marshal(replyv.Interface())
	if err != nil {
		log.Println("Failed to marshal method result")
	}
	// Constuct reply RPCMessage
	rpcMessage := message.RPCMessage{
		ServiceMethod: msg.ServiceMethod,
		Payload:       replyMessage,
	}

	if methodErr != nil {
		rpcMessage.Error = methodErr.Error()
	}
	// Encode body
	result, err := codec.Encode(&rpcMessage)

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
