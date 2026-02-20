package register

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

const serviceTemplate = `[Unit]
Description=vibessh reverse tunnel
After=network-online.target

[Service]
ExecStart=autossh -M 0 -o ServerAliveInterval=30 -o ServerAliveCountMax=3 \
  -o ExitOnForwardFailure=yes -N \
  -R {{.RemotePort}}:localhost:22 {{.User}}@{{.VPS}}
Restart=always

[Install]
WantedBy=default.target
`

type serviceParams struct {
	VPS        string
	User       string
	RemotePort string
}

// Run sets up the autossh reverse-tunnel systemd user service.
func Run() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("--register is only supported on Linux (current OS: %s)", runtime.GOOS)
	}

	if _, err := exec.LookPath("autossh"); err != nil {
		return fmt.Errorf("autossh not found in PATH — install it first (e.g. sudo apt install autossh)")
	}

	scanner := bufio.NewScanner(os.Stdin)

	params := serviceParams{RemotePort: "2222"}

	params.VPS = prompt(scanner, "VPS address (e.g. your-vps.com): ")
	if params.VPS == "" {
		return fmt.Errorf("VPS address is required")
	}

	params.User = prompt(scanner, "VPS user (e.g. ubuntu): ")
	if params.User == "" {
		return fmt.Errorf("VPS user is required")
	}

	remotePort := prompt(scanner, "Remote port [2222]: ")
	if strings.TrimSpace(remotePort) != "" {
		params.RemotePort = strings.TrimSpace(remotePort)
	}

	tmpl, err := template.New("service").Parse(serviceTemplate)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	serviceDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("create systemd user dir: %w", err)
	}

	servicePath := filepath.Join(serviceDir, "vibestunnel.service")
	if err := os.WriteFile(servicePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}

	fmt.Printf("Wrote %s\n", servicePath)

	if err := runCmd("systemctl", "--user", "daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}

	if err := runCmd("systemctl", "--user", "enable", "--now", "vibestunnel"); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}

	fmt.Println()
	fmt.Println("vibestunnel service enabled and started.")
	fmt.Printf("The tunnel will forward remote port %s → localhost:22\n", params.RemotePort)
	fmt.Println()
	fmt.Println("On your VPS, make sure sshd_config contains:")
	fmt.Println("  GatewayPorts yes")

	return nil
}

func prompt(scanner *bufio.Scanner, label string) string {
	fmt.Print(label)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
