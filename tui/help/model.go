package help

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dylan/gitdash/tui/shared"
)

type Model struct {
	width  int
	height int
}

func New() Model {
	return Model{}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33")).Render("GitDash Help"))
	b.WriteString("\n\n")

	groups := shared.Keys.FullHelp()
	groupNames := []string{"Navigation", "Focus", "Staging", "Actions", "General"}

	for i, group := range groups {
		if i < len(groupNames) {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Render(groupNames[i]))
			b.WriteString("\n")
		}
		for _, k := range group {
			help := k.Help()
			key := shared.HelpKeyStyle.Render(help.Key)
			desc := shared.HelpDescStyle.Render(help.Desc)
			b.WriteString("  " + key + "  " + desc + "\n")
		}
		b.WriteString("\n")
	}

	content := shared.HelpOverlayStyle.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
