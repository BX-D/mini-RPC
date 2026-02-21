package registry

import (
	"context"
	"encoding/json"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdRegistry struct {
	client *clientv3.Client
}

// NewEtcdRegistry 创建Etcd注册中心
func NewEtcdRegistry(endpoints []string) (*EtcdRegistry, error) {
	c, err := clientv3.New(clientv3.Config{
		Endpoints: endpoints,
	})

	if err != nil {
		return nil, err
	}

	return &EtcdRegistry{client: c}, nil
}

// Register 注册服务
func (r *EtcdRegistry) Register(serviceName string, instance ServiceInstance, ttl int64) error {
	ctx := context.TODO()

	lease, err := r.client.Grant(ctx, ttl)
	if err != nil {
		return err
	}

	val, err := json.Marshal(instance)

	if err != nil {
		return err
	}

	_, err = r.client.Put(ctx, "/mini-rpc/"+serviceName+"/"+instance.Addr, string(val), clientv3.WithLease(lease.ID))

	if err != nil {
		return err
	}

	ch, err := r.client.KeepAlive(ctx, lease.ID)

	if err != nil {
		return err
	}

	// Open a goroutine to consume keep-alive responses
	go func() {
		for range ch {
			// Just consume the channel to keep the lease alive
		}
	}()
	return nil
}

// Deregister 注销服务
func (r *EtcdRegistry) Deregister(serviceName string, addr string) error {
	// Revoke the lease to remove the service registration
	ctx := context.TODO()
	_, err := r.client.Delete(ctx, "/mini-rpc/"+serviceName+"/"+addr)

	if err != nil {
		return err
	}

	return nil
}

// Watch 监听服务变更
func (r *EtcdRegistry) Watch(serviceName string) <-chan []ServiceInstance {
	ctx := context.TODO()
	ch := make(chan []ServiceInstance, 1)
	prefix := "/mini-rpc/" + serviceName + "/"

	go func() {
		watchChan := r.client.Watch(ctx, prefix, clientv3.WithPrefix())
		for range watchChan {
			instances, _ := r.Discover(serviceName)
			ch <- instances
		}

	}()

	return ch
}

// Discover 发现服务实例
func (r *EtcdRegistry) Discover(serviceName string) ([]ServiceInstance, error) {
	ctx := context.TODO()
	prefix := "/mini-rpc/" + serviceName + "/"
	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())

	if err != nil {
		return nil, err
	}

	instances := make([]ServiceInstance, 0)

	for _, kv := range resp.Kvs {
		var instance ServiceInstance
		if err := json.Unmarshal(kv.Value, &instance); err != nil {
			continue
		}
		instances = append(instances, instance)
	}

	return instances, nil
}
