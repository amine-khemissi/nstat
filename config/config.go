package config

import (
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type TCPTargetConfig struct {
	Host string
	Port int
}

type Config struct {
	PingTarget     string
	PingTarget2    string
	TCPHost        string
	TCPPort        int
	TCPTargets     []TCPTargetConfig // multiple TCP targets for granular testing
	PingInterval   time.Duration
	LANInterval    time.Duration
	RTTWindow      int
	LossThreshold  int
	LogRotateEvery time.Duration
	MTUEnabled     bool // enable MTU probing
	KernelStats    bool // enable kernel TCP stats collection

	Dir       string
	LogFile   string
	StateFile string
	PIDFile   string
	GraphFile string
}

func dataDir() string {
	switch runtime.GOOS {
	case "windows":
		if d := os.Getenv("APPDATA"); d != "" {
			return filepath.Join(d, "nstat")
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "AppData", "Roaming", "nstat")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "nstat")
		}
	}
	// Linux / other Unix
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "nstat")
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "nstat")
}

func Default() *Config {
	dir := dataDir()
	return &Config{
		PingTarget:  "8.8.8.8",
		PingTarget2: "1.1.1.1",
		TCPHost:     "8.8.8.8",
		TCPPort:     53,
		TCPTargets: []TCPTargetConfig{
			{Host: "8.8.8.8", Port: 53},   // Google DNS
			{Host: "1.1.1.1", Port: 53},   // Cloudflare DNS
			{Host: "8.8.8.8", Port: 443},  // Google HTTPS (tests different port)
		},
		PingInterval:   5 * time.Second,
		LANInterval:    30 * time.Second,
		RTTWindow:      60,
		LossThreshold:  3,
		LogRotateEvery: 24 * time.Hour,
		MTUEnabled:     true,
		KernelStats:    true,
		Dir:            dir,
		LogFile:        filepath.Join(dir, "nstat.log"),
		StateFile:      filepath.Join(dir, "nstat.state.json"),
		PIDFile:        filepath.Join(dir, "nstat.pid"),
		GraphFile:      filepath.Join(dir, "nstat_graph.svg"),
	}
}
