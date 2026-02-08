package diffview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dylan/gitdash/tui/shared"
)

type Model struct {
	viewport viewport.Model
	file     string
	repoPath string
	ready    bool
	width    int
	height   int
}

func New() Model {
	return Model{}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	headerHeight := 1
	footerHeight := 1
	contentHeight := h - headerHeight - footerHeight
	if contentHeight < 1 {
		contentHeight = 1
	}
	m.viewport = viewport.New(w, contentHeight)
	m.viewport.YPosition = headerHeight
	m.ready = true
}

func (m *Model) SetContent(rawDiff, file, repoPath string) {
	m.file = file
	m.repoPath = repoPath
	styled := styleDiff(rawDiff)
	m.viewport.SetContent(styled)
	m.viewport.GotoTop()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	header := shared.DiffHeaderStyle.Width(m.width).Render(fmt.Sprintf(" Diff: %s", m.file))
	footer := shared.DiffFooterStyle.Width(m.width).Render("j/k: scroll  s: stage  u: unstage  q/esc: close")

	return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)
}

func styleDiff(raw string) string {
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- "):
			b.WriteString(shared.DiffMetaStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			b.WriteString(shared.DiffHunkStyle.Render(line))
		case strings.HasPrefix(line, "+"):
			b.WriteString(shared.DiffAddStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			b.WriteString(shared.DiffRemoveStyle.Render(line))
		case strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index "):
			b.WriteString(shared.DiffMetaStyle.Render(line))
		default:
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return b.String()
}
