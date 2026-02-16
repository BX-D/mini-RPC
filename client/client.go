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
	"sync/atomic"
)

type Client struct {
	registry   registry.Registry // find service instance from registry
	balancer   loadbalance.Balancer
	transports map[string][]*transport.ClientTransport // transport for each service instance
	codecType  codec.CodecType
	mu         sync.Mutex
	poolSize   int
	counter    uint64
}

func NewClient(reg registry.Registry, bal loadbalance.Balancer, codecType byte, poolSize int) *Client {
	return &Client{
		registry:   reg,
		balancer:   bal,
		transports: make(map[string][]*transport.ClientTransport),
		codecType:  codec.CodecType(codecType),
		poolSize:   poolSize,
	}
}

func (c *Client) getTransport(addr string) (*transport.ClientTransport, error) {
	// Check if transport pool exists for the address
	n := atomic.AddUint64(&c.counter, 1)
	c.mu.Lock()
	pool, ok := c.transports[addr]

	if !ok {
		// No pool exists, create one
		pool = make([]*transport.ClientTransport, c.poolSize)
		c.transports[addr] = pool
		// Create initial transports and fill the pool
		for i := 0; i < c.poolSize; i++ {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				return nil, err
			}
			pool[i] = transport.NewClientTransport(conn, c.codecType)
		}
	}
	c.mu.Unlock()
	return pool[n%uint64(c.poolSize)], nil
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
