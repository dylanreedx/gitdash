package projectmanager

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dylan/gitdash/config"
	"github.com/dylan/gitdash/tui/shared"
)

type Mode int

const (
	ModeBrowse Mode = iota
	ModeAddProject
	ModeAddRepo
	ModeEdit
	ModeConfirmDelete
)

type ActionKind int

const (
	ActionNone ActionKind = iota
	ActionClose
)

type KeyResult struct {
	Action   ActionKind
	Projects []config.ProjectConfig
	Changed  bool
}

type ItemKind int

const (
	ProjectItem ItemKind = iota
	RepoItem
)

type FlatItem struct {
	Kind         ItemKind
	ProjectIndex int
	RepoIndex    int // -1 for project items
	Label        string
}

type inputField int

const (
	fieldName inputField = iota
	fieldPath
)

type Model struct {
	projects   []config.ProjectConfig
	flatItems  []FlatItem
	cursor     int
	scrollOffset int
	width      int
	height     int
	mode       Mode
	changed    bool

	// Input fields
	nameInput  textinput.Model
	pathInput  textinput.Model
	activeField inputField

	// Context for add/edit/delete
	addToProject int // project index for adding repo
	editItem     int // flat item index being edited
	deleteItem   int // flat item index being deleted
}

func New() Model {
	ni := textinput.New()
	ni.Placeholder = "project name..."
	ni.CharLimit = 100

	pi := textinput.New()
	pi.Placeholder = "path..."
	pi.CharLimit = 200

	return Model{
		nameInput: ni,
		pathInput: pi,
	}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetProjects deep-copies the project list so edits don't mutate the app's live config.
func (m *Model) SetProjects(projects []config.ProjectConfig) {
	m.projects = make([]config.ProjectConfig, len(projects))
	for i, p := range projects {
		m.projects[i] = config.ProjectConfig{
			Name: p.Name,
			Path: p.Path,
		}
		m.projects[i].Repos = make([]config.RepoConfig, len(p.Repos))
		copy(m.projects[i].Repos, p.Repos)
	}
	m.cursor = 0
	m.scrollOffset = 0
	m.mode = ModeBrowse
	m.changed = false
	m.rebuildFlatItems()
}

func (m *Model) rebuildFlatItems() {
	m.flatItems = nil
	for pi, proj := range m.projects {
		m.flatItems = append(m.flatItems, FlatItem{
			Kind:         ProjectItem,
			ProjectIndex: pi,
			RepoIndex:    -1,
			Label:        proj.Name,
		})
		for ri, repo := range proj.Repos {
			m.flatItems = append(m.flatItems, FlatItem{
				Kind:         RepoItem,
				ProjectIndex: pi,
				RepoIndex:    ri,
				Label:        repo.Path,
			})
		}
	}
	if m.cursor >= len(m.flatItems) {
		m.cursor = max(0, len(m.flatItems)-1)
	}
}

func (m *Model) ensureCursorVisible() {
	visibleH := m.height - 6 // title + footer + padding
	if visibleH < 1 {
		visibleH = 1
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visibleH {
		m.scrollOffset = m.cursor - visibleH + 1
	}
}

// InInputMode returns true when a text input is active.
func (m Model) InInputMode() bool {
	return m.mode == ModeAddProject || m.mode == ModeAddRepo || m.mode == ModeEdit
}

// HandleKey processes a key event and returns an action result.
func (m *Model) HandleKey(msg tea.KeyMsg) KeyResult {
	switch m.mode {
	case ModeBrowse:
		return m.handleBrowseKey(msg)
	case ModeAddProject:
		return m.handleAddProjectKey(msg)
	case ModeAddRepo:
		return m.handleAddRepoKey(msg)
	case ModeEdit:
		return m.handleEditKey(msg)
	case ModeConfirmDelete:
		return m.handleDeleteKey(msg)
	}
	return KeyResult{Action: ActionNone}
}

func (m *Model) handleBrowseKey(msg tea.KeyMsg) KeyResult {
	switch msg.String() {
	case "esc", "q", "P":
		return KeyResult{
			Action:   ActionClose,
			Projects: m.projects,
			Changed:  m.changed,
		}
	case "j", "down":
		if m.cursor < len(m.flatItems)-1 {
			m.cursor++
			m.ensureCursorVisible()
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
		}
	case "n":
		m.mode = ModeAddProject
		m.activeField = fieldName
		m.nameInput.SetValue("")
		m.pathInput.SetValue("")
		m.nameInput.Focus()
		m.pathInput.Blur()
	case "a":
		if len(m.flatItems) > 0 {
			item := m.flatItems[m.cursor]
			m.addToProject = item.ProjectIndex
			m.mode = ModeAddRepo
			m.pathInput.SetValue("")
			m.pathInput.Focus()
		}
	case "e":
		if len(m.flatItems) > 0 {
			m.editItem = m.cursor
			item := m.flatItems[m.cursor]
			m.mode = ModeEdit
			if item.Kind == ProjectItem {
				proj := m.projects[item.ProjectIndex]
				m.nameInput.SetValue(proj.Name)
				m.pathInput.SetValue(proj.Path)
				m.activeField = fieldName
				m.nameInput.Focus()
				m.pathInput.Blur()
			} else {
				repo := m.projects[item.ProjectIndex].Repos[item.RepoIndex]
				m.pathInput.SetValue(repo.Path)
				m.activeField = fieldPath
				m.pathInput.Focus()
				m.nameInput.Blur()
			}
		}
	case "x":
		if len(m.flatItems) > 0 {
			m.deleteItem = m.cursor
			m.mode = ModeConfirmDelete
		}
	}
	return KeyResult{Action: ActionNone}
}

func (m *Model) handleAddProjectKey(msg tea.KeyMsg) KeyResult {
	switch msg.String() {
	case "esc":
		m.mode = ModeBrowse
		m.nameInput.Blur()
		m.pathInput.Blur()
	case "tab":
		if m.activeField == fieldName {
			m.activeField = fieldPath
			m.nameInput.Blur()
			m.pathInput.Focus()
		} else {
			m.activeField = fieldName
			m.pathInput.Blur()
			m.nameInput.Focus()
		}
	case "enter":
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return KeyResult{Action: ActionNone}
		}
		proj := config.ProjectConfig{
			Name: name,
			Path: strings.TrimSpace(m.pathInput.Value()),
		}
		m.projects = append(m.projects, proj)
		m.changed = true
		m.mode = ModeBrowse
		m.nameInput.Blur()
		m.pathInput.Blur()
		m.rebuildFlatItems()
		m.cursor = len(m.flatItems) - 1
		m.ensureCursorVisible()
	}
	return KeyResult{Action: ActionNone}
}

func (m *Model) handleAddRepoKey(msg tea.KeyMsg) KeyResult {
	switch msg.String() {
	case "esc":
		m.mode = ModeBrowse
		m.pathInput.Blur()
	case "enter":
		path := strings.TrimSpace(m.pathInput.Value())
		if path == "" {
			return KeyResult{Action: ActionNone}
		}
		m.projects[m.addToProject].Repos = append(
			m.projects[m.addToProject].Repos,
			config.RepoConfig{Path: path},
		)
		m.changed = true
		m.mode = ModeBrowse
		m.pathInput.Blur()
		m.rebuildFlatItems()
		m.ensureCursorVisible()
	}
	return KeyResult{Action: ActionNone}
}

func (m *Model) handleEditKey(msg tea.KeyMsg) KeyResult {
	item := m.flatItems[m.editItem]
	switch msg.String() {
	case "esc":
		m.mode = ModeBrowse
		m.nameInput.Blur()
		m.pathInput.Blur()
	case "tab":
		if item.Kind == ProjectItem {
			if m.activeField == fieldName {
				m.activeField = fieldPath
				m.nameInput.Blur()
				m.pathInput.Focus()
			} else {
				m.activeField = fieldName
				m.pathInput.Blur()
				m.nameInput.Focus()
			}
		}
	case "enter":
		if item.Kind == ProjectItem {
			name := strings.TrimSpace(m.nameInput.Value())
			if name == "" {
				return KeyResult{Action: ActionNone}
			}
			m.projects[item.ProjectIndex].Name = name
			m.projects[item.ProjectIndex].Path = strings.TrimSpace(m.pathInput.Value())
		} else {
			path := strings.TrimSpace(m.pathInput.Value())
			if path == "" {
				return KeyResult{Action: ActionNone}
			}
			m.projects[item.ProjectIndex].Repos[item.RepoIndex].Path = path
		}
		m.changed = true
		m.mode = ModeBrowse
		m.nameInput.Blur()
		m.pathInput.Blur()
		m.rebuildFlatItems()
	}
	return KeyResult{Action: ActionNone}
}

func (m *Model) handleDeleteKey(msg tea.KeyMsg) KeyResult {
	switch msg.String() {
	case "y":
		item := m.flatItems[m.deleteItem]
		if item.Kind == ProjectItem {
			m.projects = append(m.projects[:item.ProjectIndex], m.projects[item.ProjectIndex+1:]...)
		} else {
			repos := &m.projects[item.ProjectIndex].Repos
			*repos = append((*repos)[:item.RepoIndex], (*repos)[item.RepoIndex+1:]...)
		}
		m.changed = true
		m.mode = ModeBrowse
		m.rebuildFlatItems()
		m.ensureCursorVisible()
	case "n", "esc":
		m.mode = ModeBrowse
	}
	return KeyResult{Action: ActionNone}
}

// Update forwards non-key messages to the active textinput.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.InInputMode() {
		return m, nil
	}

	var cmd tea.Cmd
	switch m.mode {
	case ModeAddProject, ModeEdit:
		if m.activeField == fieldName {
			m.nameInput, cmd = m.nameInput.Update(msg)
		} else {
			m.pathInput, cmd = m.pathInput.Update(msg)
		}
	case ModeAddRepo:
		m.pathInput, cmd = m.pathInput.Update(msg)
	}
	return m, cmd
}

// View renders the project manager.
func (m Model) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Render("Project Manager")
	b.WriteString(title)
	b.WriteString("\n\n")

	switch m.mode {
	case ModeAddProject:
		b.WriteString(m.renderAddProject())
	case ModeAddRepo:
		b.WriteString(m.renderAddRepo())
	case ModeEdit:
		b.WriteString(m.renderEdit())
	case ModeConfirmDelete:
		b.WriteString(m.renderBrowse())
		b.WriteString("\n")
		b.WriteString(m.renderDeleteConfirm())
	default:
		b.WriteString(m.renderBrowse())
	}

	b.WriteString("\n\n")

	// Footer help
	switch m.mode {
	case ModeAddProject:
		b.WriteString(shared.HelpDescStyle.Render("tab: switch field  enter: create  esc: cancel"))
	case ModeAddRepo:
		b.WriteString(shared.HelpDescStyle.Render("enter: add  esc: cancel"))
	case ModeEdit:
		item := m.flatItems[m.editItem]
		if item.Kind == ProjectItem {
			b.WriteString(shared.HelpDescStyle.Render("tab: switch field  enter: save  esc: cancel"))
		} else {
			b.WriteString(shared.HelpDescStyle.Render("enter: save  esc: cancel"))
		}
	case ModeConfirmDelete:
		b.WriteString(shared.HelpDescStyle.Render("y: confirm delete  n/esc: cancel"))
	default:
		b.WriteString(shared.HelpDescStyle.Render("j/k: navigate  n: new project  a: add repo  e: edit  x: delete  esc/q/P: close"))
	}

	content := b.String()
	return lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width).
		Height(m.height).
		MaxHeight(m.height).
		Render(content)
}

func (m Model) renderBrowse() string {
	if len(m.flatItems) == 0 {
		return shared.HelpDescStyle.Render("No projects configured. Press n to add one.")
	}

	var b strings.Builder
	visibleH := m.height - 8
	if visibleH < 1 {
		visibleH = 1
	}

	end := m.scrollOffset + visibleH
	if end > len(m.flatItems) {
		end = len(m.flatItems)
	}

	for i := m.scrollOffset; i < end; i++ {
		item := m.flatItems[i]
		line := m.renderItem(item)

		if i == m.cursor {
			line = shared.CursorStyle.Width(m.width - 6).Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderItem(item FlatItem) string {
	switch item.Kind {
	case ProjectItem:
		proj := m.projects[item.ProjectIndex]
		repoCount := len(proj.Repos)
		name := shared.ProjectHeaderStyle.Render(proj.Name)
		count := shared.HelpDescStyle.Render(fmt.Sprintf("(%d repos)", repoCount))
		line := "  " + name + " " + count
		if proj.Path != "" {
			line += "  " + shared.DimFileStyle.Render(proj.Path)
		}
		return line
	case RepoItem:
		return "      " + shared.BranchStyle.Render(item.Label)
	}
	return ""
}

func (m Model) renderAddProject() string {
	var b strings.Builder
	b.WriteString(shared.RepoHeaderStyle.Render("New Project"))
	b.WriteString("\n\n")

	nameLabel := "Name: "
	pathLabel := "Path: "
	if m.activeField == fieldName {
		nameLabel = shared.BranchStyle.Render(nameLabel)
	} else {
		nameLabel = shared.HelpDescStyle.Render(nameLabel)
	}
	if m.activeField == fieldPath {
		pathLabel = shared.BranchStyle.Render(pathLabel)
	} else {
		pathLabel = shared.HelpDescStyle.Render(pathLabel)
	}

	b.WriteString(nameLabel)
	b.WriteString(m.nameInput.View())
	b.WriteString("\n")
	b.WriteString(pathLabel)
	b.WriteString(m.pathInput.View())
	b.WriteString("\n")
	b.WriteString(shared.HelpDescStyle.Render("  (path is optional — project root for conductor.db)"))

	return b.String()
}

func (m Model) renderAddRepo() string {
	var b strings.Builder
	projName := ""
	if m.addToProject < len(m.projects) {
		projName = m.projects[m.addToProject].Name
	}
	b.WriteString(shared.RepoHeaderStyle.Render("Add Repo to " + projName))
	b.WriteString("\n\n")
	b.WriteString(shared.BranchStyle.Render("Path: "))
	b.WriteString(m.pathInput.View())
	return b.String()
}

func (m Model) renderEdit() string {
	var b strings.Builder
	item := m.flatItems[m.editItem]

	if item.Kind == ProjectItem {
		b.WriteString(shared.RepoHeaderStyle.Render("Edit Project"))
		b.WriteString("\n\n")

		nameLabel := "Name: "
		pathLabel := "Path: "
		if m.activeField == fieldName {
			nameLabel = shared.BranchStyle.Render(nameLabel)
		} else {
			nameLabel = shared.HelpDescStyle.Render(nameLabel)
		}
		if m.activeField == fieldPath {
			pathLabel = shared.BranchStyle.Render(pathLabel)
		} else {
			pathLabel = shared.HelpDescStyle.Render(pathLabel)
		}

		b.WriteString(nameLabel)
		b.WriteString(m.nameInput.View())
		b.WriteString("\n")
		b.WriteString(pathLabel)
		b.WriteString(m.pathInput.View())
	} else {
		b.WriteString(shared.RepoHeaderStyle.Render("Edit Repo"))
		b.WriteString("\n\n")
		b.WriteString(shared.BranchStyle.Render("Path: "))
		b.WriteString(m.pathInput.View())
	}

	return b.String()
}

func (m Model) renderDeleteConfirm() string {
	if m.deleteItem >= len(m.flatItems) {
		return ""
	}
	item := m.flatItems[m.deleteItem]
	var target string
	if item.Kind == ProjectItem {
		target = "project \"" + m.projects[item.ProjectIndex].Name + "\" and all its repos"
	} else {
		target = "repo \"" + item.Label + "\""
	}
	return shared.ErrorStyle.Render("Delete " + target + "? (y/n)")
}
