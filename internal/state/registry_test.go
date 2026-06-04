package state

import (
	"testing"

	dockerpkg "github.com/squizzeak/qnap-docker-mdns/internal/docker"
)

func fakeProbe(port uint16) bool {
	return port != 0 && port != 9999
}

func TestBuildRegistryEnabledContainer(t *testing.T) {
	ct := dockerpkg.Container{
		Name:   "grafana",
		Labels: map[string]string{"qnap-docker-mdns.enable": "true"},
		Ports:  []dockerpkg.PortBinding{{HostIP: "0.0.0.0", HostPort: 3000}},
	}
	reg := BuildRegistry([]dockerpkg.Container{ct}, "local", fakeProbe, 0, false)
	if len(reg.Backends) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(reg.Backends))
	}
	b := reg.Backends[0]
	if b.Status != StatusValid {
		t.Errorf("expected valid, got %v: %s", b.Status, b.StatusReason)
	}
	if b.Port != 3000 {
		t.Errorf("expected port 3000, got %d", b.Port)
	}
	if b.Hostnames[0] != "grafana.local" {
		t.Errorf("expected grafana.local, got %s", b.Hostnames[0])
	}
}

func TestBuildRegistryNotEnabled(t *testing.T) {
	ct := dockerpkg.Container{
		Name:   "skipped",
		Labels: map[string]string{},
		Ports:  []dockerpkg.PortBinding{{HostIP: "0.0.0.0", HostPort: 80}},
	}
	reg := BuildRegistry([]dockerpkg.Container{ct}, "local", fakeProbe, 0, false)
	if len(reg.Backends) != 0 {
		t.Errorf("expected 0 backends for disabled container, got %d", len(reg.Backends))
	}
}

func TestBuildRegistryNoLoopbackPorts(t *testing.T) {
	ct := dockerpkg.Container{
		Name:   "bad",
		Labels: map[string]string{"qnap-docker-mdns.enable": "true"},
		Ports:  []dockerpkg.PortBinding{{HostIP: "192.168.1.10", HostPort: 80}},
	}
	reg := BuildRegistry([]dockerpkg.Container{ct}, "local", fakeProbe, 0, false)
	if len(reg.Backends) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(reg.Backends))
	}
	if reg.Backends[0].Status != StatusMisconfig {
		t.Errorf("expected misconfig, got %v", reg.Backends[0].Status)
	}
}

func TestBuildRegistryMultipleCandidatesWithoutLabel(t *testing.T) {
	ct := dockerpkg.Container{
		Name:   "multi",
		Labels: map[string]string{"qnap-docker-mdns.enable": "true"},
		Ports: []dockerpkg.PortBinding{
			{HostIP: "0.0.0.0", HostPort: 80},
			{HostIP: "0.0.0.0", HostPort: 3000},
		},
	}
	reg := BuildRegistry([]dockerpkg.Container{ct}, "local", fakeProbe, 0, false)
	if len(reg.Backends) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(reg.Backends))
	}
	if reg.Backends[0].Status != StatusMisconfig {
		t.Errorf("expected misconfig, got %v", reg.Backends[0].Status)
	}
}

func TestBuildRegistryWithAliases(t *testing.T) {
	ct := dockerpkg.Container{
		Name:   "app",
		Labels: map[string]string{"qnap-docker-mdns.enable": "true", "qnap-docker-mdns.aliases": "app2.local, app3.local"},
		Ports:  []dockerpkg.PortBinding{{HostIP: "0.0.0.0", HostPort: 8080}},
	}
	reg := BuildRegistry([]dockerpkg.Container{ct}, "local", fakeProbe, 0, false)
	if len(reg.Backends) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(reg.Backends))
	}
	b := reg.Backends[0]
	if len(b.Hostnames) != 3 {
		t.Fatalf("expected 3 hostnames (1 primary + 2 aliases), got %d", len(b.Hostnames))
	}
	if b.Hostnames[0] != "app.local" {
		t.Errorf("expected app.local, got %s", b.Hostnames[0])
	}
}

func TestSortBackends(t *testing.T) {
	backends := []Backend{
		{ContainerName: "zebra"},
		{ContainerName: "alpha"},
	}
	SortBackends(backends)
	if backends[0].ContainerName != "alpha" {
		t.Errorf("expected alpha first, got %s", backends[0].ContainerName)
	}
}

func TestNormalizedKey(t *testing.T) {
	key := NormalizedKey("grafana", "grafana.local")
	if key != "container:grafana|hostname:grafana.local" {
		t.Errorf("unexpected key: %s", key)
	}
}
