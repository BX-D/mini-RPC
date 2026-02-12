package loadbalance

import (
	"fmt"
	"math/rand"
	"mini-rpc/registry"
)

type WeightedRandomBalancer struct{}

func (b *WeightedRandomBalancer) Pick(instances []registry.ServiceInstance) (*registry.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available")
	}

	// 计算总权重
	totalWeight := 0
	for _, v := range instances {
		totalWeight += v.Weight
	}

	// 生成一个随机数，范围是0到总权重
	r := rand.Intn(totalWeight)
	for _, v := range instances {
		r -= v.Weight
		if r < 0 {
			return &v, nil
		}
	}

	return nil, fmt.Errorf("unexpected error in weighted random selection")
}

func (b *WeightedRandomBalancer) Name() string {
	return "WeightedRandom"
}
