package commitview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dylan/gitdash/conductor"
	"github.com/dylan/gitdash/git"
	"github.com/dylan/gitdash/tui/shared"
)

// conventionalTypes defines the order of type selector badges.
var conventionalTypes = []string{
	"feat", "fix", "refactor", "docs",
	"test", "chore", "perf", "style",
	"ci", "build",
}

type Model struct {
	textArea    textarea.Model
	repo        *git.RepoStatus
	err         error
	generating  bool
	amend       bool
	spinnerView string
	width       int
	height      int

	// Type selector
	selectedType int // index into conventionalTypes, -1 = none

	// Right panel context data
	stagedStats        []git.CommitFileStat
	recentCommits      []git.RecentCommitInfo
	featureSuggestions []conductor.FeatureMatch
}

func New() Model {
	ta := textarea.New()
	ta.Placeholder = "Write your commit message..."
	ta.Prompt = "  "
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // no limit
	ta.SetWidth(72)
	ta.SetHeight(6)

	// Disable ctrl+a (LineStart) so it falls through to AmendToggle
	ta.KeyMap.LineStart.SetEnabled(false)

	return Model{
		textArea:     ta,
		selectedType: -1,
	}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.recalcTextArea()
}

func (m *Model) recalcTextArea() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	var taW int
	if m.width >= 80 {
		// Two-panel: left panel is 55%
		leftW := m.width * 55 / 100
		taW = leftW - 4
	} else {
		taW = m.width - 4
	}
	if taW > 76 {
		taW = 76
	}
	if taW < 20 {
		taW = 20
	}
	m.textArea.SetWidth(taW)

	// overhead: header(2) + type selector(3) + spacing(1) + info bar(1) + help(2) + padding(3) = 12
	overhead := 12
	taH := m.height - overhead
	if taH < 3 {
		taH = 3
	}
	if taH > 12 {
		taH = 12
	}
	m.textArea.SetHeight(taH)
}

func (m *Model) SetRepo(repo *git.RepoStatus) {
	m.repo = repo
	m.err = nil
	m.amend = false
	m.selectedType = -1
	m.stagedStats = nil
	m.recentCommits = nil
	m.featureSuggestions = nil
	m.textArea.Reset()
	m.textArea.Focus()
	if m.width > 0 && m.height > 0 {
		m.recalcTextArea()
	}
}

func (m *Model) SetContextData(stats []git.CommitFileStat, recent []git.RecentCommitInfo, features []conductor.FeatureMatch) {
	m.stagedStats = stats
	m.recentCommits = recent
	m.featureSuggestions = features
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

func (m *Model) SetSpinnerView(view string) {
	m.spinnerView = view
}

func (m *Model) SetAIMessage(msg string) {
	m.textArea.SetValue(msg)
	m.textArea.CursorStart()
	m.detectTypeFromMessage(msg)
}

func (m *Model) ToggleAmend() {
	m.amend = !m.amend
}

func (m *Model) SetAmendMessage(msg string) {
	m.textArea.SetValue(msg)
	m.textArea.CursorStart()
	m.detectTypeFromMessage(msg)
}

func (m Model) IsAmend() bool {
	return m.amend
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.textArea, cmd = m.textArea.Update(msg)
	return m, cmd
}

func (m Model) Value() string {
	return strings.TrimSpace(m.textArea.Value())
}

// CycleTypeForward cycles to the next conventional commit type.
func (m *Model) CycleTypeForward() {
	m.selectedType++
	if m.selectedType >= len(conventionalTypes) {
		m.selectedType = -1 // wrap to "none"
	}
	m.applyTypePrefix()
}

// CycleTypeBackward cycles to the previous conventional commit type.
func (m *Model) CycleTypeBackward() {
	m.selectedType--
	if m.selectedType < -1 {
		m.selectedType = len(conventionalTypes) - 1
	}
	m.applyTypePrefix()
}

// applyTypePrefix rewrites the textarea's first line with the selected type prefix.
func (m *Model) applyTypePrefix() {
	val := m.textArea.Value()
	// Strip existing conventional prefix if present
	stripped := stripConventionalPrefix(val)

	if m.selectedType == -1 {
		// No type selected — just use stripped message
		m.textArea.SetValue(stripped)
	} else {
		typeName := conventionalTypes[m.selectedType]
		if stripped == "" {
			m.textArea.SetValue(typeName + ": ")
		} else {
			m.textArea.SetValue(typeName + ": " + stripped)
		}
	}
	// Move cursor to end of first line
	m.textArea.CursorEnd()
}

// detectTypeFromMessage auto-selects a type badge if the message starts with a conventional prefix.
func (m *Model) detectTypeFromMessage(msg string) {
	lower := strings.ToLower(msg)
	for i, t := range conventionalTypes {
		if strings.HasPrefix(lower, t+":") || strings.HasPrefix(lower, t+"(") {
			m.selectedType = i
			return
		}
	}
	m.selectedType = -1
}

// stripConventionalPrefix removes a leading "type: " or "type(scope): " from a message.
func stripConventionalPrefix(msg string) string {
	lower := strings.ToLower(msg)
	for _, t := range conventionalTypes {
		if strings.HasPrefix(lower, t+":") {
			rest := msg[len(t)+1:]
			return strings.TrimLeft(rest, " ")
		}
		if strings.HasPrefix(lower, t+"(") {
			// Find closing "): "
			end := strings.Index(msg, "):")
			if end != -1 {
				rest := msg[end+2:]
				return strings.TrimLeft(rest, " ")
			}
		}
	}
	return msg
}

func (m Model) countStaged() int {
	if m.repo == nil {
		return 0
	}
	count := 0
	for _, f := range m.repo.Files {
		if f.StagingState == git.Staged {
			count++
		}
	}
	return count
}

func (m Model) View() string {
	if m.width >= 80 {
		return m.renderTwoPanel()
	}
	return m.renderSingleColumn()
}

// renderSingleColumn is the fallback for narrow terminals.
func (m Model) renderSingleColumn() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")
	b.WriteString(m.renderTypeSelector(m.width - 4))
	b.WriteString("\n")
	b.WriteString(m.renderTextAreaOrSpinner())
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString("  " + shared.ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n")
	}

	b.WriteString(m.renderInfoBar())
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

// renderTwoPanel renders the two-panel commit view layout.
func (m Model) renderTwoPanel() string {
	leftW := m.width * 55 / 100
	rightW := m.width - leftW

	left := m.renderLeftPanel(leftW)
	right := m.renderRightPanel(rightW)

	// Ensure both panels are the same height
	h := m.height
	if h < 1 {
		h = 1
	}

	leftStyled := lipgloss.NewStyle().
		Width(leftW).
		Height(h).
		MaxHeight(h).
		Render(left)

	rightStyled := shared.CommitRightBorderStyle.
		Width(rightW - 1). // -1 for border
		Height(h).
		MaxHeight(h).
		Render(right)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, rightStyled)
}

func (m Model) renderLeftPanel(w int) string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")
	b.WriteString(m.renderTypeSelector(w - 4))
	b.WriteString("\n")
	b.WriteString(m.renderTextAreaOrSpinner())
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString("  " + shared.ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n")
	}

	b.WriteString(m.renderInfoBar())
	b.WriteString("\n\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

func (m Model) renderHeader() string {
	if m.repo == nil {
		return ""
	}
	action := "Commit to"
	if m.amend {
		action = "Amend on"
	}
	return shared.CommitHeaderStyle.Render(fmt.Sprintf("  %s: %s [%s]", action, m.repo.Name, m.repo.Branch))
}

func (m Model) renderTypeSelector(maxW int) string {
	var badges []string
	for i, t := range conventionalTypes {
		var badge string
		if i == m.selectedType {
			style, ok := shared.PrefixBadgeStyles[t]
			if !ok {
				style = shared.PrefixBadgeFallback
			}
			badge = style.Render(t)
		} else {
			badge = shared.CommitTypeDimStyle.Render(t)
		}
		badges = append(badges, badge)
	}

	// Flow badges into rows that fit within maxW
	var rows []string
	var currentRow []string
	currentW := 0
	indent := "  "

	for _, badge := range badges {
		bw := lipgloss.Width(badge)
		if currentW > 0 && currentW+bw+1 > maxW {
			rows = append(rows, indent+strings.Join(currentRow, " "))
			currentRow = nil
			currentW = 0
		}
		currentRow = append(currentRow, badge)
		if currentW == 0 {
			currentW = bw
		} else {
			currentW += bw + 1
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, indent+strings.Join(currentRow, " "))
	}

	return strings.Join(rows, "\n")
}

func (m Model) renderTextAreaOrSpinner() string {
	if m.generating {
		spinLabel := "Generating commit message..."
		if m.spinnerView != "" {
			spinLabel = m.spinnerView + " " + spinLabel
		}
		return "  " + shared.HelpDescStyle.Render(spinLabel) + "\n"
	}
	return m.textArea.View()
}

func (m Model) renderInfoBar() string {
	val := m.textArea.Value()
	subjectLine := val
	if idx := strings.IndexByte(val, '\n'); idx >= 0 {
		subjectLine = val[:idx]
	}
	subjectLen := len(subjectLine)

	var lenIndicator string
	switch {
	case subjectLen == 0:
		lenIndicator = shared.HelpDescStyle.Render("0")
	case subjectLen <= 50:
		lenIndicator = shared.CommitFileStyle.Render(fmt.Sprintf("%d", subjectLen))
	case subjectLen <= 72:
		lenIndicator = lipgloss.NewStyle().
			Foreground(shared.FeedbackWarningStyle.GetForeground()).
			Render(fmt.Sprintf("%d", subjectLen))
	default:
		lenIndicator = shared.ErrorStyle.Render(fmt.Sprintf("%d", subjectLen))
	}

	row := m.textArea.Line() + 1
	info := m.textArea.LineInfo()
	col := info.ColumnOffset + 1

	return shared.HelpDescStyle.Render(fmt.Sprintf("  Subject: %s/72  Ln %d, Col %d", lenIndicator, row, col))
}

func (m Model) renderHelp() string {
	amendHint := "C-a: amend"
	if m.amend {
		amendHint = "C-a: new commit"
	}
	return shared.HelpDescStyle.Render(fmt.Sprintf("  C-y: commit  tab: AI  C-t: type  %s  esc: cancel", amendHint))
}

// --- Right Panel ---

func (m Model) renderRightPanel(w int) string {
	var b strings.Builder
	contentW := w - 3 // padding

	b.WriteString("\n")

	// Section 1: Staged files with stats
	b.WriteString(m.renderStagedFilesSection(contentW))

	// Section 2: Recent commits
	b.WriteString(m.renderRecentCommitsSection(contentW))

	// Section 3: Conductor feature suggestions (only if data exists)
	if len(m.featureSuggestions) > 0 {
		b.WriteString(m.renderFeatureSuggestionsSection(contentW))
	}

	return b.String()
}

func (m Model) renderStagedFilesSection(w int) string {
	var b strings.Builder

	// Compute totals
	totalAdd := 0
	totalDel := 0
	for _, s := range m.stagedStats {
		totalAdd += s.Added
		totalDel += s.Deleted
	}

	stagedCount := m.countStaged()
	header := fmt.Sprintf("Staged Files (%d)", stagedCount)
	if totalAdd > 0 || totalDel > 0 {
		stats := ""
		if totalAdd > 0 {
			stats += shared.CommitStatAddStyle.Render(fmt.Sprintf("+%d", totalAdd))
		}
		if totalDel > 0 {
			if stats != "" {
				stats += " "
			}
			stats += shared.CommitStatDelStyle.Render(fmt.Sprintf("-%d", totalDel))
		}
		header += "  " + stats
	}
	b.WriteString(" " + shared.CommitSectionHeaderStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(" " + shared.SectionDividerStyle.Render(strings.Repeat("─", w)))
	b.WriteString("\n")

	if len(m.stagedStats) > 0 {
		maxFiles := 10
		for i, s := range m.stagedStats {
			if i >= maxFiles {
				b.WriteString(shared.HelpDescStyle.Render(fmt.Sprintf("  ... and %d more", len(m.stagedStats)-maxFiles)))
				b.WriteString("\n")
				break
			}
			path := shared.RenderPath(s.Path)
			stats := ""
			if s.Added > 0 {
				stats += shared.CommitStatAddStyle.Render(fmt.Sprintf("+%d", s.Added))
			}
			if s.Deleted > 0 {
				if stats != "" {
					stats += " "
				}
				stats += shared.CommitStatDelStyle.Render(fmt.Sprintf("-%d", s.Deleted))
			}

			line := " " + path
			if stats != "" {
				// Right-align stats
				pathW := lipgloss.Width(line)
				statsW := lipgloss.Width(stats)
				gap := w - pathW - statsW
				if gap < 2 {
					gap = 2
				}
				line += strings.Repeat(" ", gap) + stats
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	} else if stagedCount > 0 {
		// Fallback: show file list from repo without stats
		count := 0
		for _, f := range m.repo.Files {
			if f.StagingState == git.Staged {
				if count >= 10 {
					b.WriteString(shared.HelpDescStyle.Render(fmt.Sprintf("  ... and %d more", stagedCount-10)))
					b.WriteString("\n")
					break
				}
				b.WriteString(" " + shared.RenderPath(f.Path))
				b.WriteString("\n")
				count++
			}
		}
	}
	b.WriteString("\n")
	return b.String()
}

func (m Model) renderRecentCommitsSection(w int) string {
	var b strings.Builder

	b.WriteString(" " + shared.CommitSectionHeaderStyle.Render("Recent Commits"))
	b.WriteString("\n")
	b.WriteString(" " + shared.SectionDividerStyle.Render(strings.Repeat("─", w)))
	b.WriteString("\n")

	if len(m.recentCommits) > 0 {
		maxCommits := 5
		for i, c := range m.recentCommits {
			if i >= maxCommits {
				break
			}
			hash := shared.GraphHashStyle.Render(c.Hash)
			msg := styleCommitMessage(c.Message)
			line := " " + hash + " " + msg

			// Truncate if too wide
			if lipgloss.Width(line) > w {
				// Crude truncation: just render what we have
				line = " " + hash + " " + truncateStyled(c.Message, w-lipgloss.Width(hash)-3)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	} else {
		b.WriteString(shared.HelpDescStyle.Render("  No recent commits"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

func (m Model) renderFeatureSuggestionsSection(w int) string {
	var b strings.Builder

	b.WriteString(" " + shared.CommitSectionHeaderStyle.Render("Conductor Features"))
	b.WriteString("\n")
	b.WriteString(" " + shared.SectionDividerStyle.Render(strings.Repeat("─", w)))
	b.WriteString("\n")

	maxFeatures := 5
	for i, fm := range m.featureSuggestions {
		if i >= maxFeatures {
			break
		}
		score := shared.HelpDescStyle.Render(fmt.Sprintf("(%d%%)", int(fm.Score*100)))
		desc := fm.Feature.Description
		if len(desc) > w-10 {
			desc = desc[:w-13] + "..."
		}
		b.WriteString(fmt.Sprintf(" %s %s", desc, score))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

// styleCommitMessage applies conventional commit badge styling to a message.
func styleCommitMessage(msg string) string {
	lower := strings.ToLower(msg)
	for _, t := range conventionalTypes {
		for _, suffix := range []string{":", "("} {
			prefix := t + suffix
			if strings.HasPrefix(lower, prefix) {
				end := len(prefix)
				if suffix == "(" {
					closeIdx := strings.Index(msg[end:], "):")
					if closeIdx != -1 {
						end = end + closeIdx + 2
					}
				}
				style, ok := shared.PrefixBadgeStyles[t]
				if !ok {
					style = shared.PrefixBadgeFallback
				}
				return style.Render(msg[:end]) + msg[end:]
			}
		}
	}
	return msg
}

// truncateStyled truncates a plain string to fit within maxW visible chars.
func truncateStyled(s string, maxW int) string {
	if maxW <= 3 {
		return "..."
	}
	if len(s) <= maxW {
		return s
	}
	return s[:maxW-3] + "..."
}
