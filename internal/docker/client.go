package docker

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

type Client struct {
	cli          *client.Client
	probeTimeout time.Duration
}

type Container struct {
	ID          string
	Name        string
	Labels      map[string]string
	Ports       []PortBinding
}

type PortBinding struct {
	HostIP   string
	HostPort uint16
}

func NewClient(socketPath string, probeTimeout time.Duration) (*Client, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost("unix://" + socketPath),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &Client{cli: cli, probeTimeout: probeTimeout}, nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}

func (c *Client) ListRunningContainers(ctx context.Context) ([]Container, error) {
	containers, err := c.cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return nil, fmt.Errorf("container list: %w", err)
	}

	var result []Container
	for _, ct := range containers {
		name := strings.TrimPrefix(ct.Names[0], "/")
		var ports []PortBinding
		for _, p := range ct.Ports {
			if p.Type == "tcp" && p.PublicPort > 0 {
				hostIP := p.IP
				if hostIP == "" {
					hostIP = "0.0.0.0"
				}
				ports = append(ports, PortBinding{
					HostIP:   hostIP,
					HostPort: uint16(p.PublicPort),
				})
			}
		}
		result = append(result, Container{
			ID:     ct.ID,
			Name:   name,
			Labels: ct.Labels,
			Ports:  ports,
		})
	}
	return result, nil
}

func FilterLoopbackPorts(ports []PortBinding) []PortBinding {
	var filtered []PortBinding
	for _, p := range ports {
		ip := net.ParseIP(p.HostIP)
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func (c *Client) ProbePort(ctx context.Context, hostPort uint16) bool {
	addr := net.JoinHostPort("localhost", strconv.Itoa(int(hostPort)))
	probeCtx, cancel := context.WithTimeout(ctx, c.probeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, "GET", "http://"+addr+"/", nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

func SelectCandidatePort(ports []PortBinding, candidates []uint16, labelPort uint16, labelValid bool) (uint16, bool, error) {
	switch {
	case len(candidates) == 0:
		return 0, false, fmt.Errorf("no HTTP-capable candidate ports")
	case len(candidates) == 1:
		return candidates[0], true, nil
	default:
		if labelValid {
			for _, c := range candidates {
				if c == labelPort {
					return c, true, nil
				}
			}
			return 0, false, fmt.Errorf("label port %d not HTTP-capable", labelPort)
		}
		return 0, false, fmt.Errorf("multiple HTTP-capable candidates, set qnap-docker-mdns.port")
	}
}

func (c *Client) Events(ctx context.Context) (<-chan events.Message, <-chan error) {
	return c.cli.Events(ctx, types.EventsOptions{})
}

func isTrackedEvent(event events.Message) bool {
	switch event.Action {
	case "start", "stop", "die", "destroy", "rename":
		return true
	}
	return false
}

func (c *Client) InspectContainer(ctx context.Context, containerID string) (*Container, error) {
	info, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", containerID, err)
	}

	name := strings.TrimPrefix(info.Name, "/")
	var ports []PortBinding
	for _, p := range info.NetworkSettings.Ports {
		if p == nil {
			continue
		}
		for _, binding := range p {
			if binding.HostIP == "" {
				binding.HostIP = "0.0.0.0"
			}
			hostPort, err := strconv.ParseUint(binding.HostPort, 10, 16)
			if err != nil {
				continue
			}
			ports = append(ports, PortBinding{
				HostIP:   binding.HostIP,
				HostPort: uint16(hostPort),
			})
		}
	}

	return &Container{
		ID:     containerID,
		Name:   name,
		Labels: info.Config.Labels,
		Ports:  ports,
	}, nil
}
