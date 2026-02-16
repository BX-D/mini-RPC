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
	transports map[string]chan *transport.ClientTransport // transport for each service instance
	codecType  codec.CodecType
	mu         sync.Mutex
	poolSize   int
}

func NewClient(reg registry.Registry, bal loadbalance.Balancer, codecType byte, poolSize int) *Client {
	return &Client{
		registry:   reg,
		balancer:   bal,
		transports: make(map[string]chan *transport.ClientTransport),
		codecType:  codec.CodecType(codecType),
		poolSize:   poolSize,
	}
}

func (c *Client) getTransport(addr string) (*transport.ClientTransport, error) {
	// Check if transport pool exists for the address
	c.mu.Lock()
	pool, ok := c.transports[addr]

	if !ok {
		// No pool exists, create one
		pool = make(chan *transport.ClientTransport, c.poolSize)
		c.transports[addr] = pool
	}

	c.mu.Unlock()

	if !ok {
		// Create initial transports and fill the pool
		for i := 0; i < c.poolSize; i++ {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				return nil, err
			}
			pool <- transport.NewClientTransport(conn, c.codecType)
		}
	}

	return <-pool, nil
}

func (c *Client) putTransport(addr string, t *transport.ClientTransport) {
	c.transports[addr] <- t
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

	defer c.putTransport(instance.Addr, t)

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
