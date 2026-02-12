package loadbalance

import "mini-rpc/registry"

type Balancer interface {
	Pick(instances []registry.ServiceInstance) (*registry.ServiceInstance, error)
	Name() string
}
