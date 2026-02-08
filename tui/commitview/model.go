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
	textInput  textinput.Model
	repo       *git.RepoStatus
	err        error
	generating bool
	width      int
	height     int
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
	m.textInput.Reset()
	m.textInput.Focus()
}

func (m *Model) SetError(err error) {
	m.err = err
}

func (m *Model) SetGenerating(v bool) {
	m.generating = v
}

func (m *Model) SetAIMessage(msg string) {
	m.textInput.SetValue(msg)
	m.textInput.CursorEnd()
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
		header := shared.CommitHeaderStyle.Render(fmt.Sprintf("  Commit to: %s [%s]", m.repo.Name, m.repo.Branch))
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
		b.WriteString("  " + shared.HelpDescStyle.Render("Generating commit message..."))
		b.WriteString("\n\n")
	} else {
		b.WriteString("  " + m.textInput.View())
		b.WriteString("\n\n")
	}

	if m.err != nil {
		b.WriteString("  " + shared.ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	b.WriteString(shared.HelpDescStyle.Render("  enter: commit  tab: AI generate  esc: cancel"))

	return b.String()
}
