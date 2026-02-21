package loadbalance

import (
	"fmt"
	"hash/crc32"
	"mini-rpc/registry"
	"sort"
)

// ConsistentHashBalancer maps keys to instances using a hash ring.
// The same key always maps to the same instance (until the ring changes),
// providing cache affinity — useful for stateful services or local caches.
//
// Virtual nodes: each real instance is mapped to N virtual nodes on the ring.
// Without virtual nodes, 3 instances might cluster together on the ring,
// causing uneven load distribution. 100 virtual nodes per instance ensures
// statistical uniformity.
//
//	Hash Ring:
//	                  0
//	                ╱   ╲
//	              ╱       ╲
//	         B ●               ● A
//	           │    key ◆──►   │   (clockwise to nearest node → A)
//	         C ●               ● A' (virtual node of A)
//	              ╲       ╱
//	                ╲   ╱
type ConsistentHashBalancer struct {
	replicas int                                  // Virtual nodes per real instance
	ring     []uint32                             // Sorted hash values on the ring
	nodes    map[uint32]*registry.ServiceInstance // Hash value → instance mapping
}

// NewConsistentHashBalancer creates a hash ring with 100 virtual nodes per instance.
func NewConsistentHashBalancer() *ConsistentHashBalancer {
	return &ConsistentHashBalancer{
		replicas: 100,
		ring:     []uint32{},
		nodes:    make(map[uint32]*registry.ServiceInstance),
	}
}

// Add places an instance onto the hash ring with N virtual nodes.
// Each virtual node is hashed from "{addr}#{i}" to spread evenly across the ring.
func (b *ConsistentHashBalancer) Add(instance *registry.ServiceInstance) {
	for i := 0; i < b.replicas; i++ {
		key := fmt.Sprintf("%s#%d", instance.Addr, i)
		hash := crc32.ChecksumIEEE([]byte(key))
		b.ring = append(b.ring, hash)
		b.nodes[hash] = instance
	}
	// Keep the ring sorted for binary search in Pick()
	sort.Slice(b.ring, func(i, j int) bool {
		return b.ring[i] < b.ring[j]
	})
}

// Pick finds the instance responsible for the given key.
// It hashes the key, then binary-searches for the first node >= hash on the ring.
// If the hash is larger than all nodes, it wraps around to the first node (ring property).
//
// Note: Pick takes a string key (not []ServiceInstance) because consistent hashing
// is key-based — it doesn't implement the Balancer interface directly.
func (b *ConsistentHashBalancer) Pick(key string) (*registry.ServiceInstance, error) {
	hash := crc32.ChecksumIEEE([]byte(key))

	// Binary search: find first node with hash >= key's hash
	idx := sort.Search(len(b.ring), func(i int) bool {
		return b.ring[i] >= hash
	})

	// Wrap around: if key's hash > all nodes, go to the first node
	if idx == len(b.ring) {
		idx = 0
	}

	return b.nodes[b.ring[idx]], nil
}

func (b *ConsistentHashBalancer) Name() string {
	return "ConsistentHash"
}
