package loadbalance

import (
	"fmt"
	"math/rand"
	"mini-rpc/registry"
)

// WeightedRandomBalancer selects instances probabilistically based on their weight.
// An instance with weight 10 gets roughly 2x the traffic of one with weight 5.
//
// Best for: heterogeneous instances (e.g., some servers have more CPU/memory).
//
// Algorithm:
//  1. Sum all weights → totalWeight
//  2. Generate random number r in [0, totalWeight)
//  3. Subtract each instance's weight from r until r < 0
//  4. The instance that makes r negative is selected
type WeightedRandomBalancer struct{}

func (b *WeightedRandomBalancer) Pick(instances []registry.ServiceInstance) (*registry.ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available")
	}

	// Calculate total weight
	totalWeight := 0
	for _, v := range instances {
		totalWeight += v.Weight
	}

	// Random selection proportional to weight
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
