package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/misaelabanto/vibessh/internal/hosts"
)

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2).Bold(true).Foreground(lipgloss.Color("205"))
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	dimStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	emptyTitleStyle   = lipgloss.NewStyle().MarginLeft(2).Bold(true).Foreground(lipgloss.Color("205"))
	emptyHintStyle    = lipgloss.NewStyle().MarginLeft(2).Foreground(lipgloss.Color("241"))
	emptyKeyStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	emptyCodeStyle    = lipgloss.NewStyle().MarginLeft(4).Foreground(lipgloss.Color("243"))
	bannerStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	confirmStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	confirmNameStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
)

const banner = "" +
	"            __                              __         \n" +
	"         __/\\ \\                            /\\ \\        \n" +
	" __  __ /\\_\\ \\ \\____     __    ____    ____\\ \\ \\___    \n" +
	"/\\ \\/\\ \\\\/\\ \\ \\ '__`\\  /'__`\\ /',__\\  /',__\\\\ \\  _ `\\  \n" +
	"\\ \\ \\_/ |\\ \\ \\ \\ \\L\\ \\/\\  __//\\__, `\\/\\__, `\\\\ \\ \\ \\ \\ \n" +
	" \\ \\___/  \\ \\_\\ \\_,__/\\ \\____\\/\\____/\\/\\____/ \\ \\_\\ \\_\\\n" +
	"  \\/__/    \\/_/\\/___/  \\/____/\\/___/  \\/___/   \\/_/\\/_/"

var addKey = key.NewBinding(
	key.WithKeys("a"),
	key.WithHelp("a", "add host"),
)

var editKey = key.NewBinding(
	key.WithKeys("e"),
	key.WithHelp("e", "edit host"),
)

var deleteKey = key.NewBinding(
	key.WithKeys("d"),
	key.WithHelp("d", "delete host"),
)

// nodeItem wraps a Node to satisfy list.Item and list.DefaultItem.
type nodeItem struct {
	node hosts.Node
}

func (n nodeItem) Title() string {
	return n.node.Name
}

func (n nodeItem) Description() string {
	parts := []string{n.node.Address}
	if n.node.OS != "" {
		parts = append(parts, n.node.OS)
	}
	if n.node.Port != 0 && n.node.Port != 22 {
		parts = append(parts, fmt.Sprintf("port %d", n.node.Port))
	}
	return strings.Join(parts, "  ")
}

func (n nodeItem) FilterValue() string {
	return n.node.Name + " " + n.node.Address
}

// itemDelegate renders each list item.
type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 2 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ni, ok := item.(nodeItem)
	if !ok {
		return
	}

	if index == m.Index() {
		fmt.Fprintf(w, "%s\n%s",
			selectedItemStyle.Render("> "+ni.Title()),
			selectedItemStyle.Render("  "+ni.Description()),
		)
	} else {
		fmt.Fprintf(w, "%s\n%s",
			itemStyle.Render(ni.Title()),
			itemStyle.Render(dimStyle.Render(ni.Description())),
		)
	}
}

type pickerState int

const (
	stateList pickerState = iota
	stateForm
	stateConfirmDelete
)

// Model is the bubbletea application model.
type Model struct {
	list        list.Model
	form        formModel
	state       pickerState
	selected    *hosts.Node
	quitting    bool
	confirmName string
	status      string
}

func newModel(nodes []hosts.Node) Model {
	items := make([]list.Item, len(nodes))
	for i, n := range nodes {
		items[i] = nodeItem{node: n}
	}

	l := list.New(items, itemDelegate{}, 0, 0)
	l.Title = "vibessh — Select a node"
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding { return []key.Binding{addKey, editKey, deleteKey} }
	l.AdditionalFullHelpKeys = func() []key.Binding { return []key.Binding{addKey, editKey, deleteKey} }

	return Model{list: l}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		bannerHeight := lipgloss.Height(bannerStyle.Render(banner))
		m.list.SetSize(msg.Width, msg.Height-bannerHeight-1)
		return m, nil
	}

	switch m.state {
	case stateList:
		return m.updateList(msg)
	case stateForm:
		return m.updateForm(msg)
	case stateConfirmDelete:
		return m.updateConfirmDelete(msg)
	}
	return m, nil
}

func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "a":
			if m.list.FilterState() != list.Filtering {
				m.status = ""
				m.state = stateForm
				m.form = newFormModel()
				return m, textinputBlink()
			}

		case "e":
			if m.list.FilterState() != list.Filtering {
				if item, ok := m.list.SelectedItem().(nodeItem); ok {
					m.status = ""
					m.state = stateForm
					m.form = newEditFormModel(item.node)
					return m, textinputBlink()
				}
			}

		case "d":
			if m.list.FilterState() != list.Filtering {
				if item, ok := m.list.SelectedItem().(nodeItem); ok {
					m.status = ""
					m.confirmName = item.node.Name
					m.state = stateConfirmDelete
					return m, nil
				}
			}

		case "enter":
			if m.list.FilterState() == list.Filtering {
				break
			}
			if item, ok := m.list.SelectedItem().(nodeItem); ok {
				n := item.node
				m.selected = &n
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)

	if m.form.done {
		if m.form.result != nil {
			node := *m.form.result
			if m.form.mode == modeEdit {
				if err := hosts.Update(m.form.originalName, node); err != nil {
					m.status = "Could not save host: " + err.Error()
				} else {
					m.replaceItem(m.form.originalName, node)
				}
			} else {
				if err := hosts.Append(node); err != nil {
					m.status = "Could not save host: " + err.Error()
				} else {
					m.list.InsertItem(len(m.list.Items()), nodeItem{node: node})
				}
			}
		}
		m.state = stateList
		m.form = formModel{}
	}

	return m, cmd
}

// replaceItem swaps the list item named oldName with one holding node.
func (m *Model) replaceItem(oldName string, node hosts.Node) {
	for i, it := range m.list.Items() {
		if ni, ok := it.(nodeItem); ok && ni.node.Name == oldName {
			m.list.SetItem(i, nodeItem{node: node})
			return
		}
	}
}

func (m Model) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "y", "Y":
			if err := hosts.Delete(m.confirmName); err != nil {
				m.status = "Could not delete host: " + err.Error()
			} else {
				m.removeItem(m.confirmName)
			}
			m.confirmName = ""
			m.state = stateList
		case "n", "N", "esc", "q", "ctrl+c":
			m.confirmName = ""
			m.state = stateList
		}
	}
	return m, nil
}

// removeItem deletes the list item with the given name, if present.
func (m *Model) removeItem(name string) {
	for i, it := range m.list.Items() {
		if ni, ok := it.(nodeItem); ok && ni.node.Name == name {
			m.list.RemoveItem(i)
			return
		}
	}
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.state == stateForm {
		return m.form.View()
	}

	b := bannerStyle.Render(banner)
	sections := []string{b}
	if m.status != "" {
		sections = append(sections, errorStyle.Render(m.status))
	}

	switch {
	case m.state == stateConfirmDelete:
		sections = append(sections, m.confirmView())
	case len(m.list.Items()) == 0:
		sections = append(sections, m.emptyView())
	default:
		sections = append(sections, m.list.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) confirmView() string {
	prompt := confirmStyle.Render("Delete host ") +
		confirmNameStyle.Render(`"`+m.confirmName+`"`) +
		confirmStyle.Render("? (y/n)")
	return "\n  " + prompt + "\n\n" +
		formHelpStyle.Render("  y delete • n/esc cancel")
}

func (m Model) emptyView() string {
	return emptyTitleStyle.Render("vibessh — No hosts configured") + "\n\n" +
		emptyHintStyle.Render("Press "+emptyKeyStyle.Render("a")+" to add your first host interactively, or create:") + "\n\n" +
		emptyCodeStyle.Render("~/.vibessh/hosts.yaml") + "\n\n" +
		emptyCodeStyle.Render("hosts:") + "\n" +
		emptyCodeStyle.Render("  - name: my-server") + "\n" +
		emptyCodeStyle.Render("    address:  192.168.1.100") + "\n" +
		emptyCodeStyle.Render("    user:     ubuntu") + "\n\n" +
		emptyHintStyle.Render("Press "+emptyKeyStyle.Render("q")+" to quit.")
}

// textinputBlink returns the Blink command for textinput.
func textinputBlink() tea.Cmd {
	return newFormModel().Init()
}

// Run displays the TUI picker and returns the selected node, or nil if cancelled.
func Run(nodes []hosts.Node) (*hosts.Node, error) {
	m := newModel(nodes)
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("tui: %w", err)
	}
	final := result.(Model)
	return final.selected, nil
}
