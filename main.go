package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	vibessh "github.com/misael/vibessh/internal/ssh"
	"github.com/misael/vibessh/internal/tailscale"
	"github.com/misael/vibessh/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	client := tailscale.NewClient("")

	switch len(os.Args) {
	case 1:
		// Interactive TUI picker.
		nodes, err := client.Peers(ctx, false)
		if err != nil {
			return err
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

		// Try to match against known peers first.
		nodes, err := client.Peers(ctx, true)
		if err == nil {
			if node := matchNode(nodes, arg); node != nil {
				return vibessh.Connect(*node)
			}
		}

		// Fall back to passing the argument directly to SSH.
		return vibessh.ConnectRaw(arg)

	default:
		fmt.Fprintf(os.Stderr, "usage: vibessh [hostname|ip]\n")
		os.Exit(2)
		return nil
	}
}

// matchNode finds the first node matching by hostname prefix, DNSName prefix, or exact IP.
func matchNode(nodes []tailscale.Node, arg string) *tailscale.Node {
	lower := strings.ToLower(arg)
	for i, n := range nodes {
		if strings.HasPrefix(strings.ToLower(n.Hostname), lower) {
			return &nodes[i]
		}
		if strings.HasPrefix(strings.ToLower(n.DNSName), lower) {
			return &nodes[i]
		}
		if n.IP.IsValid() && n.IP.String() == arg {
			return &nodes[i]
		}
	}
	return nil
}
