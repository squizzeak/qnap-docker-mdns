package mdns

import (
	"testing"
)

func TestChildKey(t *testing.T) {
	key := childKey("grafana.local", "192.168.1.10")
	if key != "grafana.local|192.168.1.10" {
		t.Errorf("unexpected key: %s", key)
	}
}

func TestPublishUnpublish(t *testing.T) {
	p := NewPublisher()
	if err := p.Publish("test.local", "127.0.0.1"); err == nil {
		t.Log("publish attempted (may fail without avahi)")
	}
}

func TestPublishDeduplicate(t *testing.T) {
	p := NewPublisher()
	key := childKey("test.local", "127.0.0.1")
	p.children[key] = &childProcess{Hostname: "test.local", IP: "127.0.0.1"}
	if err := p.Publish("test.local", "127.0.0.1"); err != nil {
		t.Error("expected no error for duplicate publish")
	}
}

func TestStopAll(t *testing.T) {
	p := NewPublisher()
	if err := p.StopAll(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestActivePublishers(t *testing.T) {
	p := NewPublisher()
	p.children["a|1"] = &childProcess{Hostname: "a", IP: "1"}
	p.children["b|2"] = &childProcess{Hostname: "b", IP: "2"}

	active := p.ActivePublishers()
	if len(active) != 2 {
		t.Errorf("expected 2 active publishers, got %d", len(active))
	}
}

func TestDiscoverLANAddresses(t *testing.T) {
	addresses, err := DiscoverLANAddresses()
	if err != nil {
		t.Logf("LAN discovery: %v (expected in non-NAS environments)", err)
	} else {
		t.Logf("discovered %d addresses", len(addresses))
	}
}

func TestIsHostnamePublishedByExternalNoResolution(t *testing.T) {
	result, err := IsHostnamePublishedByExternal("nonexistent-test-123.local", []string{"192.168.1.10"})
	if err != nil {
		t.Fatal(err)
	}
	if result {
		t.Error("expected false for unresolved hostname")
	}
}

func TestReconcileEmptyDesired(t *testing.T) {
	p := NewPublisher()
	if err := p.Reconcile(map[string][]string{}, nil); err != nil {
		t.Errorf("expected no error for empty reconcile, got %v", err)
	}
}
