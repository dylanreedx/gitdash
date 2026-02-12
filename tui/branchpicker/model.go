package branchpicker

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dylan/gitdash/git"
	"github.com/dylan/gitdash/tui/shared"
)

type Mode int

const (
	PickMode   Mode = iota
	CreateMode
)

type ActionKind int

const (
	ActionNone   ActionKind = iota
	ActionClose
	ActionSwitch
	ActionCreate
)

type KeyResult struct {
	Action     ActionKind
	BranchName string
}

var branchPrefixes = []string{"feat/", "fix/", "chore/", "refactor/", ""}

type Model struct {
	mode         Mode
	branches     []git.BranchInfo
	filtered     []git.BranchInfo
	repoPath     string
	cursor       int
	scrollOffset int

	filterInput textinput.Model
	createInput textinput.Model
	prefixIdx   int

	width  int
	height int
}

func New() Model {
	fi := textinput.New()
	fi.Placeholder = "filter branches..."
	fi.CharLimit = 100

	ci := textinput.New()
	ci.Placeholder = "branch name..."
	ci.CharLimit = 100

	return Model{
		filterInput: fi,
		createInput: ci,
	}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *Model) SetBranches(branches []git.BranchInfo, repoPath string) {
	m.branches = branches
	m.repoPath = repoPath
	m.mode = PickMode
	m.cursor = 0
	m.scrollOffset = 0
	m.prefixIdx = 0
	m.filterInput.SetValue("")
	m.filterInput.Focus()
	m.createInput.SetValue("")
	m.applyFilter()
}

func (m *Model) applyFilter() {
	query := strings.ToLower(m.filterInput.Value())
	if query == "" {
		m.filtered = m.branches
		return
	}
	m.filtered = nil
	for _, b := range m.branches {
		if strings.Contains(strings.ToLower(b.Name), query) {
			m.filtered = append(m.filtered, b)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	m.scrollOffset = 0
	m.ensureCursorVisible()
}

// listHeight returns how many branch items fit in the visible area.
func (m Model) listHeight() int {
	h := 15
	if len(m.filtered) < h {
		h = len(m.filtered)
	}
	if h < 1 {
		h = 1
	}
	return h
}

func (m *Model) ensureCursorVisible() {
	h := m.listHeight()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+h {
		m.scrollOffset = m.cursor - h + 1
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.mode == PickMode {
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.applyFilter()
	} else {
		m.createInput, cmd = m.createInput.Update(msg)
	}
	return m, cmd
}

func (m *Model) HandleKey(msg tea.KeyMsg) KeyResult {
	switch m.mode {
	case PickMode:
		return m.handlePickKey(msg)
	case CreateMode:
		return m.handleCreateKey(msg)
	}
	return KeyResult{Action: ActionNone}
}

func (m *Model) handlePickKey(msg tea.KeyMsg) KeyResult {
	switch msg.String() {
	case "esc", "q":
		return KeyResult{Action: ActionClose}
	case "j", "down":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.ensureCursorVisible()
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
		}
	case "enter":
		if m.cursor < len(m.filtered) {
			return KeyResult{Action: ActionSwitch, BranchName: m.filtered[m.cursor].Name}
		}
	case "n":
		m.mode = CreateMode
		m.filterInput.Blur()
		m.createInput.SetValue("")
		m.createInput.Focus()
		m.prefixIdx = 0
	}
	return KeyResult{Action: ActionNone}
}

func (m *Model) handleCreateKey(msg tea.KeyMsg) KeyResult {
	switch msg.String() {
	case "esc":
		m.mode = PickMode
		m.createInput.Blur()
		m.filterInput.Focus()
		return KeyResult{Action: ActionNone}
	case "tab":
		m.prefixIdx = (m.prefixIdx + 1) % len(branchPrefixes)
	case "enter":
		name := strings.TrimSpace(m.createInput.Value())
		if name == "" {
			return KeyResult{Action: ActionNone}
		}
		prefix := branchPrefixes[m.prefixIdx]
		return KeyResult{Action: ActionCreate, BranchName: prefix + name}
	}
	return KeyResult{Action: ActionNone}
}

func (m Model) ViewOverlay(background string, w, h int) string {
	content := m.renderContent()
	overlay := shared.BranchPickerOverlayStyle.Render(content)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
	)
}

func (m Model) renderContent() string {
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Render("Branches")
	b.WriteString(title)
	b.WriteString(" ")
	b.WriteString(shared.GraphHashStyle.Render(m.repoPath))
	b.WriteString("\n\n")

	if m.mode == PickMode {
		b.WriteString(m.renderPickMode())
	} else {
		b.WriteString(m.renderCreateMode())
	}

	return b.String()
}

func (m Model) renderPickMode() string {
	var b strings.Builder

	b.WriteString(m.filterInput.View())
	b.WriteString("\n\n")

	visibleH := m.listHeight()
	end := m.scrollOffset + visibleH
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := m.scrollOffset; i < end; i++ {
		branch := m.filtered[i]
		marker := "  "
		style := shared.BranchItemStyle
		if branch.IsCurrent {
			marker = "* "
			style = shared.BranchCurrentStyle
		}

		line := marker + style.Render(branch.Name)
		if branch.Upstream != "" {
			line += " " + shared.GraphHashStyle.Render("→ "+branch.Upstream)
		}

		if i == m.cursor {
			line = shared.CursorStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString(shared.GraphHashStyle.Render("  no matching branches"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(shared.HelpDescStyle.Render("j/k: navigate  enter: switch  n: new branch  esc: close"))

	return b.String()
}

func (m Model) renderCreateMode() string {
	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Render("New Branch"))
	b.WriteString("\n\n")

	// Prefix selector
	b.WriteString("Prefix: ")
	for i, p := range branchPrefixes {
		label := p
		if label == "" {
			label = "(none)"
		}
		if i == m.prefixIdx {
			b.WriteString(shared.BranchPrefixStyle.Render("[" + label + "]"))
		} else {
			b.WriteString(shared.GraphHashStyle.Render(" " + label + " "))
		}
		b.WriteString(" ")
	}
	b.WriteString("\n\n")

	// Preview
	prefix := branchPrefixes[m.prefixIdx]
	name := m.createInput.Value()
	if name != "" {
		b.WriteString("Preview: ")
		b.WriteString(shared.BranchCurrentStyle.Render(prefix + name))
		b.WriteString("\n\n")
	}

	b.WriteString(m.createInput.View())
	b.WriteString("\n\n")
	b.WriteString(shared.HelpDescStyle.Render("tab: cycle prefix  enter: create  esc: back"))

	return b.String()
}

