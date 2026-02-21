// Package loadbalance provides load balancing strategies for distributing
// RPC requests across multiple service instances.
//
// Three strategies are implemented:
//   - RoundRobin:      Stateless services, equal-capacity instances
//   - WeightedRandom:  Heterogeneous instances (different CPU/memory)
//   - ConsistentHash:  Stateful services requiring cache affinity
package loadbalance

import "mini-rpc/registry"

// Balancer is the interface for load balancing strategies.
// The client calls Pick() before each RPC to select a target instance.
type Balancer interface {
	// Pick selects one instance from the available list.
	// Called on every RPC call — must be goroutine-safe.
	Pick(instances []registry.ServiceInstance) (*registry.ServiceInstance, error)

	// Name returns the strategy name (for logging/debugging).
	Name() string
}
