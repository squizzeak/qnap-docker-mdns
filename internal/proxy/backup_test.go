package proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestWouldExceedLimit(t *testing.T) {
	if !WouldExceedLimit(MaxRules, 1) {
		t.Error("expected true for 64 current + 1 new (exceeds limit)")
	}
	if !WouldExceedLimit(MaxRules-1, 2) {
		t.Error("expected true for 63 current + 2 new")
	}
	if WouldExceedLimit(MaxRules, 0) {
		t.Error("expected false for exactly 64 current (at limit)")
	}
	if WouldExceedLimit(MaxRules-1, 1) {
		t.Error("expected false for 63 current + 1 new (room available)")
	}
}

func TestFindCollisions(t *testing.T) {
	current := &ReverseProxyJSON{
		List: map[string]RuleEntry{
			"1": {ID: 1, ServerName: "other.local", QnapDockerMdnsManaged: false},
		},
	}
	collisions := FindCollisions(current, []string{"app.local", "other.local"})
	if len(collisions) != 1 || collisions[0] != "other.local" {
		t.Errorf("expected [other.local], got %v", collisions)
	}
}

func TestFindCollisionsManagedIgnored(t *testing.T) {
	current := &ReverseProxyJSON{
		List: map[string]RuleEntry{
			"1": {ID: 1, ServerName: "app.local", QnapDockerMdnsManaged: true},
		},
	}
	collisions := FindCollisions(current, []string{"app.local"})
	if len(collisions) != 0 {
		t.Errorf("expected no collisions for managed entries, got %v", collisions)
	}
}

func TestFilterCollisions(t *testing.T) {
	result := FilterCollisions([]string{"a.local", "b.local", "c.local"}, []string{"b.local"})
	if len(result) != 2 || result[0] != "a.local" || result[1] != "c.local" {
		t.Errorf("expected [a.local c.local], got %v", result)
	}
}

func TestAtomicWriteJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	rp := &ReverseProxyJSON{
		List: map[string]RuleEntry{
			"1": {ID: 1, Name: "test", ServerName: "test.local"},
		},
	}
	if err := AtomicWriteJSON(path, rp); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected file to exist")
	}
}

func TestCreateDatedBackup(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "reverseproxy.json")
	original := `{"list":{"1":{"id":1,"name":"test"}}}`
	if err := os.WriteFile(jsonPath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	backupPath, err := CreateDatedBackup(jsonPath, 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("expected backup file to exist")
	}
}

func TestPruneBackups(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		name := filepath.Join(dir, fmt.Sprintf("reverseproxy.json.qnap-docker-mdns.20240101-%04d00.bak", i))
		if err := os.WriteFile(name, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	PruneBackups(dir, 3)
	entries, _ := os.ReadDir(dir)
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	if count != 3 {
		t.Errorf("expected 3 backup files after pruning, got %d", count)
	}
}

func TestPruneBackupsUnderLimit(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		name := filepath.Join(dir, fmt.Sprintf("reverseproxy.json.qnap-docker-mdns.20240101-%04d00.bak", i))
		if err := os.WriteFile(name, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	PruneBackups(dir, 5)
	entries, _ := os.ReadDir(dir)
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	if count != 3 {
		t.Errorf("expected 3 backup files (under limit), got %d", count)
	}
}

func TestRunCommand(t *testing.T) {
	result := RunCommand("echo hello")
	if result.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", result.ExitCode)
	}
}

func TestRunCommandNonExistent(t *testing.T) {
	result := RunCommand("nonexistent-command-12345")
	if result.ExitCode != -1 {
		t.Errorf("expected exit -1 for non-existent command, got %d", result.ExitCode)
	}
}
