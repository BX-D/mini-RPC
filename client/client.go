package client

import (
	"encoding/json"
	"fmt"
	"mini-rpc/codec"
	"mini-rpc/loadbalance"
	"mini-rpc/message"
	"mini-rpc/protocol"
	"mini-rpc/registry"
	"mini-rpc/transport"
	"net"
	"strings"
	"sync"
	"sync/atomic"
)

type Client struct {
	registry  registry.Registry // find service instance from registry
	balancer  loadbalance.Balancer
	pools     map[string]*transport.ConnPool // Each addr has a connection pool
	codecType codec.CodecType
	mu        sync.Mutex
	seq       uint32
}

func NewClient(reg registry.Registry, bal loadbalance.Balancer, codecType byte) *Client {
	return &Client{
		registry:  reg,
		balancer:  bal,
		pools:     make(map[string]*transport.ConnPool),
		codecType: codec.CodecType(codecType),
	}
}

func (c *Client) getPool(addr string) *transport.ConnPool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if pool, ok := c.pools[addr]; ok {
		return pool
	}

	// Create a new connection pool for the address
	pool := transport.NewConnPool(addr, 5, func() (net.Conn, error) {
		return net.Dial("tcp", addr)
	})
	c.pools[addr] = pool
	return pool
}

func (c *Client) Call(serviceMethod string, args any, reply any) error {
	//c.seq.Add(1)
	atomic.AddUint32(&c.seq, 1)
	// Marshal args to json
	payload, err := json.Marshal(args)
	if err != nil {
		return err
	}
	// Create a RPCMessage
	rpcMessage := message.RPCMessage{
		ServiceMethod: serviceMethod,
		Error:         "",
		Payload:       payload,
	}

	// Get the codec
	cdc := codec.GetCodec(c.codecType)

	// Encode the message
	body, err := cdc.Encode(&rpcMessage)

	if err != nil {
		return err
	}

	// Construct the header
	header := protocol.Header{
		CodecType: byte(c.codecType),
		MsgType:   protocol.MsgTypeRequest,
		Seq:       c.seq,
		BodyLen:   uint32(len(body)),
	}

	// Get a connection from the pool
	split := strings.Split(serviceMethod, ".")

	if len(split) != 2 {
		return fmt.Errorf("invalid serviceMethod format: %v", serviceMethod)
	}

	serviceName := split[0]

	// Get service instances from registry
	instances, err := c.registry.Discover(serviceName)

	if err != nil {
		return err
	}

	// Select an instance using load balancer
	instance, err := c.balancer.Pick(instances)

	if err != nil {
		return err
	}

	// Get the connection pool for the selected instance
	pool := c.getPool(instance.Addr)

	// Get a connection from the pool
	conn, err := pool.Get()

	if err != nil {
		return err
	}

	defer pool.Put(conn)

	// Send the request
	err = protocol.Encode(conn, &header, body)

	if err != nil {
		return err
	}

	// Wait for the response
	replyHeader, responseBody, err := protocol.Decode(conn)
	if err != nil {
		return err
	}

	if replyHeader.Seq != header.Seq {
		return fmt.Errorf("expect replyHeader with seq: %v, get %v", header.Seq, replyHeader.Seq)
	}

	if replyHeader.CodecType != header.CodecType {
		return fmt.Errorf("expect replyHeader with CodecType: %v, get %v", header.CodecType, replyHeader.CodecType)
	}

	if replyHeader.MsgType != protocol.MsgTypeResponse {
		return fmt.Errorf("expect replyHeader with MsgType: %v, get %v", header.MsgType, replyHeader.MsgType)
	}

	// Decode the response body to RPCMessage
	responseRPC := message.RPCMessage{}

	err = cdc.Decode(responseBody, &responseRPC)

	if err != nil {
		return err
	}

	if responseRPC.Error != "" {
		return fmt.Errorf("server error: %v", responseRPC.Error)
	}

	// Unmarshal the payload to reply
	err = json.Unmarshal(responseRPC.Payload, &reply)

	if err != nil {
		return err
	}

	return nil
}
