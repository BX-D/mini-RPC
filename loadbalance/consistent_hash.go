package loadbalance

import (
	"fmt"
	"hash/crc32"
	"mini-rpc/registry"
	"sort"
)

type ConsistentHashBalancer struct {
	replicas int
	ring     []uint32
	nodes    map[uint32]*registry.ServiceInstance
}

func NewConsistentHashBalancer() *ConsistentHashBalancer {
	return &ConsistentHashBalancer{
		replicas: 100, // 每个节点的虚拟节点数量
		ring:     []uint32{},
		nodes:    make(map[uint32]*registry.ServiceInstance),
	}
}

// Add 将服务实例添加到哈希环中
func (b *ConsistentHashBalancer) Add(instance *registry.ServiceInstance) {
	// 为每个实例创建多个虚拟节点
	for i := 0; i < b.replicas; i++ {
		key := fmt.Sprintf("%s#%d", instance.Addr, i)
		hash := crc32.ChecksumIEEE([]byte(key))
		b.ring = append(b.ring, hash)
		b.nodes[hash] = instance
	}

	sort.Slice(b.ring, func(i, j int) bool {
		return b.ring[i] < b.ring[j]
	})
}

func (b *ConsistentHashBalancer) Pick(key string) (*registry.ServiceInstance, error) {
	// 计算key的哈希值
	hash := crc32.ChecksumIEEE([]byte(key))

	// 在哈希环上找到第一个大于等于hash的节点(Binary Search)
	idx := sort.Search(len(b.ring), func(i int) bool {
		return b.ring[i] >= hash
	})

	// 如果idx等于ring长度，说明hash值大于所有节点，应该回到第一个节点
	if idx == len(b.ring) {
		idx = 0
	}

	return b.nodes[b.ring[idx]], nil
}

func (b *ConsistentHashBalancer) Name() string {
	return "ConsistentHash"
}
