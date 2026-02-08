package commitview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dylan/gitdash/git"
	"github.com/dylan/gitdash/tui/shared"
)

type Model struct {
	textInput   textinput.Model
	repo        *git.RepoStatus
	err         error
	generating  bool
	amend       bool
	spinnerView string
	width       int
	height      int
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter commit message..."
	ti.CharLimit = 200
	ti.Width = 60
	return Model{
		textInput: ti,
	}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.textInput.Width = w - 10
	if m.textInput.Width > 80 {
		m.textInput.Width = 80
	}
}

func (m *Model) SetRepo(repo *git.RepoStatus) {
	m.repo = repo
	m.err = nil
	m.amend = false
	m.textInput.Reset()
	m.textInput.Focus()
}

func (m *Model) SetError(err error) {
	m.err = err
}

func (m *Model) SetGenerating(v bool) {
	m.generating = v
	if !v {
		m.spinnerView = ""
	}
}

// SetSpinnerView sets the rendered spinner string for AI generation animation.
func (m *Model) SetSpinnerView(view string) {
	m.spinnerView = view
}

func (m *Model) SetAIMessage(msg string) {
	m.textInput.SetValue(msg)
	m.textInput.CursorEnd()
}

func (m *Model) ToggleAmend() {
	m.amend = !m.amend
}

func (m *Model) SetAmendMessage(msg string) {
	m.textInput.SetValue(msg)
	m.textInput.CursorEnd()
}

func (m Model) IsAmend() bool {
	return m.amend
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) Value() string {
	return strings.TrimSpace(m.textInput.Value())
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString("\n")
	if m.repo != nil {
		action := "Commit to"
		if m.amend {
			action = "Amend on"
		}
		header := shared.CommitHeaderStyle.Render(fmt.Sprintf("  %s: %s [%s]", action, m.repo.Name, m.repo.Branch))
		b.WriteString(header)
		b.WriteString("\n\n")

		// Show staged files
		b.WriteString("  Staged files:\n")
		for _, f := range m.repo.Files {
			if f.StagingState == git.Staged {
				b.WriteString(fmt.Sprintf("    %s %s\n", shared.StagedIndicator, shared.CommitFileStyle.Render(f.Path)))
			}
		}
		b.WriteString("\n")
	}

	if m.generating {
		spinLabel := "Generating commit message..."
		if m.spinnerView != "" {
			spinLabel = m.spinnerView + " " + spinLabel
		}
		b.WriteString("  " + shared.HelpDescStyle.Render(spinLabel))
		b.WriteString("\n\n")
	} else {
		b.WriteString("  " + m.textInput.View())
		b.WriteString("\n\n")
	}

	if m.err != nil {
		b.WriteString("  " + shared.ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	amendHint := "C-a: amend"
	if m.amend {
		amendHint = "C-a: new commit"
	}
	b.WriteString(shared.HelpDescStyle.Render(fmt.Sprintf("  enter: commit  tab: AI generate  %s  esc: cancel", amendHint)))

	return b.String()
}
