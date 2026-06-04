package proxy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func CreateDatedBackup(jsonPath string, maxBackups int) (string, error) {
	src, err := os.Open(jsonPath)
	if err != nil {
		return "", fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	ts := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("reverseproxy.json.qnap-docker-mdns.%s.bak", ts)
	backupPath := filepath.Join(filepath.Dir(jsonPath), backupName)

	dst, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("create backup: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("copy backup: %w", err)
	}

	PruneBackups(filepath.Dir(jsonPath), maxBackups)
	return backupPath, nil
}

func PruneBackups(dir string, maxBackups int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var backups []string
	prefix := "reverseproxy.json.qnap-docker-mdns."
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".bak") {
			backups = append(backups, e.Name())
		}
	}

	if len(backups) <= maxBackups {
		return
	}

	sort.Strings(backups)
	toDelete := len(backups) - maxBackups
	for i := 0; i < toDelete; i++ {
		os.Remove(filepath.Join(dir, backups[i]))
	}
}

func AtomicWriteJSON(path string, rp *ReverseProxyJSON) error {
	data, err := yaml.Marshal(rp)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
