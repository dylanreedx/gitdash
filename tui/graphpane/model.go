package graphpane

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dylan/gitdash/conductor"
	"github.com/dylan/gitdash/git"
	"github.com/dylan/gitdash/tui/icons"
	"github.com/dylan/gitdash/tui/shared"
)

type Section int

const (
	GraphSection Section = iota
	FilesSection
)

type Model struct {
	// Graph viewport (top section)
	graphVP  viewport.Model
	repoPath string
	lines    []git.GraphLine

	// Cached per-line rendered strings (without cursor highlight).
	// Built once in SetGraph, reused on every renderGraph call.
	renderedLines []string

	// Cursor tracking for commit selection
	cursor        int   // index into commitIndices
	commitIndices []int // line indices where IsCommit == true

	// Commit detail (middle section, display-only)
	detail     *git.CommitDetail
	detailHash string

	// Files section (bottom)
	fileCursor   int
	fileExpanded map[string]bool   // path -> expanded
	fileDiffs    map[string]string // path -> cached diff

	// Files viewport
	filesVP viewport.Model

	// Section focus
	activeSection Section

	// Linked features: short hash prefix -> feature description
	linkedFeatures map[string]string

	// Conductor commit context (enriched detail)
	commitContext *conductor.CommitContext

	showIcons bool

	ready  bool
	width  int
	height int
}

// SetShowIcons enables file type icons in the file list.
func (m *Model) SetShowIcons(show bool) {
	m.showIcons = show
}

func New() Model {
	return Model{
		fileExpanded:   make(map[string]bool),
		fileDiffs:      make(map[string]string),
		linkedFeatures: make(map[string]string),
	}
}

// SetLinkedFeatures sets the commit hash -> feature description map for display in commit detail.
func (m *Model) SetLinkedFeatures(lf map[string]string) {
	m.linkedFeatures = lf
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	if h < 1 {
		h = 1
	}
	m.ready = true
	// Width change invalidates cached rendered lines
	if len(m.lines) > 0 {
		m.buildRenderedLines()
	}
	m.rebuildViewports()
}

func (m *Model) rebuildViewports() {
	if !m.ready || m.height < 1 {
		return
	}

	graphH, _, filesH := m.sectionHeights()

	m.graphVP = viewport.New(m.width, graphH)
	m.filesVP = viewport.New(m.width, filesH)

	if len(m.renderedLines) > 0 {
		m.graphVP.SetContent(m.composeGraph())
		m.ensureGraphCursorVisible()
	}
	m.filesVP.SetContent(m.renderFiles())
}

func (m Model) sectionHeights() (graphH, detailH, filesH int) {
	h := m.height
	if m.detail != nil {
		// Reserve 2 lines for newline separators between sections
		usable := h - 2
		if usable < 6 {
			usable = 6
		}
		// 3-section layout: 30% graph, 25% detail, 45% files
		graphH = usable * 30 / 100
		detailH = usable * 25 / 100
		filesH = usable - graphH - detailH
	} else {
		graphH = h
		detailH = 0
		filesH = 0
	}
	if graphH < 3 {
		graphH = 3
	}
	if detailH < 0 {
		detailH = 0
	}
	if filesH < 0 {
		filesH = 0
	}
	return
}

func (m *Model) SetGraph(lines []git.GraphLine, repoPath string) {
	m.lines = lines
	m.repoPath = repoPath
	m.detail = nil
	m.detailHash = ""
	m.fileCursor = 0
	m.fileExpanded = make(map[string]bool)
	m.fileDiffs = make(map[string]string)
	m.activeSection = GraphSection

	// Build commit indices
	m.commitIndices = nil
	for i, l := range lines {
		if l.IsCommit {
			m.commitIndices = append(m.commitIndices, i)
		}
	}
	m.cursor = 0

	// Build cached rendered lines (expensive, done once)
	m.buildRenderedLines()

	if m.ready {
		m.rebuildViewports()
		m.graphVP.GotoTop()
	}
}

// buildRenderedLines pre-renders all graph lines. Called once on SetGraph
// and on width changes in SetSize. This is the expensive operation with
// per-character lipgloss rendering that we want to avoid repeating on j/k.
func (m *Model) buildRenderedLines() {
	m.renderedLines = make([]string, len(m.lines))
	for i, line := range m.lines {
		m.renderedLines[i] = renderLine(line)
	}
}

func (m *Model) SetCommitDetail(detail git.CommitDetail) {
	m.detail = &detail
	m.detailHash = detail.Hash
	m.commitContext = nil // clear stale context
	m.fileCursor = 0
	m.fileExpanded = make(map[string]bool)
	m.fileDiffs = make(map[string]string)
	m.rebuildViewports()
}

// SetCommitContext sets the conductor context for the current commit detail.
func (m *Model) SetCommitContext(ctx *conductor.CommitContext) {
	m.commitContext = ctx
	m.rebuildViewports()
}

func (m *Model) SetFileDiff(path, diff string) {
	m.fileDiffs[path] = diff
	m.filesVP.SetContent(m.renderFiles())
	m.ensureFileCursorVisible()
}

func (m *Model) MoveDown() {
	if len(m.commitIndices) == 0 {
		return
	}
	if m.cursor < len(m.commitIndices)-1 {
		m.cursor++
		m.graphVP.SetContent(m.composeGraph())
		m.ensureGraphCursorVisible()
	}
}

func (m *Model) MoveUp() {
	if len(m.commitIndices) == 0 {
		return
	}
	if m.cursor > 0 {
		m.cursor--
		m.graphVP.SetContent(m.composeGraph())
		m.ensureGraphCursorVisible()
	}
}

func (m *Model) ensureGraphCursorVisible() {
	if len(m.commitIndices) == 0 {
		return
	}
	lineIdx := m.commitIndices[m.cursor]
	graphH, _, _ := m.sectionHeights()
	topLine := m.graphVP.YOffset
	bottomLine := topLine + graphH - 1
	if lineIdx < topLine {
		m.graphVP.SetYOffset(lineIdx)
	} else if lineIdx > bottomLine {
		m.graphVP.SetYOffset(lineIdx - graphH + 1)
	}
}

func (m *Model) FileDown() {
	if m.detail == nil || len(m.detail.Files) == 0 {
		return
	}

	// If current file is expanded and diff extends below viewport, scroll first
	f := m.detail.Files[m.fileCursor]
	if m.fileExpanded[f.Path] {
		endLine := m.fileHeaderLine(m.fileCursor + 1)
		_, _, filesH := m.sectionHeights()
		if m.filesVP.YOffset+filesH < endLine {
			m.filesVP.SetYOffset(m.filesVP.YOffset + 1)
			return
		}
	}

	// Move to next file
	if m.fileCursor < len(m.detail.Files)-1 {
		m.fileCursor++
		m.filesVP.SetContent(m.renderFiles())
		m.ensureFileCursorVisible()
	}
}

func (m *Model) FileUp() {
	if m.detail == nil || len(m.detail.Files) == 0 {
		return
	}

	// If viewport is scrolled past current file's header, scroll up first
	headerLine := m.fileHeaderLine(m.fileCursor)
	if m.filesVP.YOffset > headerLine {
		m.filesVP.SetYOffset(m.filesVP.YOffset - 1)
		return
	}

	// Move to prev file
	if m.fileCursor > 0 {
		m.fileCursor--
		m.filesVP.SetContent(m.renderFiles())
		m.ensureFileCursorVisible()
	}
}

// fileHeaderLine returns the line index where file at idx starts in the rendered content.
// If idx >= len(files), returns the total line count.
func (m Model) fileHeaderLine(idx int) int {
	line := 0
	for i := 0; i < idx && i < len(m.detail.Files); i++ {
		line++ // header
		f := m.detail.Files[i]
		if m.fileExpanded[f.Path] {
			if diff, ok := m.fileDiffs[f.Path]; ok && diff != "" {
				line += strings.Count(diff, "\n") + 1
			} else {
				line++ // "Loading..." or "(no changes)"
			}
		}
	}
	return line
}

func (m *Model) ToggleFileExpand() string {
	if m.detail == nil || len(m.detail.Files) == 0 {
		return ""
	}
	path := m.detail.Files[m.fileCursor].Path
	if m.fileExpanded[path] {
		m.fileExpanded[path] = false
		m.filesVP.SetContent(m.renderFiles())
		m.ensureFileCursorVisible()
		return ""
	}
	m.fileExpanded[path] = true
	if _, cached := m.fileDiffs[path]; cached {
		m.filesVP.SetContent(m.renderFiles())
		m.ensureFileCursorVisible()
		return ""
	}
	// Need to fetch — caller should issue command
	m.filesVP.SetContent(m.renderFiles())
	m.ensureFileCursorVisible()
	return path
}

// ensureFileCursorVisible computes the target line by counting file headers
// and expanded diff lines, then adjusts the files viewport offset.
func (m *Model) ensureFileCursorVisible() {
	if m.detail == nil || len(m.detail.Files) == 0 {
		return
	}
	targetLine := 0
	for i := 0; i < m.fileCursor; i++ {
		targetLine++ // file header line
		f := m.detail.Files[i]
		if m.fileExpanded[f.Path] {
			if diff, ok := m.fileDiffs[f.Path]; ok && diff != "" {
				targetLine += strings.Count(diff, "\n") + 1
			} else {
				targetLine++ // "Loading..." or "(no changes)"
			}
		}
	}
	_, _, filesH := m.sectionHeights()
	topLine := m.filesVP.YOffset
	bottomLine := topLine + filesH - 1
	if targetLine < topLine {
		m.filesVP.SetYOffset(targetLine)
	} else if targetLine > bottomLine {
		m.filesVP.SetYOffset(targetLine - filesH + 1)
	}
}

func (m Model) SelectedHash() string {
	if len(m.commitIndices) == 0 {
		return ""
	}
	lineIdx := m.commitIndices[m.cursor]
	if lineIdx < len(m.lines) {
		return m.lines[lineIdx].Hash
	}
	return ""
}

func (m Model) ActiveSection() Section {
	return m.activeSection
}

func (m Model) RepoPath() string {
	return m.repoPath
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.activeSection {
		case GraphSection:
			switch {
			case key.Matches(msg, shared.Keys.Down):
				m.MoveDown()
				return m, nil
			case key.Matches(msg, shared.Keys.Up):
				m.MoveUp()
				return m, nil
			case key.Matches(msg, shared.Keys.Open), key.Matches(msg, shared.Keys.FocusDown):
				if m.detail != nil && len(m.detail.Files) > 0 {
					m.activeSection = FilesSection
					m.filesVP.SetContent(m.renderFiles())
				}
				return m, nil
			}
		case FilesSection:
			switch {
			case key.Matches(msg, shared.Keys.Down):
				m.FileDown()
				return m, nil
			case key.Matches(msg, shared.Keys.Up):
				m.FileUp()
				return m, nil
			case key.Matches(msg, shared.Keys.FocusUp), key.Matches(msg, shared.Keys.Escape):
				m.activeSection = GraphSection
				return m, nil
			case key.Matches(msg, shared.Keys.Open):
				path := m.ToggleFileExpand()
				if path != "" {
					hash := m.detailHash
					repoPath := m.repoPath
					return m, func() tea.Msg {
						diff, err := git.GetCommitFileDiff(repoPath, hash, path)
						return shared.CommitFileDiffFetchedMsg{
							FilePath: path,
							Diff:     diff,
							Hash:     hash,
							Err:      err,
						}
					}
				}
				return m, nil
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	return m.view(false)
}

func (m Model) ViewFocused() string {
	return m.view(true)
}

func (m Model) view(focused bool) string {
	if !m.ready {
		return ""
	}
	style := shared.GraphBorderStyle
	if focused {
		style = shared.GraphBorderFocusedStyle
	}

	if m.detail == nil {
		return style.Width(m.width).Height(m.height).Render(m.graphVP.View())
	}

	graphH, detailH, filesH := m.sectionHeights()

	// Each section is fixed-height to prevent layout shifts
	graphView := fixedHeight(m.graphVP.View(), graphH)
	detailView := fixedHeight(m.renderDetail(), detailH)
	filesView := fixedHeight(m.filesVP.View(), filesH)

	content := graphView + "\n" + detailView + "\n" + filesView

	return style.Width(m.width).Height(m.height).Render(content)
}

// --- Graph rendering ---

// conventionalPrefixes are highlighted in commit messages.
var conventionalPrefixes = []string{
	"feat:", "fix:", "chore:", "refactor:", "docs:", "test:", "style:", "perf:", "ci:", "build:",
	"feat(", "fix(", "chore(", "refactor(", "docs(", "test(", "style(", "perf(", "ci(", "build(",
}

// composeGraph assembles the graph content from the cached rendered lines,
// applying cursor highlight to the selected commit. This is fast because
// the expensive per-character lipgloss rendering was done once in buildRenderedLines.
func (m Model) composeGraph() string {
	if len(m.renderedLines) == 0 {
		return "  No commits"
	}

	cursorLineIdx := -1
	if len(m.commitIndices) > 0 && m.cursor < len(m.commitIndices) {
		cursorLineIdx = m.commitIndices[m.cursor]
	}

	var b strings.Builder
	for i, rendered := range m.renderedLines {
		if i == cursorLineIdx {
			b.WriteString(shared.CursorStyle.Width(m.width).Render(rendered))
		} else {
			b.WriteString(rendered)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// renderLine renders a single graph line with styling. Called once per line
// during buildRenderedLines, not on every cursor move.
func renderLine(line git.GraphLine) string {
	var b strings.Builder

	b.WriteString(colorGraphChars(line.GraphChars))

	if !line.IsCommit {
		return b.String()
	}

	if line.Hash != "" {
		hash := line.Hash
		if len(hash) > 7 {
			hash = hash[:7]
		}
		b.WriteString(shared.GraphHashStyle.Render(hash))
		b.WriteString(" ")
	}

	if line.Refs != "" {
		b.WriteString(shared.GraphRefStyle.Render(line.Refs))
		b.WriteString(" ")
	}

	b.WriteString(shared.CommitDetailMsgStyle.Render(styleMessage(line.Message)))

	return b.String()
}

// --- Commit detail rendering ---

func (m Model) renderDetail() string {
	if m.detail == nil {
		return ""
	}
	d := m.detail

	divider := shared.SectionDividerStyle.Render(strings.Repeat("─", m.width))
	label := shared.CommitDetailLabelStyle

	var b strings.Builder
	b.WriteString(divider)
	b.WriteString("\n")

	// Breathing room
	b.WriteString("\n")

	// Aligned labels: commit / author / date
	b.WriteString("  ")
	b.WriteString(label.Render("commit"))
	b.WriteString("  ")
	b.WriteString(shared.CommitDetailHashStyle.Render(d.Hash[:min(12, len(d.Hash))]))
	b.WriteString("\n")

	b.WriteString("  ")
	b.WriteString(label.Render("author"))
	b.WriteString("  ")
	b.WriteString(shared.CommitDetailAuthorStyle.Render(d.Author))
	b.WriteString("\n")

	b.WriteString("  ")
	b.WriteString(label.Render("date  "))
	b.WriteString("  ")
	date := d.Date
	if len(date) > 10 {
		date = date[:10]
	}
	b.WriteString(shared.CommitDetailDateStyle.Render(date))
	b.WriteString("\n")

	// Separator
	b.WriteString("\n")

	// Message (truncate to 3 lines, style with conventional prefix highlighting)
	msgLines := strings.Split(strings.TrimSpace(d.Message), "\n")
	maxLines := 3
	if len(msgLines) > maxLines {
		msgLines = msgLines[:maxLines]
	}
	for _, ml := range msgLines {
		b.WriteString("  ")
		b.WriteString(styleMessage(ml))
		b.WriteString("\n")
	}

	// Badge-style stats
	if d.TotalAdd > 0 || d.TotalDel > 0 {
		b.WriteString("  ")
		b.WriteString(shared.StatAddBadge.Render(fmt.Sprintf("+%d", d.TotalAdd)))
		b.WriteString(" ")
		b.WriteString(shared.StatDelBadge.Render(fmt.Sprintf("-%d", d.TotalDel)))
		b.WriteString("  ")
		b.WriteString(shared.CommitDetailDateStyle.Render(fmt.Sprintf("%d files", len(d.Files))))
		b.WriteString("\n")
	}

	// Conductor context block
	if m.commitContext != nil {
		b.WriteString(m.renderCommitContext())
	} else if desc := m.findLinkedFeature(d.Hash); desc != "" {
		// Fallback: simple linked feature badge
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(shared.ConductorPassedBadge.Render("feat"))
		b.WriteString("   ")
		b.WriteString(shared.CommitDetailMsgStyle.Render(desc))
		b.WriteString("\n")
	}

	b.WriteString(divider)

	return b.String()
}

// renderCommitContext renders the conductor context block within commit detail.
func (m Model) renderCommitContext() string {
	ctx := m.commitContext
	if ctx == nil {
		return ""
	}

	label := shared.CommitDetailLabelStyle
	var b strings.Builder

	b.WriteString("\n")

	// Session
	if ctx.Session != nil {
		b.WriteString("  ")
		b.WriteString(label.Render("session"))
		b.WriteString(" ")
		b.WriteString(shared.CommitDetailHashStyle.Render(fmt.Sprintf("#%d", ctx.Session.Number)))
		b.WriteString("\n")
	}

	// Feature
	if ctx.Feature != nil {
		indicator := "○" // pending
		switch ctx.Feature.Status {
		case "passed":
			indicator = shared.StagedFileStyle.Render("✓")
		case "in_progress":
			indicator = shared.UnstagedFileStyle.Render("●")
		case "failed":
			indicator = shared.ErrorStyle.Render("✗")
		case "blocked":
			indicator = shared.DimFileStyle.Render("⊘")
		}
		desc := ctx.Feature.Description
		maxLen := m.width - 14 // account for label + indicator + padding
		if maxLen > 0 && len(desc) > maxLen {
			desc = desc[:maxLen-1] + "…"
		}
		b.WriteString("  ")
		b.WriteString(label.Render("feature"))
		b.WriteString(" ")
		b.WriteString(indicator)
		b.WriteString(" ")
		b.WriteString(shared.CommitDetailMsgStyle.Render(desc))
		b.WriteString("\n")
	}

	// Errors
	if len(ctx.Errors) > 0 {
		b.WriteString("  ")
		b.WriteString(label.Render("errors "))
		b.WriteString(" ")
		b.WriteString(shared.CommitDetailDateStyle.Render(fmt.Sprintf("%d resolved", len(ctx.Errors))))
		b.WriteString("\n")

		for i, fe := range ctx.Errors {
			errMsg := fe.Error
			maxLen := m.width - 16
			if maxLen > 0 && len(errMsg) > maxLen {
				errMsg = errMsg[:maxLen-1] + "…"
			}
			// Last error gets ✓ if feature is passed
			if i == len(ctx.Errors)-1 && ctx.Feature != nil && ctx.Feature.Status == "passed" {
				b.WriteString("    ")
				b.WriteString(shared.StagedFileStyle.Render(fmt.Sprintf("✓ [%d]", fe.AttemptNumber)))
				b.WriteString(" ")
				b.WriteString(shared.DimFileStyle.Render(errMsg))
				b.WriteString("\n")
			} else {
				b.WriteString("    ")
				b.WriteString(shared.ErrorStyle.Render(fmt.Sprintf("✗ [%d]", fe.AttemptNumber)))
				b.WriteString(" ")
				b.WriteString(shared.DimFileStyle.Render(errMsg))
				b.WriteString("\n")
			}
		}
	}

	// Memories
	if len(ctx.Memories) > 0 {
		b.WriteString("  ")
		b.WriteString(label.Render("memory "))
		b.WriteString(" ")
		var names []string
		for _, mem := range ctx.Memories {
			names = append(names, mem.Name)
		}
		b.WriteString(shared.DimFileStyle.Render(strings.Join(names, ", ")))
		b.WriteString("\n")
	}

	return b.String()
}

// --- Files rendering ---

func (m Model) renderFiles() string {
	if m.detail == nil || len(m.detail.Files) == 0 {
		return ""
	}

	var b strings.Builder
	for i, f := range m.detail.Files {
		expanded := m.fileExpanded[f.Path]
		chevron := "▶"
		if expanded {
			chevron = "▼"
		}

		stats := ""
		if f.Added > 0 || f.Deleted > 0 {
			stats = " " + shared.StatAddBadge.Render(fmt.Sprintf("+%d", f.Added)) +
				" " + shared.StatDelBadge.Render(fmt.Sprintf("-%d", f.Deleted))
		}

		icon := ""
		if m.showIcons {
			icon = icons.ForFile(f.Path) + " "
		}

		line := fmt.Sprintf("  %s %s%s%s", chevron, icon, shared.RenderPath(f.Path), stats)

		if i == m.fileCursor && m.activeSection == FilesSection {
			line = shared.CursorStyle.Width(m.width).Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")

		if expanded {
			if diff, ok := m.fileDiffs[f.Path]; ok && diff != "" {
				b.WriteString(styleDiff(diff))
			} else if _, ok := m.fileDiffs[f.Path]; ok {
				b.WriteString("    (no changes)\n")
			} else {
				b.WriteString("    Loading...\n")
			}
		}
	}
	return b.String()
}

// findLinkedFeature checks if the given commit hash matches any linked feature.
// It tries both direct lookup and prefix matching.
func (m Model) findLinkedFeature(hash string) string {
	if len(m.linkedFeatures) == 0 || hash == "" {
		return ""
	}
	// Direct match
	if desc, ok := m.linkedFeatures[hash]; ok {
		return desc
	}
	// Prefix match: linked features may store short hashes
	for prefix, desc := range m.linkedFeatures {
		if prefix != "" && strings.HasPrefix(hash, prefix) {
			return desc
		}
	}
	// Reverse prefix: hash may be shorter than stored key
	for key, desc := range m.linkedFeatures {
		if key != "" && strings.HasPrefix(key, hash) {
			return desc
		}
	}
	return ""
}

// --- Helpers ---

// fixedHeight ensures a string has exactly h lines, truncating or padding as needed.
func fixedHeight(s string, h int) string {
	if h <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func styleDiff(raw string) string {
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		prefix := "    "
		switch {
		case strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- "):
			b.WriteString(prefix + shared.DiffMetaStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			b.WriteString(prefix + shared.DiffHunkStyle.Render(line))
		case strings.HasPrefix(line, "+"):
			b.WriteString(prefix + shared.DiffAddStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			b.WriteString(prefix + shared.DiffRemoveStyle.Render(line))
		case strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index "):
			b.WriteString(prefix + shared.DiffMetaStyle.Render(line))
		default:
			b.WriteString(prefix + line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func colorGraphChars(chars string) string {
	if len(shared.GraphLineColors) == 0 {
		return chars
	}

	var b strings.Builder
	col := 0
	for _, ch := range chars {
		switch ch {
		case ' ':
			b.WriteRune(ch)
			col++
		case '*':
			style := shared.GraphLineColors[col%len(shared.GraphLineColors)]
			b.WriteString(style.Render("●"))
			col++
		case '|', '/', '\\':
			style := shared.GraphLineColors[col%len(shared.GraphLineColors)]
			b.WriteString(style.Render(string(ch)))
			col++
		default:
			style := shared.GraphLineColors[col%len(shared.GraphLineColors)]
			b.WriteString(style.Render(string(ch)))
		}
	}
	return b.String()
}

func styleMessage(msg string) string {
	lower := strings.ToLower(msg)
	for _, prefix := range conventionalPrefixes {
		if strings.HasPrefix(lower, prefix) {
			end := len(prefix)
			if strings.HasSuffix(prefix, "(") {
				closeIdx := strings.Index(msg[end:], "):")
				if closeIdx != -1 {
					end = end + closeIdx + 2
				}
			}
			// Extract base type (e.g. "feat" from "feat:" or "feat(")
			baseType := strings.TrimRight(prefix, ":(")
			style, ok := shared.PrefixBadgeStyles[baseType]
			if !ok {
				style = shared.PrefixBadgeFallback
			}
			return style.Render(msg[:end]) + lipgloss.NewStyle().Render(msg[end:])
		}
	}
	return msg
}
