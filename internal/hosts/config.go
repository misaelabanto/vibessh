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
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
	Port    int    `yaml:"port,omitempty"`
	User    string `yaml:"user,omitempty"`
	OS      string `yaml:"os,omitempty"`
}

type config struct {
	Hosts []Node `yaml:"hosts"`
}

// configPath returns the path to ~/.vibessh/hosts.yaml.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".vibessh", "hosts.yaml"), nil
}

// readConfig reads and parses the config file. A missing file yields an empty
// config (no error) so callers can create it on the next write.
func readConfig() (config, error) {
	var cfg config

	path, err := configPath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse ~/.vibessh/hosts.yaml: %w", err)
	}

	return cfg, nil
}

// writeConfig marshals and writes the config file, creating ~/.vibessh if needed.
func writeConfig(cfg config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal hosts.yaml: %w", err)
	}

	if err := os.WriteFile(path, out, 0600); err != nil {
		return fmt.Errorf("write hosts.yaml: %w", err)
	}

	return nil
}

// Load reads ~/.vibessh/hosts.yaml and returns the sorted list of nodes.
func Load() ([]Node, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, ErrNoConfig
	}

	cfg, err := readConfig()
	if err != nil {
		return nil, err
	}

	sort.Slice(cfg.Hosts, func(i, j int) bool {
		return strings.ToLower(cfg.Hosts[i].Name) < strings.ToLower(cfg.Hosts[j].Name)
	})

	return cfg.Hosts, nil
}

// Append adds a node to ~/.vibessh/hosts.yaml, creating the file if needed.
func Append(node Node) error {
	cfg, err := readConfig()
	if err != nil {
		return err
	}

	cfg.Hosts = append(cfg.Hosts, node)
	return writeConfig(cfg)
}

// Update replaces the host named oldName with node, allowing the name to change.
// It returns an error if no host matches oldName.
func Update(oldName string, node Node) error {
	cfg, err := readConfig()
	if err != nil {
		return err
	}

	for i := range cfg.Hosts {
		if cfg.Hosts[i].Name == oldName {
			cfg.Hosts[i] = node
			return writeConfig(cfg)
		}
	}

	return fmt.Errorf("host %q not found", oldName)
}

// Delete removes the host with the given name. It returns an error if no host
// matches.
func Delete(name string) error {
	cfg, err := readConfig()
	if err != nil {
		return err
	}

	for i := range cfg.Hosts {
		if cfg.Hosts[i].Name == name {
			cfg.Hosts = append(cfg.Hosts[:i], cfg.Hosts[i+1:]...)
			return writeConfig(cfg)
		}
	}

	return fmt.Errorf("host %q not found", name)
}
