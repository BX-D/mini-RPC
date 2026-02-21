// Package client implements the RPC client with service discovery, load balancing,
// and a shared transport pool for multiplexed connections.
//
// Call flow:
//
//	Call("Arith.Add", args, reply)
//	  → Registry.Discover("Arith")   → get instance list from etcd
//	  → Balancer.Pick(instances)      → select one address
//	  → getTransport(addr)            → get a shared transport (round-robin)
//	  → transport.Send()              → send request, get response channel
//	  → <-channel                     → wait for response
//	  → json.Unmarshal → reply        → done
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

// Client manages the full RPC call lifecycle: service discovery → load balancing → transport → call.
type Client struct {
	registry   registry.Registry                       // Service discovery (etcd or mock)
	balancer   loadbalance.Balancer                    // Load balancing strategy
	transports map[string][]*transport.ClientTransport // Per-address transport pool (shared, not borrowed)
	codecType  codec.CodecType                         // Serialization format
	mu         sync.Mutex                              // Protects transports map (not the transports themselves)
	poolSize   int                                     // Number of transports per address
	counter    uint64                                  // Atomic counter for round-robin transport selection
}

// NewClient creates a client with the given registry, load balancer, codec type, and pool size.
//
// poolSize determines how many TCP connections are maintained per server address.
// Each connection supports multiplexing, so even poolSize=1 handles concurrent calls.
// Larger pools reduce write lock contention under very high concurrency.
func NewClient(reg registry.Registry, bal loadbalance.Balancer, codecType byte, poolSize int) *Client {
	return &Client{
		registry:   reg,
		balancer:   bal,
		transports: make(map[string][]*transport.ClientTransport),
		codecType:  codec.CodecType(codecType),
		poolSize:   poolSize,
	}
}

// getTransport returns a shared transport for the given address using round-robin selection.
//
// Design: transports are SHARED, not borrowed/returned. Since each ClientTransport supports
// multiplexing, there's no need to exclusively hold a transport during a call. The transport
// is only "used" during Send() (a few microseconds), not during the entire call (which includes
// waiting for the response). Shared access avoids 95% idle time from exclusive holding.
//
// Lock strategy:
//   - mu.Lock protects the transports map (read + write). This is nanosecond-level.
//   - net.Dial is inside the lock only on first access (pool creation). Subsequent calls
//     just read the map and select via atomic counter — no lock needed for selection.
func (c *Client) getTransport(addr string) (*transport.ClientTransport, error) {
	// Atomic counter for round-robin — each goroutine captures its own value (no race)
	n := atomic.AddUint64(&c.counter, 1)

	// Lock only to protect map access (not transport usage)
	c.mu.Lock()
	pool, ok := c.transports[addr]

	if !ok {
		// First access to this address — create all transports upfront
		pool = make([]*transport.ClientTransport, c.poolSize)
		c.transports[addr] = pool
		for i := 0; i < c.poolSize; i++ {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				c.mu.Unlock()
				return nil, err
			}
			pool[i] = transport.NewClientTransport(conn, c.codecType)
		}
	}
	c.mu.Unlock()

	// Round-robin selection — lock-free, uses the captured counter value
	return pool[n%uint64(c.poolSize)], nil
}

// Call performs a synchronous RPC call.
//
// Steps:
//  1. Parse serviceMethod ("Arith.Add" → service="Arith")
//  2. Discover instances from registry
//  3. Pick an instance using load balancer
//  4. Get a shared transport for that instance
//  5. Send the request and wait for the response
//  6. Unmarshal the response payload into reply
func (c *Client) Call(serviceMethod string, args any, reply any) error {
	// Step 1: Parse service name from "Service.Method" format
	split := strings.Split(serviceMethod, ".")
	if len(split) != 2 {
		return fmt.Errorf("invalid serviceMethod format: %v", serviceMethod)
	}
	serviceName := split[0]

	// Step 2: Discover available instances from the registry
	instances, err := c.registry.Discover(serviceName)
	if err != nil {
		return err
	}

	// Step 3: Select one instance using the load balancer
	instance, err := c.balancer.Pick(instances)
	if err != nil {
		return err
	}

	// Step 4: Get a shared transport for the selected instance's address
	t, err := c.getTransport(instance.Addr)
	if err != nil {
		return err
	}

	// Step 5: Send the request — returns immediately with a response channel
	_, ch, err := t.Send(serviceMethod, args)
	if err != nil {
		return err
	}

	// Block until the response arrives (routed by recvLoop via sequence number)
	resp := <-ch

	// Check for server-side errors
	if resp.Error != "" {
		return fmt.Errorf("server error: %v", resp.Error)
	}

	// Step 6: Unmarshal the JSON payload into the reply struct
	return json.Unmarshal(resp.Payload, &reply)
}
