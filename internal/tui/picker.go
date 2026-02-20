package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/misael/vibessh/internal/hosts"
)

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2).Bold(true).Foreground(lipgloss.Color("205"))
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	dimStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
)

// nodeItem wraps a Node to satisfy list.Item and list.DefaultItem.
type nodeItem struct {
	node hosts.Node
}

func (n nodeItem) Title() string {
	return n.node.Hostname
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
	return n.node.Hostname + " " + n.node.Address
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

// Model is the bubbletea application model.
type Model struct {
	list     list.Model
	selected *hosts.Node
	quitting bool
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

	return Model{list: l}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			// Guard: don't trigger when the user is typing a filter.
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

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	return m.list.View()
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
