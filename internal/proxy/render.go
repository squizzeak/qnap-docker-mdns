package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type RuleEntry struct {
	ID                   int      `json:"id"`
	Name                 string   `json:"name"`
	Protocol             string   `json:"protocol"`
	DesProtocol          string   `json:"des_protocol"`
	ServerName           string   `json:"server_name"`
	HSTS                 bool     `json:"hsts"`
	HostName             string   `json:"host_name"`
	Access               int      `json:"access"`
	Port                 int      `json:"port"`
	DesPort              int      `json:"des_port"`
	ProxyTimeout         int      `json:"proxy_timeout"`
	Header               []string `json:"header"`
	QnapDockerMdnsManaged bool   `json:"qnap_docker_mdns_managed,omitempty"`
	QnapDockerMdnsKey    string   `json:"qnap_docker_mdns_key,omitempty"`
}

type ReverseProxyJSON struct {
	List map[string]RuleEntry `json:"list"`
}

func ReadJSON(path string) (*ReverseProxyJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var rp ReverseProxyJSON
	if err := json.Unmarshal(data, &rp); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if rp.List == nil {
		rp.List = make(map[string]RuleEntry)
	}
	return &rp, nil
}

func WriteJSON(path string, rp *ReverseProxyJSON) error {
	data, err := json.Marshal(rp)
	if err != nil {
		return fmt.Errorf("marshalling: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func MaxID(rp *ReverseProxyJSON) int {
	maxID := 0
	for _, entry := range rp.List {
		if entry.ID > maxID {
			maxID = entry.ID
		}
	}
	return maxID
}

func CountEntries(rp *ReverseProxyJSON) int {
	return len(rp.List)
}

type AccessProfile struct {
	ID   int
	Name string
}

func DiscoverLocalAccessProfile(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	var profiles struct {
		List map[string]struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"list"`
	}
	if err := json.Unmarshal(data, &profiles); err != nil {
		return 0, false
	}
	for _, p := range profiles.List {
		if p.Name == "local" {
			return p.ID, true
		}
	}
	return 0, false
}

type DesiredRule struct {
	ContainerName string
	Hostname      string
	Port          uint16
	ListenPort    int
	AccessID      int
	IsAlias       bool
}

func RenderManagedEntry(rule DesiredRule, existingID int) RuleEntry {
	nameSuffix := " (managed)"
	if rule.IsAlias {
		nameSuffix = " (managed alias)"
	}
	return RuleEntry{
		ID:                   existingID,
		Name:                 rule.Hostname + nameSuffix,
		Protocol:             "http",
		DesProtocol:          "http",
		ServerName:           rule.Hostname,
		HSTS:                 false,
		HostName:             "localhost",
		Access:               rule.AccessID,
		Port:                 rule.ListenPort,
		DesPort:              int(rule.Port),
		ProxyTimeout:         60,
		Header:               []string{},
		QnapDockerMdnsManaged: true,
		QnapDockerMdnsKey:    NormalizedKey(rule.ContainerName, rule.Hostname),
	}
}

func RenderAndMerge(current *ReverseProxyJSON, desired []DesiredRule, accessID int) *ReverseProxyJSON {
	result := &ReverseProxyJSON{
		List: make(map[string]RuleEntry),
	}

	ownedKeys := make(map[string]bool)
	for _, d := range desired {
		key := NormalizedKey(d.ContainerName, d.Hostname)
		ownedKeys[key] = true
	}

	nextID := MaxID(current) + 1

	sort.SliceStable(desired, func(i, j int) bool {
		if desired[i].ContainerName != desired[j].ContainerName {
			return desired[i].ContainerName < desired[j].ContainerName
		}
		return desired[i].Hostname < desired[j].Hostname
	})

	entryIndex := 0
	for _, entry := range current.List {
		if entry.QnapDockerMdnsManaged {
			key := entry.QnapDockerMdnsKey
			if !ownedKeys[key] {
				continue
			}
		}
		result.List[fmt.Sprintf("%d", entry.ID)] = entry
		entryIndex++
	}

	for _, d := range desired {
		key := NormalizedKey(d.ContainerName, d.Hostname)
		existingID := 0
		found := false
		for _, entry := range current.List {
			if entry.QnapDockerMdnsManaged && entry.QnapDockerMdnsKey == key {
				existingID = entry.ID
				found = true
				break
			}
		}
		if !found {
			existingID = nextID
			nextID++
		}

		d.AccessID = accessID
		entry := RenderManagedEntry(d, existingID)
		result.List[fmt.Sprintf("%d", entry.ID)] = entry
	}

	return result
}

func NormalizedKey(containerName, hostname string) string {
	return "container:" + containerName + "|hostname:" + hostname
}

func EntryExists(rp *ReverseProxyJSON, containerName, hostname string) bool {
	key := NormalizedKey(containerName, hostname)
	for _, entry := range rp.List {
		if entry.QnapDockerMdnsManaged && entry.QnapDockerMdnsKey == key {
			return true
		}
	}
	return false
}

func IsUnmanagedServerName(current *ReverseProxyJSON, hostname string, accessProfileJSONPath string) bool {
	for _, entry := range current.List {
		if entry.ServerName == hostname && !entry.QnapDockerMdnsManaged {
			return true
		}
	}
	return false
}
