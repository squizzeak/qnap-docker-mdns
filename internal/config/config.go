package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DomainSuffix string `yaml:"domain_suffix"`

	ReverseProxy ReverseProxyConfig `yaml:"reverse_proxy"`
	Docker       DockerConfig       `yaml:"docker"`
	State        StateConfig        `yaml:"state"`
	Backups      BackupConfig       `yaml:"backups"`
	Reconcile    ReconcileConfig    `yaml:"reconcile"`
	Retry        RetryConfig        `yaml:"retry"`
	ProbeTimeout Duration           `yaml:"probe_timeout"`
}

type ReverseProxyConfig struct {
	JSONDB            string `yaml:"json_db"`
	ConfDir           string `yaml:"conf_dir"`
	ApacheListenPort  int    `yaml:"apache_listen_port"`
	ReloadCommand     string `yaml:"reload_command"`
	ValidateCommand   string `yaml:"validate_command"`
	AccessProfileName string `yaml:"access_profile_name"`
}

type DockerConfig struct {
	Socket string `yaml:"socket"`
}

type StateConfig struct {
	RuntimeDir       string `yaml:"runtime_dir"`
	NoticeStateFile  string `yaml:"notice_state_file"`
	LockFile         string `yaml:"lock_file"`
}

type BackupConfig struct {
	MaxBackups int `yaml:"max_backups"`
}

type ReconcileConfig struct {
	Debounce          Duration `yaml:"debounce"`
	FullRescanInterval Duration `yaml:"full_rescan_interval"`
}

type RetryConfig struct {
	ImmediateRetries int      `yaml:"immediate_retries"`
	InitialBackoff   Duration `yaml:"initial_backoff"`
	MaxBackoff       Duration `yaml:"max_backoff"`
	JitterPercent    int      `yaml:"jitter_percent"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

func DefaultConfig() *Config {
	return &Config{
		DomainSuffix: "local",
		ReverseProxy: ReverseProxyConfig{
			JSONDB:            "/etc/config/reverseproxy/reverseproxy.json",
			ConfDir:           "/etc/reverseproxy/extra",
			ApacheListenPort:  80,
			ReloadCommand:     "/usr/sbin/reverseproxy -k graceful -f /etc/reverseproxy/reverseproxy.conf",
			ValidateCommand:   "/usr/local/apache/bin/apache -t -f /etc/reverseproxy/reverseproxy.conf",
			AccessProfileName: "local",
		},
		Docker: DockerConfig{
			Socket: "/var/run/docker.sock",
		},
		State: StateConfig{
			RuntimeDir:      "/var/run/qnap-docker-mdns",
			NoticeStateFile: "/var/run/qnap-docker-mdns/notice-state.json",
			LockFile:        "/var/run/qnap-docker-mdns/daemon.lock",
		},
		Backups: BackupConfig{
			MaxBackups: 100,
		},
		Reconcile: ReconcileConfig{
			Debounce:          Duration{Duration: 500 * time.Millisecond},
			FullRescanInterval: Duration{Duration: 5 * time.Minute},
		},
		Retry: RetryConfig{
			ImmediateRetries: 1,
			InitialBackoff:   Duration{Duration: 5 * time.Second},
			MaxBackoff:       Duration{Duration: 5 * time.Minute},
			JitterPercent:    20,
		},
		ProbeTimeout: Duration{Duration: 2 * time.Second},
	}
}

func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return &cfg, nil
}

func Merge(base, override *Config) *Config {
	if override == nil {
		return base
	}
	if override.DomainSuffix != "" {
		base.DomainSuffix = override.DomainSuffix
	}
	if override.ReverseProxy.JSONDB != "" {
		base.ReverseProxy.JSONDB = override.ReverseProxy.JSONDB
	}
	if override.ReverseProxy.ConfDir != "" {
		base.ReverseProxy.ConfDir = override.ReverseProxy.ConfDir
	}
	if override.ReverseProxy.ApacheListenPort != 0 {
		base.ReverseProxy.ApacheListenPort = override.ReverseProxy.ApacheListenPort
	}
	if override.ReverseProxy.ReloadCommand != "" {
		base.ReverseProxy.ReloadCommand = override.ReverseProxy.ReloadCommand
	}
	if override.ReverseProxy.ValidateCommand != "" {
		base.ReverseProxy.ValidateCommand = override.ReverseProxy.ValidateCommand
	}
	if override.ReverseProxy.AccessProfileName != "" {
		base.ReverseProxy.AccessProfileName = override.ReverseProxy.AccessProfileName
	}
	if override.Docker.Socket != "" {
		base.Docker.Socket = override.Docker.Socket
	}
	if override.State.RuntimeDir != "" {
		base.State.RuntimeDir = override.State.RuntimeDir
	}
	if override.State.NoticeStateFile != "" {
		base.State.NoticeStateFile = override.State.NoticeStateFile
	}
	if override.State.LockFile != "" {
		base.State.LockFile = override.State.LockFile
	}
	if override.Backups.MaxBackups != 0 {
		base.Backups.MaxBackups = override.Backups.MaxBackups
	}
	if override.Reconcile.Debounce.Duration != 0 {
		base.Reconcile.Debounce = override.Reconcile.Debounce
	}
	if override.Reconcile.FullRescanInterval.Duration != 0 {
		base.Reconcile.FullRescanInterval = override.Reconcile.FullRescanInterval
	}
	if override.Retry.ImmediateRetries != 0 {
		base.Retry.ImmediateRetries = override.Retry.ImmediateRetries
	}
	if override.Retry.InitialBackoff.Duration != 0 {
		base.Retry.InitialBackoff = override.Retry.InitialBackoff
	}
	if override.Retry.MaxBackoff.Duration != 0 {
		base.Retry.MaxBackoff = override.Retry.MaxBackoff
	}
	if override.Retry.JitterPercent != 0 {
		base.Retry.JitterPercent = override.Retry.JitterPercent
	}
	if override.ProbeTimeout.Duration != 0 {
		base.ProbeTimeout = override.ProbeTimeout
	}
	return base
}

func LoadMerged(defaultPath, overridePath string) (*Config, error) {
	cfg := DefaultConfig()

	def, err := LoadFile(defaultPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		cfg = Merge(cfg, def)
	}

	over, err := LoadFile(overridePath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	} else {
		cfg = Merge(cfg, over)
	}

	return cfg, nil
}
