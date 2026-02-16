package client

import (
	"encoding/json"
	"fmt"
	"mini-rpc/codec"
	"mini-rpc/loadbalance"
	"mini-rpc/registry"
	"mini-rpc/transport"
	"net"
	"strings"
	"sync"
)

type Client struct {
	registry   registry.Registry // find service instance from registry
	balancer   loadbalance.Balancer
	transports map[string]*transport.ClientTransport // transport for each service instance
	codecType  codec.CodecType
	mu         sync.Mutex
}

func NewClient(reg registry.Registry, bal loadbalance.Balancer, codecType byte) *Client {
	return &Client{
		registry:   reg,
		balancer:   bal,
		transports: make(map[string]*transport.ClientTransport),
		codecType:  codec.CodecType(codecType),
	}
}

func (c *Client) getTransport(addr string) (*transport.ClientTransport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t, ok := c.transports[addr]; ok {
		return t, nil
	}

	// Create a new connection pool for the address
	conn, err := net.Dial("tcp", addr)

	if err != nil {
		return nil, err
	}

	t := transport.NewClientTransport(conn, c.codecType)
	c.transports[addr] = t
	return t, nil
}

func (c *Client) Call(serviceMethod string, args any, reply any) error {
	// Get transport from the pool
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

	// Get the transport for the selected instance
	t, err := c.getTransport(instance.Addr)

	if err != nil {
		return err
	}

	// Send the request and wait for the response
	_, ch, err := t.Send(serviceMethod, args)

	if err != nil {
		return err
	}

	resp := <-ch

	if resp.Error != "" {
		return fmt.Errorf("server error: %v", resp.Error)
	}

	// Unmarshal the payload to reply
	err = json.Unmarshal(resp.Payload, &reply)

	if err != nil {
		return err
	}

	return nil
}
