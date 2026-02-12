package loadbalance

import (
	"fmt"
	"mini-rpc/registry"
	"testing"
)

var testInstances = []registry.ServiceInstance{
	{Addr: ":8001", Weight: 10, Version: "1.0"},
	{Addr: ":8002", Weight: 5, Version: "1.0"},
	{Addr: ":8003", Weight: 10, Version: "1.0"},
}

func TestRoundRobin(t *testing.T) {
	b := &RoundRobinbalancer{}

	// Pick 3 times, should cycle through all instances
	results := make([]string, 3)
	for i := 0; i < 3; i++ {
		inst, err := b.Pick(testInstances)
		if err != nil {
			t.Fatal(err)
		}
		results[i] = inst.Addr
	}

	// Pick again, should wrap around to first
	inst, _ := b.Pick(testInstances)
	if inst.Addr != results[0] {
		t.Fatalf("expect wrap around to %s, got %s", results[0], inst.Addr)
	}
}

func TestRoundRobinEmpty(t *testing.T) {
	b := &RoundRobinbalancer{}
	_, err := b.Pick([]registry.ServiceInstance{})
	if err == nil {
		t.Fatal("expect error for empty instances")
	}
}

func TestWeightedRandom(t *testing.T) {
	b := &WeightedRandomBalancer{}

	counts := map[string]int{}
	n := 10000
	for i := 0; i < n; i++ {
		inst, err := b.Pick(testInstances)
		if err != nil {
			t.Fatal(err)
		}
		counts[inst.Addr]++
	}

	// Weight ratio is 10:5:10, so :8001 and :8003 should be ~2x of :8002
	ratio := float64(counts[":8001"]) / float64(counts[":8002"])
	if ratio < 1.5 || ratio > 2.5 {
		t.Fatalf("weight ratio :8001/:8002 = %.2f, expect ~2.0", ratio)
	}
}

func TestConsistentHash(t *testing.T) {
	b := NewConsistentHashBalancer()
	for i := range testInstances {
		b.Add(&testInstances[i])
	}

	// Same key should always map to the same instance
	inst1, _ := b.Pick("user-123")
	inst2, _ := b.Pick("user-123")
	if inst1.Addr != inst2.Addr {
		t.Fatalf("same key mapped to different instances: %s vs %s", inst1.Addr, inst2.Addr)
	}

	// Different keys should (likely) map to different instances
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		inst, _ := b.Pick(fmt.Sprintf("key-%d", i))
		seen[inst.Addr] = true
	}

	// With 100 different keys and 3 nodes, we should hit at least 2
	if len(seen) < 2 {
		t.Fatalf("expect at least 2 different instances, got %d", len(seen))
	}
}
