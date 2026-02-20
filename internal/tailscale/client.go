package tailscale

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"tailscale.com/client/local"
)

// Node represents a Tailscale peer.
type Node struct {
	Hostname string
	DNSName  string
	OS       string
	IP       netip.Addr
	Online   bool
}

// Client wraps the Tailscale local API client.
type Client struct {
	lc *local.Client
}

// NewClient returns a Client. Pass an empty socketPath to use the system default.
func NewClient(socketPath string) *Client {
	lc := &local.Client{}
	if socketPath != "" {
		lc.Socket = socketPath
	}
	return &Client{lc: lc}
}

// Peers returns Tailscale peers, optionally including offline nodes.
func (c *Client) Peers(ctx context.Context, includeOffline bool) ([]Node, error) {
	st, err := c.lc.Status(ctx)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "no such file") || strings.Contains(msg, "not found") {
			return nil, fmt.Errorf("tailscaled not running? Try: sudo tailscale status")
		}
		if strings.Contains(msg, "permission denied") {
			return nil, fmt.Errorf("permission denied: add yourself to the tailscale group")
		}
		return nil, fmt.Errorf("tailscale status: %w", err)
	}

	if st.BackendState != "Running" {
		return nil, fmt.Errorf("tailscale is not running (state: %s). Run: tailscale up", st.BackendState)
	}

	var nodes []Node
	for _, peer := range st.Peer {
		online := peer.Online
		if !online && !includeOffline {
			continue
		}

		dnsName := strings.TrimSuffix(peer.DNSName, ".")

		var ip netip.Addr
		for _, addr := range peer.TailscaleIPs {
			if addr.Is4() {
				ip = addr
				break
			}
		}

		nodes = append(nodes, Node{
			Hostname: peer.HostName,
			DNSName:  dnsName,
			OS:       peer.OS,
			IP:       ip,
			Online:   online,
		})
	}

	if len(nodes) == 0 {
		if includeOffline {
			return nil, fmt.Errorf("no Tailscale peers found")
		}
		return nil, fmt.Errorf("no online Tailscale peers found")
	}

	sort.Slice(nodes, func(i, j int) bool {
		return strings.ToLower(nodes[i].Hostname) < strings.ToLower(nodes[j].Hostname)
	})

	return nodes, nil
}
