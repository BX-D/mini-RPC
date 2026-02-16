package loadbalance

import (
	"fmt"
	"mini-rpc/registry"
	"sync/atomic"
)

// RoundRobinBalancer 轮询负载均衡器
type RoundRobinBalancer struct {
	counter int64
}

// Pick 从服务实例列表中选择一个实例
func (b *RoundRobinBalancer) Pick(instances []registry.ServiceInstance) (*registry.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available")
	}

	// 轮询选择实例
	index := atomic.AddInt64(&b.counter, 1) % int64(len(instances))
	return &instances[index], nil
}

// Name 返回负载均衡器的名称
func (b *RoundRobinBalancer) Name() string {
	return "RoundRobin"
}
