package device

import (
	"testing"
	"time"
)

func TestCommandAckRegistryDeliversRegisteredAck(t *testing.T) {
	registry := newCommandAckRegistry()
	waiter, release, err := registry.Register("req-1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	defer release()

	go registry.TryDeliver("req-1", AckResult{OK: true, Message: "applied", RequestID: "req-1"})

	select {
	case res := <-waiter:
		if !res.OK || res.Message != "applied" || res.RequestID != "req-1" {
			t.Fatalf("unexpected ack result: %+v", res)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for ack")
	}
}

func TestCommandAckRegistryReleaseDropsLateAck(t *testing.T) {
	registry := newCommandAckRegistry()
	_, release, err := registry.Register("req-1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	release()

	if registry.TryDeliver("req-1", AckResult{OK: true, RequestID: "req-1"}) {
		t.Fatal("late ack was delivered after release")
	}
}
