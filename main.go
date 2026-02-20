package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/misael/vibessh/internal/hosts"
	"github.com/misael/vibessh/internal/register"
	vibessh "github.com/misael/vibessh/internal/ssh"
	"github.com/misael/vibessh/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	switch len(os.Args) {
	case 1:
		// Interactive TUI picker.
		nodes, err := hosts.Load()
		if err != nil {
			if !errors.Is(err, hosts.ErrNoConfig) {
				return err
			}
			// No config yet — open TUI with empty list; user can press 'a' to add.
			nodes = []hosts.Node{}
		}

		selected, err := tui.Run(nodes)
		if err != nil {
			return err
		}
		if selected == nil {
			// User cancelled.
			return nil
		}
		return vibessh.Connect(*selected)

	case 2:
		arg := os.Args[1]

		if arg == "--register" {
			return register.Run()
		}

		// Try to match against known hosts first.
		nodes, err := hosts.Load()
		if err == nil {
			if node := matchNode(nodes, arg); node != nil {
				return vibessh.Connect(*node)
			}
		}

		// Fall back to passing the argument directly to SSH.
		return vibessh.ConnectRaw(arg)

	default:
		fmt.Fprintf(os.Stderr, "usage: vibessh [hostname|address]\n")
		os.Exit(2)
		return nil
	}
}

// matchNode finds the first node matching by hostname prefix or exact address match.
func matchNode(nodes []hosts.Node, arg string) *hosts.Node {
	lower := strings.ToLower(arg)
	for i, n := range nodes {
		if strings.HasPrefix(strings.ToLower(n.Hostname), lower) {
			return &nodes[i]
		}
		if strings.EqualFold(n.Address, arg) {
			return &nodes[i]
		}
	}
	return nil
}
