package hosts

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Node represents a host entry from the config file.
type Node struct {
	Hostname string `yaml:"hostname"`
	Address  string `yaml:"address"`
	Port     int    `yaml:"port,omitempty"`
	User     string `yaml:"user,omitempty"`
	OS       string `yaml:"os,omitempty"`
}

type config struct {
	Hosts []Node `yaml:"hosts"`
}

// Load reads ~/.vibessh/hosts.yaml and returns the sorted list of nodes.
func Load() ([]Node, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	path := filepath.Join(home, ".vibessh", "hosts.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found: create ~/.vibessh/hosts.yaml — see README")
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse ~/.vibessh/hosts.yaml: %w", err)
	}

	if len(cfg.Hosts) == 0 {
		return nil, fmt.Errorf("no hosts defined in ~/.vibessh/hosts.yaml")
	}

	sort.Slice(cfg.Hosts, func(i, j int) bool {
		return strings.ToLower(cfg.Hosts[i].Hostname) < strings.ToLower(cfg.Hosts[j].Hostname)
	})

	return cfg.Hosts, nil
}
