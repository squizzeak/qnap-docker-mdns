package docker

import (
	"testing"
)

func TestParseLabelsBasic(t *testing.T) {
	labels := map[string]string{
		"qnap-docker-mdns.enable": "true",
	}
	l := ParseLabels(labels)
	if !l.Enable {
		t.Error("expected enable=true")
	}
	if l.PortSet {
		t.Error("expected port not set")
	}
}

func TestParseLabelsNotEnabled(t *testing.T) {
	labels := map[string]string{}
	l := ParseLabels(labels)
	if l.Enable {
		t.Error("expected enable=false")
	}
}

func TestParseLabelsWithPort(t *testing.T) {
	labels := map[string]string{
		"qnap-docker-mdns.enable": "true",
		"qnap-docker-mdns.port":   "3000",
	}
	l := ParseLabels(labels)
	if !l.Enable {
		t.Error("expected enable=true")
	}
	if !l.PortSet {
		t.Error("expected port set")
	}
	if l.Port != 3000 {
		t.Errorf("expected port 3000, got %d", l.Port)
	}
}

func TestParseLabelsInvalidPort(t *testing.T) {
	labels := map[string]string{
		"qnap-docker-mdns.port": "abc",
	}
	l := ParseLabels(labels)
	if l.PortSet {
		t.Error("expected port not set for invalid value")
	}
}

func TestParseLabelsWithHostname(t *testing.T) {
	labels := map[string]string{
		"qnap-docker-mdns.hostname": "grafana.local",
	}
	l := ParseLabels(labels)
	if l.Hostname != "grafana.local" {
		t.Errorf("expected grafana.local, got %s", l.Hostname)
	}
}

func TestParseLabelsWithAliases(t *testing.T) {
	labels := map[string]string{
		"qnap-docker-mdns.aliases": "metrics.local, graphs.local",
	}
	l := ParseLabels(labels)
	if len(l.Aliases) != 2 {
		t.Errorf("expected 2 aliases, got %d", len(l.Aliases))
	}
	if l.Aliases[0] != "metrics.local" {
		t.Errorf("expected metrics.local, got %s", l.Aliases[0])
	}
	if l.Aliases[1] != "graphs.local" {
		t.Errorf("expected graphs.local, got %s", l.Aliases[1])
	}
}

func TestParseLabelsEmptyAliases(t *testing.T) {
	labels := map[string]string{
		"qnap-docker-mdns.aliases": "",
	}
	l := ParseLabels(labels)
	if len(l.Aliases) != 0 {
		t.Errorf("expected 0 aliases, got %d", len(l.Aliases))
	}
}

func TestNormalizeHostnameAlreadyFQDN(t *testing.T) {
	result := NormalizeHostname("Grafana.Local")
	if result != "grafana.local" {
		t.Errorf("expected grafana.local, got %s", result)
	}
}

func TestNormalizeHostnameSuffixAdded(t *testing.T) {
	result := NormalizeHostname("grafana")
	if result != "grafana.local" {
		t.Errorf("expected grafana.local, got %s", result)
	}
}

func TestDefaultHostname(t *testing.T) {
	result := DefaultHostname("MyContainer", "local")
	if result != "mycontainer.local" {
		t.Errorf("expected mycontainer.local, got %s", result)
	}
}

func TestFilterLoopbackPorts(t *testing.T) {
	ports := []PortBinding{
		{HostIP: "0.0.0.0", HostPort: 80},
		{HostIP: "127.0.0.1", HostPort: 3000},
		{HostIP: "192.168.1.10", HostPort: 8080},
	}
	filtered := FilterLoopbackPorts(ports)
	if len(filtered) != 2 {
		t.Errorf("expected 2 loopback ports, got %d", len(filtered))
	}
}

func TestSelectCandidatePortSingle(t *testing.T) {
	port, ok, err := SelectCandidatePort(nil, []uint16{3000}, 0, false)
	if !ok || err != nil {
		t.Fatal("expected success")
	}
	if port != 3000 {
		t.Errorf("expected 3000, got %d", port)
	}
}

func TestSelectCandidatePortNone(t *testing.T) {
	_, _, err := SelectCandidatePort(nil, []uint16{}, 0, false)
	if err == nil {
		t.Fatal("expected error for no candidates")
	}
}

func TestSelectCandidatePortMultipleWithLabel(t *testing.T) {
	port, ok, err := SelectCandidatePort(nil, []uint16{80, 3000}, 3000, true)
	if !ok || err != nil {
		t.Fatal("expected success")
	}
	if port != 3000 {
		t.Errorf("expected 3000, got %d", port)
	}
}

func TestSelectCandidatePortMultipleWithoutLabel(t *testing.T) {
	_, _, err := SelectCandidatePort(nil, []uint16{80, 3000}, 0, false)
	if err == nil {
		t.Fatal("expected error for multiple candidates without label")
	}
}

func TestSelectCandidatePortLabelNotInCandidates(t *testing.T) {
	_, _, err := SelectCandidatePort(nil, []uint16{80}, 3000, true)
	if err == nil {
		t.Fatal("expected error for label port not in candidates")
	}
}
