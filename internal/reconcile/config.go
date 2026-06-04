package reconcile

import (
	"time"

	"github.com/squizzeak/qnap-docker-mdns/internal/config"
)

type ConfigAdapter struct {
	*config.Config
}

func (a *ConfigAdapter) DomainSuffix() string {
	return a.Config.DomainSuffix
}

func (a *ConfigAdapter) ProbeTimeout() time.Duration {
	return a.Config.ProbeTimeout.Duration
}

func (a *ConfigAdapter) DebounceWindow() time.Duration {
	return a.Config.Reconcile.Debounce.Duration
}

func (a *ConfigAdapter) FullRescanInterval() time.Duration {
	return a.Config.Reconcile.FullRescanInterval.Duration
}

func (a *ConfigAdapter) JSONPath() string {
	return a.Config.ReverseProxy.JSONDB
}

func (a *ConfigAdapter) AccessProfilePath() string {
	return "/etc/config/reverseproxy/access.json"
}

func (a *ConfigAdapter) ListenPort() int {
	return a.Config.ReverseProxy.ApacheListenPort
}

func (a *ConfigAdapter) ReloadCommand() string {
	return a.Config.ReverseProxy.ReloadCommand
}

func (a *ConfigAdapter) ValidateCommand() string {
	return a.Config.ReverseProxy.ValidateCommand
}

func (a *ConfigAdapter) MaxBackups() int {
	return a.Config.Backups.MaxBackups
}

func (a *ConfigAdapter) LockFilePath() string {
	return a.Config.State.LockFile
}

func (a *ConfigAdapter) NoticeStateFile() string {
	return a.Config.State.NoticeStateFile
}
