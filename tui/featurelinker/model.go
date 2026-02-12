package featurelinker

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dylan/gitdash/conductor"
	"github.com/dylan/gitdash/tui/shared"
)

type linkerMode int

const (
	modeBrowse linkerMode = iota
	modeSearch
)

type Model struct {
	matches    []conductor.FeatureMatch // scored matches (browse default)
	allItems   []conductor.FeatureMatch // all active features (superset)
	filtered   []conductor.FeatureMatch // currently displayed list
	cursor     int
	scrollOffset int
	visible    bool
	commitHash string
	commitMsg  string
	width      int
	height     int

	mode        linkerMode
	filterInput textinput.Model

	// AI state
	aiPending   bool
	aiSpinner   string
	aiRankedIDs []string

	// Conductor context
	conductorData *conductor.ConductorData
}

func New() Model {
	fi := textinput.New()
	fi.Placeholder = "search features..."
	fi.CharLimit = 100
	return Model{filterInput: fi}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *Model) Show(matches []conductor.FeatureMatch, hash, msg string,
	allFeatures []conductor.Feature, data *conductor.ConductorData) {

	m.matches = matches
	m.cursor = 0
	m.scrollOffset = 0
	m.visible = true
	m.commitHash = hash
	m.commitMsg = msg
	m.mode = modeBrowse
	m.filterInput.SetValue("")
	m.filterInput.Blur()
	m.aiPending = false
	m.aiSpinner = ""
	m.aiRankedIDs = nil
	m.conductorData = data

	// Build allItems: scored matches first, then remaining active features at score 0
	seen := make(map[string]bool)
	m.allItems = make([]conductor.FeatureMatch, len(matches))
	copy(m.allItems, matches)
	for _, fm := range matches {
		seen[fm.Feature.ID] = true
	}
	for _, f := range allFeatures {
		if !seen[f.ID] && (f.Status == "pending" || f.Status == "in_progress" || f.Status == "failed") {
			m.allItems = append(m.allItems, conductor.FeatureMatch{Feature: f, Score: 0})
		}
	}

	// Default display: scored matches only (browse mode)
	m.filtered = matches
}

func (m *Model) Hide() {
	m.visible = false
	m.matches = nil
	m.allItems = nil
	m.filtered = nil
	m.conductorData = nil
}

func (m Model) IsVisible() bool {
	return m.visible
}

func (m Model) InSearchMode() bool {
	return m.mode == modeSearch
}

// Selected returns the selected feature match, or nil if on [skip].
func (m Model) Selected() *conductor.FeatureMatch {
	if m.cursor >= 0 && m.cursor < len(m.filtered) {
		return &m.filtered[m.cursor]
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

	if m.mode == modeSearch {
		return m.handleSearchKey(msg)
	}
	return m.handleBrowseKey(msg)
}

func (m *Model) handleBrowseKey(msg tea.KeyMsg) KeyResult {
	s := msg.String()
	switch s {
	case "j", "down":
		if m.cursor < len(m.filtered) { // allow going to [skip] entry
			m.cursor++
		}
		m.ensureCursorVisible()
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		m.ensureCursorVisible()
	case "enter":
		if m.cursor >= len(m.filtered) {
			return KeyResult{Action: ActionSkip}
		}
		return KeyResult{Action: ActionLink, Feature: &m.filtered[m.cursor]}
	case "esc", "s":
		return KeyResult{Action: ActionSkip}
	case "/":
		m.mode = modeSearch
		m.filterInput.SetValue("")
		m.filterInput.Focus()
		m.filtered = m.allItems
		m.cursor = 0
		m.scrollOffset = 0
	}
	return KeyResult{Action: ActionNone}
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) KeyResult {
	s := msg.String()
	switch s {
	case "esc":
		m.mode = modeBrowse
		m.filterInput.Blur()
		m.filterInput.SetValue("")
		m.filtered = m.matches
		m.cursor = 0
		m.scrollOffset = 0
		return KeyResult{Action: ActionNone}
	case "enter":
		if m.cursor >= len(m.filtered) {
			return KeyResult{Action: ActionSkip}
		}
		return KeyResult{Action: ActionLink, Feature: &m.filtered[m.cursor]}
	case "down":
		if m.cursor < len(m.filtered) {
			m.cursor++
		}
		m.ensureCursorVisible()
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}
		m.ensureCursorVisible()
	}
	return KeyResult{Action: ActionNone}
}

// Update handles textinput updates in search mode.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if m.mode != modeSearch {
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m *Model) applyFilter() {
	query := strings.ToLower(m.filterInput.Value())
	if query == "" {
		m.filtered = m.allItems
		return
	}
	m.filtered = nil
	for _, fm := range m.allItems {
		desc := strings.ToLower(fm.Feature.Description)
		cat := strings.ToLower(fm.Feature.Category)
		id := strings.ToLower(fm.Feature.ID)
		if strings.Contains(desc, query) || strings.Contains(cat, query) || strings.Contains(id, query) {
			m.filtered = append(m.filtered, fm)
		}
	}
	if m.cursor >= len(m.filtered)+1 { // +1 for [skip]
		m.cursor = max(0, len(m.filtered))
	}
	m.scrollOffset = 0
}

// SetAISuggestions integrates AI-ranked feature IDs into the display.
func (m *Model) SetAISuggestions(rankedIDs []string) {
	m.aiRankedIDs = rankedIDs
	m.aiPending = false

	if len(rankedIDs) == 0 {
		return
	}

	// Only re-sort if cursor hasn't moved from initial position
	if m.cursor != 0 {
		return
	}

	// Build rank map
	rankMap := make(map[string]int)
	for i, id := range rankedIDs {
		rankMap[id] = i + 1
	}

	// Apply AI boost to all items
	for i := range m.allItems {
		if rank, ok := rankMap[m.allItems[i].Feature.ID]; ok {
			m.allItems[i].AIRanked = true
			m.allItems[i].AIRank = rank
			// Boost score: higher rank = more boost
			boost := 0.4 / float64(rank)
			m.allItems[i].Score += boost
			if m.allItems[i].Score > 1.0 {
				m.allItems[i].Score = 1.0
			}
		}
	}

	// Apply same to matches
	for i := range m.matches {
		if rank, ok := rankMap[m.matches[i].Feature.ID]; ok {
			m.matches[i].AIRanked = true
			m.matches[i].AIRank = rank
			boost := 0.4 / float64(rank)
			m.matches[i].Score += boost
			if m.matches[i].Score > 1.0 {
				m.matches[i].Score = 1.0
			}
		}
	}

	// Add AI-only features to matches if not already present
	matchIDs := make(map[string]bool)
	for _, fm := range m.matches {
		matchIDs[fm.Feature.ID] = true
	}
	for _, fm := range m.allItems {
		if fm.AIRanked && !matchIDs[fm.Feature.ID] {
			m.matches = append(m.matches, fm)
		}
	}

	// Re-sort matches by score
	sort.Slice(m.matches, func(i, j int) bool {
		return m.matches[i].Score > m.matches[j].Score
	})

	// Update filtered view
	if m.mode == modeBrowse {
		m.filtered = m.matches
	} else {
		m.applyFilter()
	}
	m.cursor = 0
	m.scrollOffset = 0
}

func (m *Model) SetAISpinner(view string) {
	m.aiSpinner = view
}

func (m *Model) SetAIPending(pending bool) {
	m.aiPending = pending
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

func (m Model) maxVisibleItems() int {
	// Reserve lines for: title(1) + blank(1) + [search(2)] + help(1) + blank(1) + skip(1) + detail(5) = ~12
	maxH := m.height - 16
	if maxH < 5 {
		maxH = 5
	}
	if maxH > 15 {
		maxH = 15
	}
	return maxH
}

func (m *Model) ensureCursorVisible() {
	maxVisible := m.maxVisibleItems()
	totalItems := len(m.filtered) + 1 // +1 for [skip]

	if totalItems <= maxVisible {
		m.scrollOffset = 0
		return
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+maxVisible {
		m.scrollOffset = m.cursor - maxVisible + 1
	}
}

func statusIcon(status string) string {
	switch status {
	case "in_progress":
		return "●"
	case "failed":
		return "✗"
	default:
		return "○"
	}
}

func statusIconStyle(status string) lipgloss.Style {
	switch status {
	case "in_progress":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ffaa00"))
	case "failed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555"))
	default:
		return shared.DimFileStyle
	}
}

func (m Model) renderContent() string {
	var b strings.Builder

	// Title with AI status
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Render("Link commit to feature?")
	if m.aiPending {
		title += " " + m.aiSpinner + " " + shared.DimFileStyle.Render("analyzing...")
	} else if len(m.aiRankedIDs) > 0 {
		title += " " + lipgloss.NewStyle().Foreground(lipgloss.Color("#55aaff")).Render("AI")
	}
	b.WriteString(title)
	b.WriteString("\n")

	// Search input (always show line, but only active in search mode)
	if m.mode == modeSearch {
		b.WriteString(m.filterInput.View())
	}
	b.WriteString("\n")

	maxVisible := m.maxVisibleItems()
	totalItems := len(m.filtered) + 1 // +1 for [skip]

	start := m.scrollOffset
	end := start + maxVisible
	if end > totalItems {
		end = totalItems
	}

	if start > 0 {
		b.WriteString(shared.DimFileStyle.Render(fmt.Sprintf("  ↑ %d more", start)))
		b.WriteString("\n")
	}

	for i := start; i < end; i++ {
		if i < len(m.filtered) {
			match := m.filtered[i]
			prefix := "  "
			if i == m.cursor {
				prefix = "→ "
			}

			icon := statusIconStyle(match.Feature.Status).Render(statusIcon(match.Feature.Status))

			desc := match.Feature.Description
			maxDesc := 40
			if len(desc) > maxDesc {
				desc = desc[:maxDesc-3] + "..."
			}

			cat := shared.DimFileStyle.Render("[" + match.Feature.Category + "]")

			var aiTag string
			if match.AIRanked {
				aiTag = " " + lipgloss.NewStyle().Foreground(lipgloss.Color("#55aaff")).Render(fmt.Sprintf("AI#%d", match.AIRank))
			}

			var score string
			if match.Score > 0 {
				score = " " + shared.DimFileStyle.Render(fmt.Sprintf("(%d%%)", int(match.Score*100)))
			}

			line := prefix + icon + " " + desc + " " + cat + aiTag + score

			if i == m.cursor {
				line = shared.CursorStyle.Render(line)
			}

			b.WriteString(line)
			b.WriteString("\n")
		} else {
			// [skip] entry
			skipLine := "  [skip]"
			if m.cursor >= len(m.filtered) {
				skipLine = "→ [skip]"
				skipLine = shared.CursorStyle.Render(skipLine)
			} else {
				skipLine = shared.DimFileStyle.Render(skipLine)
			}
			b.WriteString(skipLine)
			b.WriteString("\n")
		}
	}

	if end < totalItems {
		b.WriteString(shared.DimFileStyle.Render(fmt.Sprintf("  ↓ %d more", totalItems-end)))
		b.WriteString("\n")
	}

	// Detail section for selected feature
	if m.cursor >= 0 && m.cursor < len(m.filtered) {
		b.WriteString("\n")
		b.WriteString(m.renderDetail(m.filtered[m.cursor]))
	}

	b.WriteString("\n")
	if m.mode == modeSearch {
		b.WriteString(shared.HelpDescStyle.Render("↑/↓: navigate  enter: link  esc: back"))
	} else {
		b.WriteString(shared.HelpDescStyle.Render("j/k: navigate  enter: link  /: search  esc: skip"))
	}

	return b.String()
}

func (m Model) renderDetail(fm conductor.FeatureMatch) string {
	var b strings.Builder

	divider := shared.SectionDividerStyle.Render(strings.Repeat("─", 40))
	b.WriteString(divider)
	b.WriteString("\n")

	// Full description
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Render(fm.Feature.Description))
	b.WriteString("\n")

	// Phase + category
	meta := shared.DimFileStyle.Render(fmt.Sprintf("Phase %d · %s · %s", fm.Feature.Phase, fm.Feature.Category, fm.Feature.Status))
	b.WriteString(meta)
	b.WriteString("\n")

	// Related memories from conductor data
	if m.conductorData != nil && len(m.conductorData.Memories) > 0 {
		cat := strings.ToLower(fm.Feature.Category)
		var related []conductor.Memory
		for _, mem := range m.conductorData.Memories {
			for _, tag := range mem.Tags {
				if strings.Contains(strings.ToLower(tag), cat) || strings.Contains(cat, strings.ToLower(tag)) {
					related = append(related, mem)
					break
				}
			}
		}
		if len(related) > 0 {
			if len(related) > 2 {
				related = related[:2]
			}
			for _, mem := range related {
				name := mem.Name
				if len(name) > 30 {
					name = name[:27] + "..."
				}
				b.WriteString(shared.DimFileStyle.Render("  💡 " + name))
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}
