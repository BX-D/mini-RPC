package loadbalance

import (
	"fmt"
	"mini-rpc/registry"
	"sync/atomic"
)

// RoundRobinBalancer distributes requests evenly across all instances in order.
// Uses an atomic counter for lock-free, goroutine-safe operation.
//
// Best for: stateless services where all instances have similar capacity.
type RoundRobinBalancer struct {
	counter int64 // Atomic counter, incremented on each Pick()
}

// Pick selects the next instance in round-robin order.
// The atomic counter ensures even distribution without locks.
func (b *RoundRobinBalancer) Pick(instances []registry.ServiceInstance) (*registry.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available")
	}
	index := atomic.AddInt64(&b.counter, 1) % int64(len(instances))
	return &instances[index], nil
}

func (b *RoundRobinBalancer) Name() string {
	return "RoundRobin"
}
