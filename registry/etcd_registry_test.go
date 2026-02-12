package registry

import (
	"testing"
	"time"
)

func TestRegisterAndDiscover(t *testing.T) {
	reg, err := NewEtcdRegistry([]string{"localhost:2379"})
	if err != nil {
		t.Fatal(err)
	}

	// Register two instances
	inst1 := ServiceInstance{Addr: "127.0.0.1:8001", Weight: 10, Version: "1.0"}
	inst2 := ServiceInstance{Addr: "127.0.0.1:8002", Weight: 5, Version: "1.0"}

	if err := reg.Register("Arith", inst1, 10); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("Arith", inst2, 10); err != nil {
		t.Fatal(err)
	}

	// Discover
	instances, err := reg.Discover("Arith")
	if err != nil {
		t.Fatal(err)
	}

	if len(instances) != 2 {
		t.Fatalf("expect 2 instances, got %d", len(instances))
	}

	// Deregister one
	if err := reg.Deregister("Arith", inst1.Addr); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	instances, err = reg.Discover("Arith")
	if err != nil {
		t.Fatal(err)
	}

	if len(instances) != 1 {
		t.Fatalf("expect 1 instance after deregister, got %d", len(instances))
	}

	if instances[0].Addr != inst2.Addr {
		t.Fatalf("expect %s, got %s", inst2.Addr, instances[0].Addr)
	}

	// Cleanup
	reg.Deregister("Arith", inst2.Addr)
}
