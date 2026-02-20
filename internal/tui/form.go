package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/misael/vibessh/internal/hosts"
)

var (
	formTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginBottom(1)
	labelStyle      = lipgloss.NewStyle().Width(10).Align(lipgloss.Right).Foreground(lipgloss.Color("241"))
	activeLabelStyle = lipgloss.NewStyle().Width(10).Align(lipgloss.Right).Foreground(lipgloss.Color("135"))
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	formHelpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
)

var fieldLabels = [5]string{"Hostname", "Address", "Port", "User", "OS"}

type formModel struct {
	inputs  [5]textinput.Model
	focused int
	err     string
	done    bool
	result  *hosts.Node
}

func newFormModel() formModel {
	var inputs [5]textinput.Model
	placeholders := [5]string{"my-server", "192.168.1.1", "22", "root", "linux"}

	for i := range inputs {
		t := textinput.New()
		t.Placeholder = placeholders[i]
		t.CharLimit = 128
		inputs[i] = t
	}
	inputs[0].Focus()

	return formModel{inputs: inputs, focused: 0}
}

func (f formModel) Init() tea.Cmd {
	return textinput.Blink
}

func (f formModel) Update(msg tea.Msg) (formModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			f.done = true
			f.result = nil
			return f, nil

		case "enter":
			if errMsg := f.validate(); errMsg != "" {
				f.err = errMsg
				return f, nil
			}
			node := f.toNode()
			f.result = &node
			f.done = true
			return f, nil

		case "tab", "down":
			f.inputs[f.focused].Blur()
			f.focused = (f.focused + 1) % len(f.inputs)
			f.inputs[f.focused].Focus()
			return f, textinput.Blink

		case "shift+tab", "up":
			f.inputs[f.focused].Blur()
			f.focused = (f.focused - 1 + len(f.inputs)) % len(f.inputs)
			f.inputs[f.focused].Focus()
			return f, textinput.Blink
		}
	}

	var cmd tea.Cmd
	f.inputs[f.focused], cmd = f.inputs[f.focused].Update(msg)
	return f, cmd
}

func (f formModel) validate() string {
	if strings.TrimSpace(f.inputs[0].Value()) == "" {
		return "Hostname is required"
	}
	if strings.TrimSpace(f.inputs[1].Value()) == "" {
		return "Address is required"
	}
	if v := strings.TrimSpace(f.inputs[2].Value()); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 {
			return "Port must be a number between 1 and 65535"
		}
	}
	return ""
}

func (f formModel) toNode() hosts.Node {
	port := 0
	if v := strings.TrimSpace(f.inputs[2].Value()); v != "" {
		port, _ = strconv.Atoi(v)
	}
	return hosts.Node{
		Hostname: strings.TrimSpace(f.inputs[0].Value()),
		Address:  strings.TrimSpace(f.inputs[1].Value()),
		Port:     port,
		User:     strings.TrimSpace(f.inputs[3].Value()),
		OS:       strings.TrimSpace(f.inputs[4].Value()),
	}
}

func (f formModel) View() string {
	var b strings.Builder

	b.WriteString(formTitleStyle.Render("Add Host") + "\n\n")

	for i, input := range f.inputs {
		lbl := fieldLabels[i]
		var label string
		if i == f.focused {
			label = activeLabelStyle.Render(lbl)
		} else {
			label = labelStyle.Render(lbl)
		}
		b.WriteString(label + "  " + input.View() + "\n")
	}

	if f.err != "" {
		b.WriteString("\n" + errorStyle.Render(f.err) + "\n")
	}

	b.WriteString(formHelpStyle.Render("\ntab/shift+tab navigate • enter submit • esc cancel"))

	return b.String()
}
