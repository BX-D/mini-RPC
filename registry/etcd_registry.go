// Package registry provides the etcd-based implementation of the Registry interface.
//
// etcd is a distributed key-value store that provides strong consistency (Raft protocol).
// We use it as a "distributed phonebook" for services:
//
//	Key:   /mini-rpc/{ServiceName}/{Addr}
//	Value: JSON-encoded ServiceInstance
//
// Registration uses TTL-based leases: if the server crashes, the lease expires
// and the entry is automatically removed — preventing "ghost" instances.
package registry

import (
	"context"
	"encoding/json"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdRegistry implements the Registry interface using etcd v3.
type EtcdRegistry struct {
	client *clientv3.Client // etcd client connection (thread-safe, shared across goroutines)
}

// NewEtcdRegistry creates a new registry connected to the given etcd endpoints.
func NewEtcdRegistry(endpoints []string) (*EtcdRegistry, error) {
	c, err := clientv3.New(clientv3.Config{
		Endpoints: endpoints,
	})
	if err != nil {
		return nil, err
	}
	return &EtcdRegistry{client: c}, nil
}

// Register adds a service instance to etcd with a TTL lease.
//
// Flow:
//  1. Create a lease with the given TTL (e.g., 10 seconds)
//  2. Put the key-value pair with the lease attached
//  3. Start KeepAlive to automatically renew the lease
//
// Note: leaseID is a local variable, NOT stored on the struct.
// This prevents a data race when multiple servers share one EtcdRegistry instance
// (discovered via `go test -race`).
func (r *EtcdRegistry) Register(serviceName string, instance ServiceInstance, ttl int64) error {
	ctx := context.TODO()

	// Create a TTL-based lease — if KeepAlive stops, the entry auto-expires
	lease, err := r.client.Grant(ctx, ttl)
	if err != nil {
		return err
	}

	// Serialize the instance metadata
	val, err := json.Marshal(instance)
	if err != nil {
		return err
	}

	// Store in etcd: key = /mini-rpc/{service}/{addr}, value = JSON metadata
	_, err = r.client.Put(ctx, "/mini-rpc/"+serviceName+"/"+instance.Addr, string(val), clientv3.WithLease(lease.ID))
	if err != nil {
		return err
	}

	// Start background lease renewal — KeepAlive sends heartbeats to etcd
	ch, err := r.client.KeepAlive(ctx, lease.ID)
	if err != nil {
		return err
	}

	// Consume KeepAlive responses to prevent the channel from filling up
	go func() {
		for range ch {
		}
	}()
	return nil
}

// Deregister removes a service instance from etcd.
// Called during graceful shutdown before closing the listener.
func (r *EtcdRegistry) Deregister(serviceName string, addr string) error {
	ctx := context.TODO()
	_, err := r.client.Delete(ctx, "/mini-rpc/"+serviceName+"/"+addr)
	if err != nil {
		return err
	}
	return nil
}

// Watch monitors a service prefix in etcd and emits updated instance lists
// whenever changes occur (new registrations, deregistrations, lease expirations).
//
// Uses etcd's Watch API (server-push), which is more efficient than polling.
func (r *EtcdRegistry) Watch(serviceName string) <-chan []ServiceInstance {
	ctx := context.TODO()
	ch := make(chan []ServiceInstance, 1)
	prefix := "/mini-rpc/" + serviceName + "/"

	go func() {
		// Watch all keys under the service prefix
		watchChan := r.client.Watch(ctx, prefix, clientv3.WithPrefix())
		for range watchChan {
			// On any change, re-fetch the full instance list
			// (simpler than parsing individual watch events)
			instances, _ := r.Discover(serviceName)
			ch <- instances
		}
	}()

	return ch
}

// Discover returns all currently registered instances for a service.
// Queries etcd with a key prefix to find all instances under /mini-rpc/{serviceName}/.
func (r *EtcdRegistry) Discover(serviceName string) ([]ServiceInstance, error) {
	ctx := context.TODO()
	prefix := "/mini-rpc/" + serviceName + "/"

	// Get all keys with the prefix
	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	// Deserialize each value into a ServiceInstance
	instances := make([]ServiceInstance, 0)
	for _, kv := range resp.Kvs {
		var instance ServiceInstance
		if err := json.Unmarshal(kv.Value, &instance); err != nil {
			continue // Skip malformed entries
		}
		instances = append(instances, instance)
	}

	return instances, nil
}
