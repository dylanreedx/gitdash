package projectmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

type DirEntry struct {
	AbsPath string
	RelPath string
	HasGit  bool
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

	// Dir finder
	configDir    string
	scanRoot     string
	allDirs      []DirEntry
	filteredDirs []DirEntry
	dirCursor    int
	dirScroll    int
	showDirList  bool
}

func New(configDir, scanRoot string) Model {
	ni := textinput.New()
	ni.Placeholder = "project name..."
	ni.CharLimit = 100

	pi := textinput.New()
	pi.Placeholder = "path..."
	pi.CharLimit = 200

	return Model{
		nameInput: ni,
		pathInput: pi,
		configDir: configDir,
		scanRoot:  scanRoot,
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

// listHeight returns how many items fit in the visible area.
func (m Model) listHeight() int {
	h := m.height - 6 // title + footer + padding
	if h < 1 {
		h = 1
	}
	return h
}

func (m *Model) ensureCursorVisible() {
	h := m.listHeight()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+h {
		m.scrollOffset = m.cursor - h + 1
	}
}

// walkDirs recursively collects directories up to maxDepth, skipping hidden dirs.
func walkDirs(root string, maxDepth int) []DirEntry {
	var result []DirEntry
	var walk func(dir string, relPrefix string, depth int)
	walk = func(dir string, relPrefix string, depth int) {
		if depth > maxDepth {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			absPath := filepath.Join(dir, e.Name())
			relPath := filepath.Join(relPrefix, e.Name())
			hasGit := false
			if info, err := os.Stat(filepath.Join(absPath, ".git")); err == nil && info.IsDir() {
				hasGit = true
			}
			result = append(result, DirEntry{
				AbsPath: absPath,
				RelPath: relPath,
				HasGit:  hasGit,
			})
			walk(absPath, relPath, depth+1)
		}
	}
	walk(root, "", 1)
	return result
}

// scanDirs populates allDirs and filteredDirs from root. When preferGit is true,
// git-containing dirs are sorted first.
func (m *Model) scanDirs(root string, preferGit bool) {
	m.allDirs = walkDirs(root, 3)
	if preferGit {
		sort.SliceStable(m.allDirs, func(i, j int) bool {
			if m.allDirs[i].HasGit != m.allDirs[j].HasGit {
				return m.allDirs[i].HasGit
			}
			return false
		})
	}
	m.filteredDirs = m.allDirs
	m.dirCursor = 0
	m.dirScroll = 0
}

// scanRootForMode returns the directory to scan based on the current mode.
func (m *Model) scanRootForMode() string {
	switch m.mode {
	case ModeAddRepo:
		if m.addToProject < len(m.projects) {
			if p := m.projects[m.addToProject].Path; p != "" {
				return p
			}
		}
		return m.scanRoot
	case ModeAddProject, ModeEdit:
		return m.scanRoot
	}
	return m.scanRoot
}

// applyDirFilter filters allDirs by the current pathInput value (case-insensitive substring).
func (m *Model) applyDirFilter() {
	query := strings.ToLower(m.pathInput.Value())
	if query == "" {
		m.filteredDirs = m.allDirs
	} else {
		m.filteredDirs = nil
		for _, d := range m.allDirs {
			if strings.Contains(strings.ToLower(d.RelPath), query) {
				m.filteredDirs = append(m.filteredDirs, d)
			}
		}
	}
	if m.dirCursor >= len(m.filteredDirs) {
		m.dirCursor = max(0, len(m.filteredDirs)-1)
	}
	m.ensureDirCursorVisible()
}

const dirMaxVisible = 8

// ensureDirCursorVisible keeps the dir cursor in the visible scroll window.
func (m *Model) ensureDirCursorVisible() {
	if m.dirCursor < m.dirScroll {
		m.dirScroll = m.dirCursor
	}
	if m.dirCursor >= m.dirScroll+dirMaxVisible {
		m.dirScroll = m.dirCursor - dirMaxVisible + 1
	}
}

// resetDirFinder clears the dir finder state.
func (m *Model) resetDirFinder() {
	m.allDirs = nil
	m.filteredDirs = nil
	m.dirCursor = 0
	m.dirScroll = 0
	m.showDirList = false
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
		// Pre-scan dirs but don't show yet (name field is first)
		m.scanDirs(m.scanRootForMode(), false)
		m.showDirList = false
	case "a":
		if len(m.flatItems) > 0 {
			item := m.flatItems[m.cursor]
			m.addToProject = item.ProjectIndex
			m.mode = ModeAddRepo
			m.pathInput.SetValue("")
			m.pathInput.Focus()
			m.scanDirs(m.scanRootForMode(), true)
			m.showDirList = true
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
				m.scanDirs(m.scanRootForMode(), false)
				m.showDirList = false // name field is first
			} else {
				repo := m.projects[item.ProjectIndex].Repos[item.RepoIndex]
				m.pathInput.SetValue(repo.Path)
				m.activeField = fieldPath
				m.pathInput.Focus()
				m.nameInput.Blur()
				m.scanDirs(m.scanRootForMode(), true)
				m.showDirList = true
				m.applyDirFilter() // filter with pre-filled path
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
	// Dir list navigation when showing and path field is focused
	if m.showDirList && m.activeField == fieldPath {
		switch msg.String() {
		case "ctrl+n", "down":
			if m.dirCursor < len(m.filteredDirs)-1 {
				m.dirCursor++
				m.ensureDirCursorVisible()
			}
			return KeyResult{Action: ActionNone}
		case "ctrl+p", "up":
			if m.dirCursor > 0 {
				m.dirCursor--
				m.ensureDirCursorVisible()
			}
			return KeyResult{Action: ActionNone}
		case "enter":
			if len(m.filteredDirs) > 0 {
				selected := m.filteredDirs[m.dirCursor]
				m.pathInput.SetValue(selected.AbsPath)
				m.showDirList = false
				return KeyResult{Action: ActionNone}
			}
		}
	}

	switch msg.String() {
	case "esc":
		m.mode = ModeBrowse
		m.nameInput.Blur()
		m.pathInput.Blur()
		m.resetDirFinder()
	case "tab":
		if m.activeField == fieldName {
			m.activeField = fieldPath
			m.nameInput.Blur()
			m.pathInput.Focus()
			m.showDirList = true
			m.applyDirFilter()
		} else {
			m.activeField = fieldName
			m.pathInput.Blur()
			m.nameInput.Focus()
			m.showDirList = false
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
		m.resetDirFinder()
		m.rebuildFlatItems()
		m.cursor = len(m.flatItems) - 1
		m.ensureCursorVisible()
	}
	return KeyResult{Action: ActionNone}
}

func (m *Model) handleAddRepoKey(msg tea.KeyMsg) KeyResult {
	// Dir list navigation
	if m.showDirList {
		switch msg.String() {
		case "ctrl+n", "down":
			if m.dirCursor < len(m.filteredDirs)-1 {
				m.dirCursor++
				m.ensureDirCursorVisible()
			}
			return KeyResult{Action: ActionNone}
		case "ctrl+p", "up":
			if m.dirCursor > 0 {
				m.dirCursor--
				m.ensureDirCursorVisible()
			}
			return KeyResult{Action: ActionNone}
		case "enter":
			if len(m.filteredDirs) > 0 {
				selected := m.filteredDirs[m.dirCursor]
				m.pathInput.SetValue(selected.AbsPath)
				m.showDirList = false
				return KeyResult{Action: ActionNone}
			}
		}
	}

	switch msg.String() {
	case "esc":
		m.mode = ModeBrowse
		m.pathInput.Blur()
		m.resetDirFinder()
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
		m.resetDirFinder()
		m.rebuildFlatItems()
		m.ensureCursorVisible()
	}
	return KeyResult{Action: ActionNone}
}

func (m *Model) handleEditKey(msg tea.KeyMsg) KeyResult {
	item := m.flatItems[m.editItem]

	// Dir list navigation when showing and path field is focused
	if m.showDirList && m.activeField == fieldPath {
		switch msg.String() {
		case "ctrl+n", "down":
			if m.dirCursor < len(m.filteredDirs)-1 {
				m.dirCursor++
				m.ensureDirCursorVisible()
			}
			return KeyResult{Action: ActionNone}
		case "ctrl+p", "up":
			if m.dirCursor > 0 {
				m.dirCursor--
				m.ensureDirCursorVisible()
			}
			return KeyResult{Action: ActionNone}
		case "enter":
			if len(m.filteredDirs) > 0 {
				selected := m.filteredDirs[m.dirCursor]
				m.pathInput.SetValue(selected.AbsPath)
				m.showDirList = false
				return KeyResult{Action: ActionNone}
			}
		}
	}

	switch msg.String() {
	case "esc":
		m.mode = ModeBrowse
		m.nameInput.Blur()
		m.pathInput.Blur()
		m.resetDirFinder()
	case "tab":
		if item.Kind == ProjectItem {
			if m.activeField == fieldName {
				m.activeField = fieldPath
				m.nameInput.Blur()
				m.pathInput.Focus()
				m.showDirList = true
				m.applyDirFilter()
			} else {
				m.activeField = fieldName
				m.pathInput.Blur()
				m.nameInput.Focus()
				m.showDirList = false
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
		m.resetDirFinder()
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
	pathUpdated := false
	switch m.mode {
	case ModeAddProject, ModeEdit:
		if m.activeField == fieldName {
			m.nameInput, cmd = m.nameInput.Update(msg)
		} else {
			m.pathInput, cmd = m.pathInput.Update(msg)
			pathUpdated = true
		}
	case ModeAddRepo:
		m.pathInput, cmd = m.pathInput.Update(msg)
		pathUpdated = true
	}
	if pathUpdated && m.showDirList {
		m.applyDirFilter()
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
	dirHint := ""
	if m.showDirList {
		dirHint = "  ctrl+n/p: dirs  "
	}
	switch m.mode {
	case ModeAddProject:
		b.WriteString(shared.HelpDescStyle.Render("tab: switch field" + dirHint + "  enter: create  esc: cancel"))
	case ModeAddRepo:
		b.WriteString(shared.HelpDescStyle.Render(dirHint + "enter: select/add  esc: cancel"))
	case ModeEdit:
		item := m.flatItems[m.editItem]
		if item.Kind == ProjectItem {
			b.WriteString(shared.HelpDescStyle.Render("tab: switch field" + dirHint + "  enter: save  esc: cancel"))
		} else {
			b.WriteString(shared.HelpDescStyle.Render(dirHint + "enter: select/save  esc: cancel"))
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
	visibleH := m.listHeight()

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
	b.WriteString(m.renderDirList())
	if !m.showDirList {
		b.WriteString("\n")
		b.WriteString(shared.HelpDescStyle.Render("  (path is optional — project root for conductor.db)"))
	}

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
	b.WriteString(m.renderDirList())
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

	b.WriteString(m.renderDirList())
	return b.String()
}

func (m Model) renderDirList() string {
	if !m.showDirList || len(m.filteredDirs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")

	end := m.dirScroll + dirMaxVisible
	if end > len(m.filteredDirs) {
		end = len(m.filteredDirs)
	}

	for i := m.dirScroll; i < end; i++ {
		d := m.filteredDirs[i]
		line := "  " + d.RelPath
		if d.HasGit {
			line += " " + shared.BranchStyle.Render("[git]")
		}
		if i == m.dirCursor {
			line = shared.CursorStyle.Render(line)
		} else {
			line = shared.DimFileStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	remaining := len(m.filteredDirs) - end
	if remaining > 0 {
		b.WriteString(shared.HelpDescStyle.Render(fmt.Sprintf("  %d more...", remaining)))
		b.WriteString("\n")
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
