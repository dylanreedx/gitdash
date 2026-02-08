package featurelinker

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dylan/gitdash/conductor"
	"github.com/dylan/gitdash/tui/shared"
)

type Model struct {
	matches    []conductor.FeatureMatch
	cursor     int
	visible    bool
	commitHash string
	commitMsg  string
	width      int
	height     int
}

func New() Model {
	return Model{}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *Model) Show(matches []conductor.FeatureMatch, hash, msg string) {
	m.matches = matches
	m.cursor = 0 // best match pre-selected
	m.visible = true
	m.commitHash = hash
	m.commitMsg = msg
}

func (m *Model) Hide() {
	m.visible = false
	m.matches = nil
}

func (m Model) IsVisible() bool {
	return m.visible
}

// Selected returns the selected feature match, or nil if skipped.
func (m Model) Selected() *conductor.FeatureMatch {
	if m.cursor >= 0 && m.cursor < len(m.matches) {
		return &m.matches[m.cursor]
	}
	return nil
}

func (m Model) CommitHash() string {
	return m.commitHash
}

func (m Model) CommitMsg() string {
	return m.commitMsg
}

type ActionKind int

const (
	ActionNone ActionKind = iota
	ActionLink
	ActionSkip
)

type KeyResult struct {
	Action  ActionKind
	Feature *conductor.FeatureMatch
}

func (m *Model) HandleKey(msg tea.KeyMsg) KeyResult {
	if !m.visible {
		return KeyResult{Action: ActionNone}
	}

	s := msg.String()
	switch s {
	case "j", "down":
		if m.cursor < len(m.matches) { // allow going to [skip] entry
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		if m.cursor >= len(m.matches) {
			// [skip] selected
			return KeyResult{Action: ActionSkip}
		}
		return KeyResult{Action: ActionLink, Feature: &m.matches[m.cursor]}
	case "esc", "s":
		return KeyResult{Action: ActionSkip}
	}

	return KeyResult{Action: ActionNone}
}

func (m Model) ViewOverlay(background string, w, h int) string {
	if !m.visible {
		return background
	}

	content := m.renderContent()
	overlay := shared.BranchPickerOverlayStyle.Render(content)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
	)
}

func (m Model) renderContent() string {
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Render("Link commit to feature?")
	b.WriteString(title)
	b.WriteString("\n\n")

	for i, match := range m.matches {
		prefix := "  "
		if i == m.cursor {
			prefix = "\u2192 "
		}

		score := fmt.Sprintf("(%d%%)", int(match.Score*100))
		desc := match.Feature.Description

		// Truncate long descriptions
		maxDesc := 40
		if len(desc) > maxDesc {
			desc = desc[:maxDesc-3] + "..."
		}

		line := prefix + desc + " " + shared.DimFileStyle.Render(score)

		if i == m.cursor {
			line = shared.CursorStyle.Render(line)
		} else {
			line = shared.DimFileStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	// [skip] option
	skipLine := "  [skip]"
	if m.cursor >= len(m.matches) {
		skipLine = "\u2192 [skip]"
		skipLine = shared.CursorStyle.Render(skipLine)
	} else {
		skipLine = shared.DimFileStyle.Render(skipLine)
	}
	b.WriteString(skipLine)
	b.WriteString("\n\n")

	b.WriteString(shared.HelpDescStyle.Render("j/k: navigate  enter: link  esc: skip"))

	return b.String()
}
