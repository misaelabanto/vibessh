package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/misaelabanto/vibessh/internal/hosts"
)

var (
	formTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginBottom(1)
	labelStyle       = lipgloss.NewStyle().Width(10).Align(lipgloss.Right).Foreground(lipgloss.Color("241"))
	activeLabelStyle = lipgloss.NewStyle().Width(10).Align(lipgloss.Right).Foreground(lipgloss.Color("135"))
	errorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	formHelpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
)

var fieldLabels = [5]string{"Name", "Address", "Port", "User", "OS"}

type formMode int

const (
	modeAdd formMode = iota
	modeEdit
)

type formModel struct {
	inputs       [5]textinput.Model
	focused      int
	err          string
	done         bool
	result       *hosts.Node
	mode         formMode
	originalName string
}

// newBlankInputs builds the five textinputs with placeholders, nothing focused.
func newBlankInputs() [5]textinput.Model {
	var inputs [5]textinput.Model
	placeholders := [5]string{"my-server", "192.168.1.1", "22", "root", "linux"}

	for i := range inputs {
		t := textinput.New()
		t.Placeholder = placeholders[i]
		t.CharLimit = 128
		inputs[i] = t
	}
	return inputs
}

func newFormModel() formModel {
	inputs := newBlankInputs()
	inputs[0].Focus()
	return formModel{inputs: inputs, focused: 0, mode: modeAdd}
}

// newEditFormModel builds a form pre-filled with node's values for editing.
func newEditFormModel(node hosts.Node) formModel {
	inputs := newBlankInputs()
	inputs[0].SetValue(node.Name)
	inputs[1].SetValue(node.Address)
	if node.Port != 0 {
		inputs[2].SetValue(strconv.Itoa(node.Port))
	}
	inputs[3].SetValue(node.User)
	inputs[4].SetValue(node.OS)
	inputs[0].Focus()

	return formModel{inputs: inputs, focused: 0, mode: modeEdit, originalName: node.Name}
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
		return "Name is required"
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
		Name:    strings.TrimSpace(f.inputs[0].Value()),
		Address: strings.TrimSpace(f.inputs[1].Value()),
		Port:    port,
		User:    strings.TrimSpace(f.inputs[3].Value()),
		OS:      strings.TrimSpace(f.inputs[4].Value()),
	}
}

func (f formModel) View() string {
	var b strings.Builder

	title := "Add Host"
	if f.mode == modeEdit {
		title = "Edit Host"
	}
	b.WriteString(formTitleStyle.Render(title) + "\n\n")

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
