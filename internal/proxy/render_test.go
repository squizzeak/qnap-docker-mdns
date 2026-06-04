package proxy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMaxIDEmpty(t *testing.T) {
	rp := &ReverseProxyJSON{List: map[string]RuleEntry{}}
	if id := MaxID(rp); id != 0 {
		t.Errorf("expected 0, got %d", id)
	}
}

func TestMaxIDWithEntries(t *testing.T) {
	rp := &ReverseProxyJSON{
		List: map[string]RuleEntry{
			"1": {ID: 1},
			"5": {ID: 5},
		},
	}
	if id := MaxID(rp); id != 5 {
		t.Errorf("expected 5, got %d", id)
	}
}

func TestCountEntries(t *testing.T) {
	rp := &ReverseProxyJSON{
		List: map[string]RuleEntry{
			"1": {ID: 1},
			"2": {ID: 2},
		},
	}
	if n := CountEntries(rp); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

func TestRenderManagedEntry(t *testing.T) {
	rule := DesiredRule{
		ContainerName: "grafana",
		Hostname:      "grafana.local",
		Port:          3000,
		ListenPort:    80,
		AccessID:      1,
	}
	entry := RenderManagedEntry(rule, 5)
	if entry.ID != 5 {
		t.Errorf("expected ID 5, got %d", entry.ID)
	}
	if entry.Name != "grafana.local (managed)" {
		t.Errorf("expected 'grafana.local (managed)', got %s", entry.Name)
	}
	if entry.ServerName != "grafana.local" {
		t.Errorf("expected grafana.local, got %s", entry.ServerName)
	}
	if entry.HostName != "localhost" {
		t.Errorf("expected localhost, got %s", entry.HostName)
	}
	if entry.DesPort != 3000 {
		t.Errorf("expected 3000, got %d", entry.DesPort)
	}
	if entry.Access != 1 {
		t.Errorf("expected access 1, got %d", entry.Access)
	}
	if !entry.QnapDockerMdnsManaged {
		t.Error("expected managed=true")
	}
}

func TestRenderManagedAliasEntry(t *testing.T) {
	rule := DesiredRule{
		ContainerName: "grafana",
		Hostname:      "grafana.local",
		Port:          3000,
		ListenPort:    80,
		AccessID:      1,
		IsAlias:       true,
	}
	entry := RenderManagedEntry(rule, 5)
	if entry.Name != "grafana.local (managed alias)" {
		t.Errorf("expected 'grafana.local (managed alias)', got %s", entry.Name)
	}
}

func TestRenderAndMergePreservesUnmanaged(t *testing.T) {
	current := &ReverseProxyJSON{
		List: map[string]RuleEntry{
			"1": {ID: 1, Name: "manual", ServerName: "manual.local", QnapDockerMdnsManaged: false},
		},
	}
	desired := []DesiredRule{
		{ContainerName: "app", Hostname: "app.local", Port: 8080, ListenPort: 80, AccessID: 1},
	}
	result := RenderAndMerge(current, desired, 1)
	if len(result.List) != 2 {
		t.Errorf("expected 2 entries (1 unmanaged + 1 managed), got %d", len(result.List))
	}
	if _, ok := result.List["1"]; !ok {
		t.Error("expected unmanaged entry preserved with ID 1")
	}
}

func TestRenderAndMergeReusesExistingID(t *testing.T) {
	current := &ReverseProxyJSON{
		List: map[string]RuleEntry{
			"1": {ID: 1, Name: "app.local (managed)", ServerName: "app.local", QnapDockerMdnsManaged: true, QnapDockerMdnsKey: "container:app|hostname:app.local"},
		},
	}
	desired := []DesiredRule{
		{ContainerName: "app", Hostname: "app.local", Port: 8080, ListenPort: 80, AccessID: 1},
	}
	result := RenderAndMerge(current, desired, 1)
	entry, ok := result.List["1"]
	if !ok {
		t.Fatal("expected entry with ID 1")
	}
	if entry.ID != 1 {
		t.Errorf("expected reused ID 1, got %d", entry.ID)
	}
}

func TestRenderAndMergeRemovesStaleManaged(t *testing.T) {
	current := &ReverseProxyJSON{
		List: map[string]RuleEntry{
			"1": {ID: 1, Name: "gone.local (managed)", ServerName: "gone.local", QnapDockerMdnsManaged: true, QnapDockerMdnsKey: "container:gone|hostname:gone.local"},
		},
	}
	result := RenderAndMerge(current, []DesiredRule{}, 1)
	if len(result.List) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result.List))
	}
}

func TestDiscoverLocalAccessProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.json")
	content := `{"list":{"1":{"id":1,"name":"local"}}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	id, ok := DiscoverLocalAccessProfile(path)
	if !ok {
		t.Fatal("expected to discover local profile")
	}
	if id != 1 {
		t.Errorf("expected ID 1, got %d", id)
	}
}

func TestDiscoverLocalAccessProfileMissing(t *testing.T) {
	id, ok := DiscoverLocalAccessProfile("/nonexistent.json")
	if ok {
		t.Errorf("expected not found, got id=%d", id)
	}
}

func TestIsUnmanagedServerName(t *testing.T) {
	rp := &ReverseProxyJSON{
		List: map[string]RuleEntry{
			"1": {ID: 1, ServerName: "existing.local", QnapDockerMdnsManaged: false},
		},
	}
	if !IsUnmanagedServerName(rp, "existing.local", "") {
		t.Error("expected true for unmanaged hostname")
	}
	if IsUnmanagedServerName(rp, "other.local", "") {
		t.Error("expected false for unknown hostname")
	}
}

func TestNormalizedKey(t *testing.T) {
	key := NormalizedKey("grafana", "grafana.local")
	if key != "container:grafana|hostname:grafana.local" {
		t.Errorf("unexpected key: %s", key)
	}
}

func TestEntryExists(t *testing.T) {
	rp := &ReverseProxyJSON{
		List: map[string]RuleEntry{
			"1": {ID: 1, QnapDockerMdnsManaged: true, QnapDockerMdnsKey: "container:app|hostname:app.local"},
			"2": {ID: 2, QnapDockerMdnsManaged: false, ServerName: "manual.local"},
		},
	}
	if !EntryExists(rp, "app", "app.local") {
		t.Error("expected EntryExists=true for managed entry")
	}
	if EntryExists(rp, "app", "other.local") {
		t.Error("expected EntryExists=false for unknown hostname")
	}
	if EntryExists(rp, "other", "app.local") {
		t.Error("expected EntryExists=false for unknown container")
	}
	if EntryExists(rp, "app", "manual.local") {
		t.Error("expected EntryExists=false for unmanaged entry")
	}
}
