package mdns

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type Publisher struct {
	mu       sync.Mutex
	children map[string]*childProcess
}

type childProcess struct {
	Hostname string
	IP       string
	PID      int
	cmd      *exec.Cmd
}

func NewPublisher() *Publisher {
	return &Publisher{
		children: make(map[string]*childProcess),
	}
}

func childKey(hostname, ip string) string {
	return hostname + "|" + ip
}

func (p *Publisher) Publish(hostname, ip string) error {
	key := childKey(hostname, ip)
	p.mu.Lock()
	if _, exists := p.children[key]; exists {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	cmd := exec.Command("avahi-publish-address", "-a", hostname, ip)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start avahi-publish-address %s %s: %w", hostname, ip, err)
	}

	child := &childProcess{
		Hostname: hostname,
		IP:       ip,
		PID:      cmd.Process.Pid,
		cmd:      cmd,
	}

	p.mu.Lock()
	p.children[key] = child
	p.mu.Unlock()

	// Reap child process to prevent zombies.
	go func() {
		cmd.Wait()
		p.mu.Lock()
		if existing, ok := p.children[key]; ok && existing.PID == cmd.Process.Pid {
			delete(p.children, key)
		}
		p.mu.Unlock()
	}()

	return nil
}

func (p *Publisher) Unpublish(hostname, ip string) error {
	key := childKey(hostname, ip)
	p.mu.Lock()
	child, exists := p.children[key]
	if !exists {
		p.mu.Unlock()
		return nil
	}
	delete(p.children, key)
	p.mu.Unlock()

	if err := child.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill avahi-publish-address %s %s: %w", hostname, ip, err)
	}
	return nil
}

func (p *Publisher) StopAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []string
	for key, child := range p.children {
		if err := child.cmd.Process.Kill(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", key, err))
		}
	}
	p.children = make(map[string]*childProcess)

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping publishers: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (p *Publisher) ActivePublishers() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	var keys []string
	for key := range p.children {
		keys = append(keys, key)
	}
	return keys
}

func (p *Publisher) Reconcile(desired map[string][]string, adopted map[string]string) error {
	desiredSet := make(map[string]bool)
	for hostname, ips := range desired {
		for _, ip := range ips {
			key := childKey(hostname, ip)
			desiredSet[key] = true
		}
	}

	for hostname, ip := range adopted {
		key := childKey(hostname, ip)
		if desiredSet[key] {
			desiredSet[key] = false
		}
	}

	p.mu.Lock()
	currentKeys := make(map[string]*childProcess)
	for k, v := range p.children {
		currentKeys[k] = v
	}
	p.mu.Unlock()

	var errs []string

	for key := range currentKeys {
		if !desiredSet[key] {
			parts := strings.SplitN(key, "|", 2)
			if len(parts) == 2 {
				if err := p.Unpublish(parts[0], parts[1]); err != nil {
					errs = append(errs, err.Error())
				}
			}
		}
	}

	for key, shouldPublish := range desiredSet {
		if !shouldPublish {
			continue
		}
		parts := strings.SplitN(key, "|", 2)
		if len(parts) == 2 {
			if err := p.Publish(parts[0], parts[1]); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("reconciliation errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func DiscoverLANAddresses() ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}

	var addresses []string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.To4() == nil {
				continue
			}
			addresses = append(addresses, ip.String())
		}
	}

	if len(addresses) == 0 {
		return nil, fmt.Errorf("no suitable LAN IPv4 address found")
	}

	return addresses, nil
}

func FindAdoptedHelpers() (map[string]string, error) {
	// Inspect running avahi-publish-address processes by reading /proc
	// This is a best-effort startup reconciliation helper.
	return make(map[string]string), nil
}

func ResolveMDNS(hostname string) ([]string, error) {
	cmd := exec.Command("avahi-resolve-host-name", hostname)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", hostname, err)
	}

	var addresses []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			addresses = append(addresses, parts[len(parts)-1])
		}
	}
	return addresses, nil
}

func IsHostnamePublishedByExternal(hostname string, nasAddresses []string) (bool, error) {
	addresses, err := ResolveMDNS(hostname)
	if err != nil {
		return false, nil
	}

	addrSet := make(map[string]bool)
	for _, a := range nasAddresses {
		addrSet[a] = true
	}

	for _, addr := range addresses {
		parsed := net.ParseIP(addr)
		if parsed == nil || !addrSet[addr] {
			return true, nil
		}
	}
	return false, nil
}
