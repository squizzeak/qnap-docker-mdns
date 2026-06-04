package docker

import (
	"fmt"
	"strings"
)

type Labels struct {
	Enable   bool
	Port     uint16
	PortSet  bool
	Hostname string
	Aliases  []string
}

func ParseLabels(labels map[string]string) Labels {
	var out Labels

	if v, ok := labels["qnap-docker-mdns.enable"]; ok && v == "true" {
		out.Enable = true
	}

	if v, ok := labels["qnap-docker-mdns.port"]; ok {
		var port uint16
		if _, err := fmt.Sscanf(v, "%d", &port); err == nil && port > 0 && port <= 65535 {
			out.Port = port
			out.PortSet = true
		}
	}

	if v, ok := labels["qnap-docker-mdns.hostname"]; ok && v != "" {
		out.Hostname = NormalizeHostname(v)
	}

	if v, ok := labels["qnap-docker-mdns.aliases"]; ok && v != "" {
		for _, a := range strings.Split(v, ",") {
			a = strings.TrimSpace(a)
			if a != "" {
				out.Aliases = append(out.Aliases, NormalizeHostname(a))
			}
		}
	}

	return out
}

func NormalizeHostname(hostname string) string {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if !strings.Contains(hostname, ".") {
		hostname += ".local"
	}
	return hostname
}

func DefaultHostname(containerName, suffix string) string {
	return strings.ToLower(containerName) + "." + suffix
}
