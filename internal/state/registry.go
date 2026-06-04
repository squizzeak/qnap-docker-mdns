package state

import (
	"sort"

	dockerpkg "github.com/squizzeak/qnap-docker-mdns/internal/docker"
)

type ContainerStatus int

const (
	StatusValid          ContainerStatus = iota
	StatusMisconfig                       // actionable misconfiguration
	StatusOperationalFailure              // operational failure
)

type Backend struct {
	ContainerName string
	Hostnames     []string
	Port          uint16
	Status        ContainerStatus
	StatusReason  string
}

type Registry struct {
	Backends []Backend
}

func BuildRegistry(containers []dockerpkg.Container, cfgSuffix string, probeFn func(uint16) bool, labelPort uint16, labelPortValid bool) *Registry {
	var backends []Backend

	for _, ct := range containers {
		labels := dockerpkg.ParseLabels(ct.Labels)

		if !labels.Enable {
			continue
		}

		b := Backend{ContainerName: ct.Name}

		loopbackPorts := dockerpkg.FilterLoopbackPorts(ct.Ports)
		if len(loopbackPorts) == 0 {
			b.Status = StatusMisconfig
			b.StatusReason = "no loopback-reachable host-published TCP ports"
			backends = append(backends, b)
			continue
		}

		var candidatePorts []uint16
		for _, p := range loopbackPorts {
			if probeFn(p.HostPort) {
				candidatePorts = append(candidatePorts, p.HostPort)
			}
		}

		port, ok, err := dockerpkg.SelectCandidatePort(loopbackPorts, candidatePorts, labelPort, labelPortValid)
		if !ok {
			b.Status = StatusMisconfig
			b.StatusReason = err.Error()
			backends = append(backends, b)
			continue
		}

		b.Port = port

		hostname := labels.Hostname
		if hostname == "" {
			hostname = dockerpkg.DefaultHostname(ct.Name, cfgSuffix)
		}

		b.Hostnames = append(b.Hostnames, hostname)
		b.Hostnames = append(b.Hostnames, labels.Aliases...)
		b.Status = StatusValid

		backends = append(backends, b)
	}

	SortBackends(backends)

	return &Registry{Backends: backends}
}

func SortBackends(backends []Backend) {
	sort.SliceStable(backends, func(i, j int) bool {
		return backends[i].ContainerName < backends[j].ContainerName
	})
}

func NormalizedKey(containerName, hostname string) string {
	return "container:" + containerName + "|hostname:" + hostname
}
