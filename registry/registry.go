// Package registry defines the service discovery interface and data types.
//
// Service discovery solves the problem of "how does the client find the server?"
// Instead of hardcoding IP:port, servers register themselves in a central registry (etcd),
// and clients query the registry to find available instances.
package registry

// ServiceInstance represents a single running instance of a service.
type ServiceInstance struct {
	Addr    string // Network address, e.g., "127.0.0.1:8080"
	Weight  int    // Weight for load balancing (higher = more traffic)
	Version string // Service version for canary deployments
}

// Registry is the interface for service registration and discovery.
// Implementations include EtcdRegistry (production) and MockRegistry (testing).
type Registry interface {
	// Register adds a service instance to the registry with a TTL lease.
	// The instance will be automatically removed if KeepAlive stops (e.g., server crashes).
	Register(serviceName string, instance ServiceInstance, ttl int64) error

	// Deregister removes a service instance from the registry.
	// Called during graceful shutdown BEFORE closing the listener.
	Deregister(serviceName string, addr string) error

	// Discover returns all currently registered instances for a service.
	// The client calls this to get the instance list for load balancing.
	Discover(serviceName string) ([]ServiceInstance, error)

	// Watch returns a channel that emits updated instance lists whenever
	// the service's instances change (new instances, removals, etc.).
	// This enables real-time service discovery without polling.
	Watch(serviceName string) <-chan []ServiceInstance
}
