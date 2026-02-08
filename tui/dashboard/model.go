package dashboard

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dylan/gitdash/config"
	"github.com/dylan/gitdash/git"
	"github.com/dylan/gitdash/tui/icons"
	"github.com/dylan/gitdash/tui/shared"
)

type ItemKind int

const (
	RepoHeader    ItemKind = iota
	SectionHeader
	DocHeader
	FolderHeader
	File
)

type FlatItem struct {
	Kind      ItemKind
	RepoIndex int
	FileIndex int
	File      *git.FileEntry
	Repo      *git.RepoStatus
	Section   string // "staged", "unstaged", or "docs"
	Tier      int    // 1=bright, 2=normal, 3=dim
	Dir       string // directory path for folder grouping
}

type Model struct {
	repos            []git.RepoStatus
	flatItems        []FlatItem
	repoHeaders      []int // indices into flatItems for repo headers
	collapsed        map[int]bool
	docsCollapsed    map[int]bool
	foldersCollapsed map[string]bool // "repoIndex:dir" -> collapsed
	priorityRules    []config.PriorityRule
	display          config.DisplayConfig
	cursor           int
	width            int
	height           int
}

func New(rules []config.PriorityRule, display config.DisplayConfig) Model {
	return Model{
		collapsed:        make(map[int]bool),
		docsCollapsed:    make(map[int]bool),
		foldersCollapsed: make(map[string]bool),
		priorityRules:    rules,
		display:          display,
	}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *Model) SetRepos(repos []git.RepoStatus) {
	m.repos = repos
	// Auto-collapse repos on first load
	if len(m.collapsed) == 0 {
		for i := range repos {
			m.collapsed[i] = true
		}
	}
	m.rebuildFlatItems()
}

func (m *Model) ToggleCollapse() {
	item, ok := m.SelectedItem()
	if !ok {
		return
	}
	ri := item.RepoIndex
	m.collapsed[ri] = !m.collapsed[ri]
	m.rebuildFlatItems()
}

func (m *Model) ToggleDocsCollapse() {
	item, ok := m.SelectedItem()
	if !ok || item.Kind != DocHeader {
		return
	}
	ri := item.RepoIndex
	m.docsCollapsed[ri] = !m.isDocsCollapsed(ri)
	m.rebuildFlatItems()
}

func (m *Model) ToggleFolderCollapse() {
	item, ok := m.SelectedItem()
	if !ok || item.Kind != FolderHeader {
		return
	}
	key := folderKey(item.RepoIndex, item.Dir)
	m.foldersCollapsed[key] = !m.foldersCollapsed[key]
	m.rebuildFlatItems()
}

func folderKey(repoIndex int, dir string) string {
	return fmt.Sprintf("%d:%s", repoIndex, dir)
}

func (m *Model) isFolderCollapsed(repoIndex int, dir string) bool {
	return m.foldersCollapsed[folderKey(repoIndex, dir)]
}

func (m *Model) IsCollapsed(repoIndex int) bool {
	return m.collapsed[repoIndex]
}

func (m *Model) isDocsCollapsed(repoIndex int) bool {
	collapsed, exists := m.docsCollapsed[repoIndex]
	if !exists {
		return true // collapsed by default
	}
	return collapsed
}

func isDocFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".md")
}

func (m *Model) rebuildFlatItems() {
	m.flatItems = nil
	m.repoHeaders = nil

	for ri := range m.repos {
		repo := &m.repos[ri]

		// Repo header
		m.repoHeaders = append(m.repoHeaders, len(m.flatItems))
		m.flatItems = append(m.flatItems, FlatItem{
			Kind:      RepoHeader,
			RepoIndex: ri,
			Repo:      repo,
		})

		if repo.Error != nil || m.collapsed[ri] {
			continue
		}

		// Collect file indices, optionally separating docs
		var staged, unstaged, docFiles []int
		for fi := range repo.Files {
			if m.display.GroupDocs && isDocFile(repo.Files[fi].Path) {
				docFiles = append(docFiles, fi)
			} else if repo.Files[fi].StagingState == git.Staged {
				staged = append(staged, fi)
			} else {
				unstaged = append(unstaged, fi)
			}
		}

		// Sort each group by dir (if grouping), then tier, then path
		sortFiles := func(indices []int) {
			sort.SliceStable(indices, func(i, j int) bool {
				pi := repo.Files[indices[i]].Path
				pj := repo.Files[indices[j]].Path
				if m.display.GroupFolders {
					di := filepath.Dir(pi)
					dj := filepath.Dir(pj)
					if di != dj {
						return di < dj
					}
				}
				ti := resolveTier(pi, m.priorityRules)
				tj := resolveTier(pj, m.priorityRules)
				if ti != tj {
					return ti < tj
				}
				return pi < pj
			})
		}
		sortFiles(staged)
		sortFiles(unstaged)

		// appendFilesWithFolders adds file items, inserting FolderHeaders when dir changes
		appendFilesWithFolders := func(indices []int, section string) {
			lastDir := ""
			for _, fi := range indices {
				file := &repo.Files[fi]
				dir := filepath.Dir(file.Path)
				if m.display.GroupFolders && dir != "." && dir != lastDir {
					m.flatItems = append(m.flatItems, FlatItem{
						Kind:      FolderHeader,
						RepoIndex: ri,
						Repo:      repo,
						Section:   section,
						Dir:       dir,
					})
					lastDir = dir
				}
				// Skip files under collapsed folder
				if m.display.GroupFolders && dir != "." && m.isFolderCollapsed(ri, dir) {
					continue
				}
				m.flatItems = append(m.flatItems, FlatItem{
					Kind:      File,
					RepoIndex: ri,
					FileIndex: fi,
					File:      file,
					Repo:      repo,
					Section:   section,
					Tier:      resolveTier(file.Path, m.priorityRules),
					Dir:       dir,
				})
			}
		}

		// Staged section
		if len(staged) > 0 {
			m.flatItems = append(m.flatItems, FlatItem{
				Kind:      SectionHeader,
				RepoIndex: ri,
				Repo:      repo,
				Section:   "staged",
			})
			appendFilesWithFolders(staged, "staged")
		}

		// Unstaged section
		if len(unstaged) > 0 {
			m.flatItems = append(m.flatItems, FlatItem{
				Kind:      SectionHeader,
				RepoIndex: ri,
				Repo:      repo,
				Section:   "unstaged",
			})
			appendFilesWithFolders(unstaged, "unstaged")
		}

		// Documents section (collapsible)
		if len(docFiles) > 0 {
			m.flatItems = append(m.flatItems, FlatItem{
				Kind:      DocHeader,
				RepoIndex: ri,
				Repo:      repo,
				Section:   "docs",
			})

			if !m.isDocsCollapsed(ri) {
				// Sort docs by path
				sort.SliceStable(docFiles, func(i, j int) bool {
					return repo.Files[docFiles[i]].Path < repo.Files[docFiles[j]].Path
				})
				for _, fi := range docFiles {
					file := &repo.Files[fi]
					m.flatItems = append(m.flatItems, FlatItem{
						Kind:      File,
						RepoIndex: ri,
						FileIndex: fi,
						File:      file,
						Repo:      repo,
						Section:   "docs",
						Tier:      3,
					})
				}
			}
		}
	}

	// Clamp cursor
	if m.cursor >= len(m.flatItems) {
		m.cursor = len(m.flatItems) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	// If cursor is on a section header, move to next file
	m.skipSectionHeaders(1)
}

// resolveTier determines a file's priority tier from rules. Default is tier 2.
func resolveTier(filePath string, rules []config.PriorityRule) int {
	ext := filepath.Ext(filePath)
	dir := filepath.Dir(filePath)
	parts := strings.Split(dir, string(filepath.Separator))

	for _, rule := range rules {
		extMatch := len(rule.Extensions) == 0
		dirMatch := len(rule.Directories) == 0

		for _, e := range rule.Extensions {
			if ext == e {
				extMatch = true
				break
			}
		}

		for _, d := range rule.Directories {
			for _, p := range parts {
				if p == d {
					dirMatch = true
					break
				}
			}
			if dirMatch {
				break
			}
		}

		if extMatch && dirMatch {
			return rule.Tier
		}
	}
	return 2
}

func isNonSelectable(kind ItemKind) bool {
	return kind == SectionHeader
}

func (m *Model) skipSectionHeaders(dir int) {
	if len(m.flatItems) == 0 {
		return
	}
	for m.cursor >= 0 && m.cursor < len(m.flatItems) && isNonSelectable(m.flatItems[m.cursor].Kind) {
		m.cursor += dir
	}
	if m.cursor >= len(m.flatItems) {
		m.cursor = len(m.flatItems) - 1
		// Try going the other direction
		for m.cursor >= 0 && isNonSelectable(m.flatItems[m.cursor].Kind) {
			m.cursor--
		}
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) MoveDown() {
	if m.cursor < len(m.flatItems)-1 {
		m.cursor++
		m.skipSectionHeaders(1)
	}
}

func (m *Model) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
		m.skipSectionHeaders(-1)
	}
}

func (m *Model) NextRepo() {
	if len(m.repoHeaders) == 0 {
		return
	}
	for _, idx := range m.repoHeaders {
		if idx > m.cursor {
			m.cursor = idx
			return
		}
	}
	// Wrap around
	m.cursor = m.repoHeaders[0]
}

func (m *Model) PrevRepo() {
	if len(m.repoHeaders) == 0 {
		return
	}
	for i := len(m.repoHeaders) - 1; i >= 0; i-- {
		if m.repoHeaders[i] < m.cursor {
			m.cursor = m.repoHeaders[i]
			return
		}
	}
	// Wrap around
	m.cursor = m.repoHeaders[len(m.repoHeaders)-1]
}

func (m Model) SelectedItem() (FlatItem, bool) {
	if m.cursor < 0 || m.cursor >= len(m.flatItems) {
		return FlatItem{}, false
	}
	return m.flatItems[m.cursor], true
}

func (m Model) SelectedRepo() (*git.RepoStatus, bool) {
	item, ok := m.SelectedItem()
	if !ok {
		return nil, false
	}
	return item.Repo, true
}

func (m Model) RepoHasStagedFiles(repoIndex int) bool {
	if repoIndex < 0 || repoIndex >= len(m.repos) {
		return false
	}
	for _, f := range m.repos[repoIndex].Files {
		if f.StagingState == git.Staged {
			return true
		}
	}
	return false
}

func (m Model) View() string {
	if len(m.flatItems) == 0 {
		return "\n  No repos configured or no changes found.\n"
	}

	// Compute visible window
	visibleHeight := m.height - 2 // leave room for status bar
	if visibleHeight < 1 {
		visibleHeight = 20
	}

	scrollOffset := 0
	if m.cursor >= visibleHeight {
		scrollOffset = m.cursor - visibleHeight + 1
	}

	var b strings.Builder
	for i, item := range m.flatItems {
		if i < scrollOffset {
			continue
		}
		if i >= scrollOffset+visibleHeight {
			break
		}

		line := m.renderItem(item)
		if i == m.cursor {
			line = shared.CursorStyle.Width(m.width).Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderItem(item FlatItem) string {
	switch item.Kind {
	case RepoHeader:
		return m.renderRepoHeader(item)
	case SectionHeader:
		return m.renderSectionHeader(item)
	case DocHeader:
		return m.renderDocHeader(item)
	case FolderHeader:
		return m.renderFolderHeader(item)
	case File:
		return m.renderFile(item)
	}
	return ""
}

func (m Model) renderRepoHeader(item FlatItem) string {
	repo := item.Repo
	name := shared.RepoHeaderStyle.Render(repo.Name)
	branch := shared.BranchStyle.Render(repo.Branch)

	chevron := "▼"
	if m.collapsed[item.RepoIndex] {
		chevron = "▶"
	}

	if repo.Error != nil {
		errStr := shared.ErrorStyle.Render(fmt.Sprintf(" (error: %v)", repo.Error))
		return fmt.Sprintf("  %s %s%s", chevron, name, errStr)
	}

	// Build sync badge
	var syncBadge string
	if repo.Ahead > 0 && repo.Behind > 0 {
		syncBadge = shared.SyncPushBadge.Render(fmt.Sprintf("↑ %d to push", repo.Ahead)) +
			" " + shared.SyncPullBadge.Render(fmt.Sprintf("↓ %d to pull", repo.Behind))
	} else if repo.Ahead > 0 {
		syncBadge = shared.SyncPushBadge.Render(fmt.Sprintf("↑ %d to push", repo.Ahead))
	} else if repo.Behind > 0 {
		syncBadge = shared.SyncPullBadge.Render(fmt.Sprintf("↓ %d to pull", repo.Behind))
	}

	fileCount := len(repo.Files)
	var left string
	if fileCount == 0 {
		left = fmt.Sprintf("  %s %s [%s] — clean", chevron, name, branch)
	} else {
		// Count staged vs unstaged
		var stagedCount, unstagedCount int
		for _, f := range repo.Files {
			if f.StagingState == git.Staged {
				stagedCount++
			} else {
				unstagedCount++
			}
		}
		summary := shared.HelpDescStyle.Render(fmt.Sprintf("%d staged, %d unstaged", stagedCount, unstagedCount))
		left = fmt.Sprintf("  %s %s [%s] %s", chevron, name, branch, summary)
	}

	if syncBadge == "" || m.width < 20 {
		return left
	}

	// Right-align the sync badge
	leftW := lipgloss.Width(left)
	badgeW := lipgloss.Width(syncBadge)
	gap := m.width - leftW - badgeW - 1
	if gap < 1 {
		return left + " " + syncBadge
	}
	return left + strings.Repeat(" ", gap) + syncBadge
}

func (m Model) renderSectionHeader(item FlatItem) string {
	if item.Section == "staged" {
		return "    " + shared.StagedSectionStyle.Render("Staged Changes:")
	}
	return "    " + shared.UnstagedSectionStyle.Render("Unstaged Changes:")
}

func (m Model) renderDocHeader(item FlatItem) string {
	// Count .md files for this repo
	count := 0
	for _, file := range item.Repo.Files {
		if isDocFile(file.Path) {
			count++
		}
	}

	chevron := "▼"
	if m.isDocsCollapsed(item.RepoIndex) {
		chevron = "▶"
	}

	label := fmt.Sprintf("Documents (%d)", count)
	return "    " + chevron + " " + shared.DimFileStyle.Render(label)
}

func (m Model) renderFolderHeader(item FlatItem) string {
	dirName := filepath.Base(item.Dir)
	icon := icons.ForDir(dirName)

	chevron := "▼"
	if m.isFolderCollapsed(item.RepoIndex, item.Dir) {
		chevron = "▶"
	}

	style := shared.FolderStyle(dirName)

	return "      " + chevron + " " + style.Render(icon+" "+item.Dir+"/")
}

func (m Model) renderFile(item FlatItem) string {
	file := item.File
	var indicator string
	var style lipgloss.Style

	if file.StagingState == git.Staged {
		indicator = shared.StagedIndicator
		switch item.Tier {
		case 1:
			style = shared.StagedFileStyle
		case 3:
			style = shared.MutedFileStyle
		default:
			style = shared.DimFileStyle
		}
	} else {
		indicator = shared.UnstagedIndicator
		switch item.Tier {
		case 1:
			style = shared.UnstagedFileStyle
		case 3:
			style = shared.MutedFileStyle
		default:
			style = shared.DimFileStyle
		}
	}

	status := fmt.Sprintf("[%s]", file.Status)

	// Show basename when grouped under a folder header
	indent := "      "
	underFolder := m.display.GroupFolders && item.Dir != "." && item.Dir != ""
	if underFolder {
		indent = "        " // extra indent under folder header
	}

	showIcons := m.display.Icons || m.display.NerdFonts
	iconStr := ""
	if showIcons {
		iconStr = style.Render(icons.ForFile(file.Path)) + " "
	}

	// Build the path display
	var pathStr string
	if file.OrigPath != "" {
		if underFolder {
			pathStr = style.Render(filepath.Base(file.OrigPath)) + style.Render(" → ") + style.Render(filepath.Base(file.Path))
		} else if item.Tier == 3 {
			pathStr = style.Render(fmt.Sprintf("%s → %s", file.OrigPath, file.Path))
		} else {
			pathStr = shared.RenderPathWithStyle(file.OrigPath, style) + style.Render(" → ") + shared.RenderPathWithStyle(file.Path, style)
		}
	} else if underFolder {
		// Under folder: just basename with file style
		pathStr = shared.PathFileStyle.Render(filepath.Base(file.Path))
		if item.Tier == 3 {
			pathStr = style.Render(filepath.Base(file.Path))
		}
	} else if item.Tier == 3 {
		// Muted tier overrides path styling
		pathStr = style.Render(file.Path)
	} else {
		pathStr = shared.RenderPathWithStyle(file.Path, style)
	}

	return fmt.Sprintf("%s%s %s%s %s", indent, indicator, iconStr, style.Render(status), pathStr)
}
