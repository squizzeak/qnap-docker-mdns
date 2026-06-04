package proxy

import (
	"encoding/json"
	"os"
)

type SyncResult struct {
	Success bool
	Error   string
}

type SyncConfig interface {
	JSONPath() string
	ValidateCommand() string
	ReloadCommand() string
	MaxBackups() int
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Sync(current, updated *ReverseProxyJSON, cfg SyncConfig) SyncResult {
	renderStr := jsonStr(updated)
	currentStr := jsonStr(current)

	if renderStr == currentStr {
		return SyncResult{Success: true}
	}

	if CountEntries(updated) > MaxRules {
		return SyncResult{Success: false, Error: "exceeds 64 rule limit"}
	}

	backupPath, err := CreateDatedBackup(cfg.JSONPath(), cfg.MaxBackups())
	if err != nil {
		return SyncResult{Success: false, Error: err.Error()}
	}

	if err := AtomicWriteJSON(cfg.JSONPath(), updated); err != nil {
		restoreBackup(cfg.JSONPath(), backupPath)
		return SyncResult{Success: false, Error: err.Error()}
	}

	scanResult := RunScanConfig()
	if scanResult.ExitCode != 0 {
		restoreBackup(cfg.JSONPath(), backupPath)
		RunScanConfig()
		return SyncResult{Success: false, Error: "scan_config failed: " + scanResult.Stderr}
	}

	validateResult := RunValidate(cfg.ValidateCommand())
	if validateResult.ExitCode != 0 {
		restoreBackup(cfg.JSONPath(), backupPath)
		RunScanConfig()
		return SyncResult{Success: false, Error: "validation failed: " + validateResult.Stderr}
	}

	reloadResult := RunReload(cfg.ReloadCommand())
	if reloadResult.ExitCode != 0 {
		restoreBackup(cfg.JSONPath(), backupPath)
		RunScanConfig()
		return SyncResult{Success: false, Error: "reload failed: " + reloadResult.Stderr}
	}

	return SyncResult{Success: true}
}

func jsonStr(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func restoreBackup(jsonPath, backupPath string) {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return
	}
	os.WriteFile(jsonPath, data, 0644)
}
