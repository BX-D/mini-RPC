package registry

type ServiceInstance struct {
	Addr    string
	Weight  int // Weight for load balancing
	Version string
}

type Registry interface {
	Register(serviceName string, instance ServiceInstance, ttl int64) error
	Deregister(serviceName string, addr string) error
	Discover(serviceName string) ([]ServiceInstance, error)
	Watch(serviceName string) <-chan []ServiceInstance
}
