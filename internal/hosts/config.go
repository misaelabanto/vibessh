package hosts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrNoConfig is returned by Load when the config file does not exist.
var ErrNoConfig = errors.New("no config file")

// Node represents a host entry from the config file.
type Node struct {
	Name string `yaml:"name"`
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
			return nil, ErrNoConfig
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse ~/.vibessh/hosts.yaml: %w", err)
	}

	sort.Slice(cfg.Hosts, func(i, j int) bool {
		return strings.ToLower(cfg.Hosts[i].Name) < strings.ToLower(cfg.Hosts[j].Name)
	})

	return cfg.Hosts, nil
}

// Append adds a node to ~/.vibessh/hosts.yaml, creating the file if needed.
func Append(node Node) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	dir := filepath.Join(home, ".vibessh")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	path := filepath.Join(dir, "hosts.yaml")

	var cfg config
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse hosts.yaml: %w", err)
		}
	}

	cfg.Hosts = append(cfg.Hosts, node)

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal hosts.yaml: %w", err)
	}

	if err := os.WriteFile(path, out, 0600); err != nil {
		return fmt.Errorf("write hosts.yaml: %w", err)
	}

	return nil
}
