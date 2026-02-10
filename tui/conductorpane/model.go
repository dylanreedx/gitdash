package conductorpane

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dylan/gitdash/conductor"
	"github.com/dylan/gitdash/tui/shared"
)

type Section int

const (
	ListSection Section = iota
	DetailSection
)

type ItemKind int

const (
	SectionSpacer ItemKind = iota
	FeatureHeader
	FeatureItem
	SessionHeader
	HandoffItem
	QualityHeader
	QualityItem
	MemoryHeader
	MemoryItem
)

type FlatItem struct {
	Kind    ItemKind
	Feature *conductor.Feature
	Session *conductor.Session
	Handoff *conductor.Handoff
	Quality *conductor.QualityReflection
	Memory  *conductor.Memory
	Label   string // suffix text for headers, or pre-built label for handoff/quality lines
}

type Model struct {
	flatItems    []FlatItem
	collapsed    map[ItemKind]bool
	cursor       int
	scrollOffset int
	width        int
	height       int

	activeSection Section
	detailVP      viewport.Model

	data         *conductor.ConductorData
	hasConductor bool
}

func New() Model {
	return Model{
		collapsed: map[ItemKind]bool{
			MemoryHeader: true, // memories collapsed by default
		},
	}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.updateDetailContent()
}

func (m *Model) SetData(data *conductor.ConductorData) {
	m.data = data
	m.hasConductor = data != nil
	m.rebuildFlatItems()
	m.updateDetailContent()
}

func (m *Model) HasConductor() bool {
	return m.hasConductor
}

func (m *Model) ToggleCollapse() {
	if len(m.flatItems) == 0 || m.cursor < 0 || m.cursor >= len(m.flatItems) {
		return
	}
	item := m.flatItems[m.cursor]
	switch item.Kind {
	case FeatureHeader, SessionHeader, QualityHeader, MemoryHeader:
		m.collapsed[item.Kind] = !m.collapsed[item.Kind]
		m.rebuildFlatItems()
	}
}

func (m *Model) MoveDown() {
	if len(m.flatItems) == 0 {
		return
	}
	m.cursor++
	if m.cursor >= len(m.flatItems) {
		m.cursor = len(m.flatItems) - 1
	}
	m.skipNonSelectable(1)
	m.ensureCursorVisible()
}

func (m *Model) MoveUp() {
	if len(m.flatItems) == 0 {
		return
	}
	m.cursor--
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.skipNonSelectable(-1)
	m.ensureCursorVisible()
}

func (m *Model) skipNonSelectable(dir int) {
	for m.cursor >= 0 && m.cursor < len(m.flatItems) {
		if m.flatItems[m.cursor].Kind != SectionSpacer {
			return
		}
		m.cursor += dir
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.flatItems) {
		m.cursor = len(m.flatItems) - 1
	}
}

// listHeight returns how many lines the list section gets.
func (m Model) listHeight() int {
	h := m.height
	if h > 15 {
		detailH := h * 35 / 100
		if detailH < 6 {
			detailH = 6
		}
		return h - detailH - 1 // -1 for divider
	}
	return h
}

func (m *Model) ensureCursorVisible() {
	listH := m.listHeight()
	if listH < 1 {
		listH = 1
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+listH {
		m.scrollOffset = m.cursor - listH + 1
	}
}

func (m Model) ActiveSection() Section {
	return m.activeSection
}

// isHeader returns true if the item kind is a section header.
func isHeader(k ItemKind) bool {
	return k == FeatureHeader || k == SessionHeader || k == QualityHeader || k == MemoryHeader
}

// NextSection jumps the cursor to the next section header.
func (m *Model) NextSection() {
	for i := m.cursor + 1; i < len(m.flatItems); i++ {
		if isHeader(m.flatItems[i].Kind) {
			m.cursor = i
			m.ensureCursorVisible()
			return
		}
	}
	// Wrap to first header
	for i := 0; i < m.cursor; i++ {
		if isHeader(m.flatItems[i].Kind) {
			m.cursor = i
			m.ensureCursorVisible()
			return
		}
	}
}

// PrevSection jumps the cursor to the previous section header.
func (m *Model) PrevSection() {
	for i := m.cursor - 1; i >= 0; i-- {
		if isHeader(m.flatItems[i].Kind) {
			m.cursor = i
			m.ensureCursorVisible()
			return
		}
	}
	// Wrap to last header
	for i := len(m.flatItems) - 1; i > m.cursor; i-- {
		if isHeader(m.flatItems[i].Kind) {
			m.cursor = i
			m.ensureCursorVisible()
			return
		}
	}
}

// updateDetailContent renders the detail for the current cursor item into the viewport.
func (m *Model) updateDetailContent() {
	if m.height <= 15 {
		return
	}
	_, detailH := m.sectionSplit()
	content := m.renderDetail(m.width, detailH)
	m.detailVP = viewport.New(m.width, detailH)
	m.detailVP.SetContent(content)
}

// sectionSplit returns listH, detailH for the current height.
func (m Model) sectionSplit() (int, int) {
	h := m.height
	if h <= 15 {
		return h, 0
	}
	detailH := h * 35 / 100
	if detailH < 6 {
		detailH = 6
	}
	return h - detailH - 1, detailH
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.activeSection {
		case ListSection:
			switch {
			case key.Matches(msg, shared.Keys.Down):
				m.MoveDown()
				m.updateDetailContent()
				return m, nil
			case key.Matches(msg, shared.Keys.Up):
				m.MoveUp()
				m.updateDetailContent()
				return m, nil
			case key.Matches(msg, shared.Keys.Open):
				if len(m.flatItems) > 0 && m.cursor >= 0 && m.cursor < len(m.flatItems) {
					item := m.flatItems[m.cursor]
					if isHeader(item.Kind) {
						m.ToggleCollapse()
						m.updateDetailContent()
					} else if item.Kind != SectionSpacer {
						m.activeSection = DetailSection
						m.updateDetailContent()
					}
				}
				return m, nil
			case key.Matches(msg, shared.Keys.FocusDown):
				if m.height > 15 {
					m.activeSection = DetailSection
					m.updateDetailContent()
				}
				return m, nil
			case key.Matches(msg, shared.Keys.NextRepo):
				m.NextSection()
				m.updateDetailContent()
				return m, nil
			case key.Matches(msg, shared.Keys.PrevRepo):
				m.PrevSection()
				m.updateDetailContent()
				return m, nil
			}
		case DetailSection:
			switch {
			case key.Matches(msg, shared.Keys.Down):
				m.detailVP.LineDown(1)
				return m, nil
			case key.Matches(msg, shared.Keys.Up):
				m.detailVP.LineUp(1)
				return m, nil
			case key.Matches(msg, shared.Keys.FocusUp), key.Matches(msg, shared.Keys.Escape):
				m.activeSection = ListSection
				return m, nil
			}
		}
	}
	return m, nil
}

func (m *Model) rebuildFlatItems() {
	m.flatItems = nil
	if m.data == nil {
		return
	}

	// Features section
	m.flatItems = append(m.flatItems, FlatItem{
		Kind:  FeatureHeader,
		Label: fmt.Sprintf("%d/%d passed", m.data.Passed, m.data.Total),
	})
	if !m.collapsed[FeatureHeader] {
		// Show non-passed features first (active/failed/blocked/pending), then passed
		for i := range m.data.Features {
			if m.data.Features[i].Status != "passed" {
				m.flatItems = append(m.flatItems, FlatItem{
					Kind:    FeatureItem,
					Feature: &m.data.Features[i],
				})
			}
		}
		for i := range m.data.Features {
			if m.data.Features[i].Status == "passed" {
				m.flatItems = append(m.flatItems, FlatItem{
					Kind:    FeatureItem,
					Feature: &m.data.Features[i],
				})
			}
		}
	}

	// Session section
	if m.data.Session != nil {
		m.flatItems = append(m.flatItems, FlatItem{Kind: SectionSpacer})
		s := m.data.Session
		m.flatItems = append(m.flatItems, FlatItem{
			Kind:    SessionHeader,
			Session: s,
			Label:   s.Status,
		})
		if !m.collapsed[SessionHeader] && m.data.Handoff != nil {
			h := m.data.Handoff
			if h.CurrentTask != "" {
				m.flatItems = append(m.flatItems, FlatItem{
					Kind:    HandoffItem,
					Handoff: h,
					Label:   "Task:  " + h.CurrentTask,
				})
			}
			if len(h.NextSteps) > 0 {
				for _, step := range h.NextSteps {
					m.flatItems = append(m.flatItems, FlatItem{
						Kind:    HandoffItem,
						Handoff: h,
						Label:   "Next:  " + step,
					})
				}
			}
			if len(h.Blockers) > 0 {
				for _, blocker := range h.Blockers {
					m.flatItems = append(m.flatItems, FlatItem{
						Kind:    HandoffItem,
						Handoff: h,
						Label:   "Block: " + blocker,
					})
				}
			}
			if len(h.FilesModified) > 0 {
				for _, file := range h.FilesModified {
					m.flatItems = append(m.flatItems, FlatItem{
						Kind:    HandoffItem,
						Handoff: h,
						Label:   "File:  " + file,
					})
				}
			}
		}
	}

	// Quality section
	if len(m.data.Quality) > 0 {
		m.flatItems = append(m.flatItems, FlatItem{Kind: SectionSpacer})
		m.flatItems = append(m.flatItems, FlatItem{
			Kind:  QualityHeader,
			Label: fmt.Sprintf("%d", len(m.data.Quality)),
		})
		if !m.collapsed[QualityHeader] {
			for i := range m.data.Quality {
				q := &m.data.Quality[i]
				for _, s := range q.ShortcutsTaken {
					m.flatItems = append(m.flatItems, FlatItem{Kind: QualityItem, Quality: q, Label: "Shortcut: " + s})
				}
				for _, s := range q.TestsSkipped {
					m.flatItems = append(m.flatItems, FlatItem{Kind: QualityItem, Quality: q, Label: "Skipped: " + s})
				}
				for _, s := range q.KnownLimitations {
					m.flatItems = append(m.flatItems, FlatItem{Kind: QualityItem, Quality: q, Label: "Limit: " + s})
				}
				for _, s := range q.DeferredWork {
					m.flatItems = append(m.flatItems, FlatItem{Kind: QualityItem, Quality: q, Label: "Deferred: " + s})
				}
				for _, s := range q.TechnicalDebt {
					m.flatItems = append(m.flatItems, FlatItem{Kind: QualityItem, Quality: q, Label: "Debt: " + s})
				}
			}
		}
	}

	// Memories section
	if len(m.data.Memories) > 0 {
		m.flatItems = append(m.flatItems, FlatItem{Kind: SectionSpacer})
		m.flatItems = append(m.flatItems, FlatItem{
			Kind:  MemoryHeader,
			Label: fmt.Sprintf("%d", len(m.data.Memories)),
		})
		if !m.collapsed[MemoryHeader] {
			for i := range m.data.Memories {
				m.flatItems = append(m.flatItems, FlatItem{
					Kind:   MemoryItem,
					Memory: &m.data.Memories[i],
				})
			}
		}
	}

	// Clamp cursor
	if m.cursor >= len(m.flatItems) {
		m.cursor = max(0, len(m.flatItems)-1)
	}
}

func (m Model) View() string {
	return m.view(false)
}

func (m Model) ViewFocused() string {
	return m.view(true)
}

func (m Model) view(focused bool) string {
	w := m.width
	h := m.height
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	style := shared.ConductorBorderStyle
	if focused {
		style = shared.ConductorBorderFocusedStyle
	}
	style = style.Width(w).Height(h)

	if !m.hasConductor {
		content := shared.DimFileStyle.Render("  No conductor data")
		return style.Render(content)
	}

	if len(m.flatItems) == 0 {
		content := shared.DimFileStyle.Render("  No features")
		return style.Render(content)
	}

	// Split layout: list on top, detail on bottom
	listH, detailH := m.sectionSplit()

	// Render list items — show cursor only when list section is active (or no detail section)
	showListCursor := focused && (m.activeSection == ListSection || detailH == 0)
	var lines []string
	for i := m.scrollOffset; i < len(m.flatItems) && len(lines) < listH; i++ {
		line := m.renderItem(m.flatItems[i], i == m.cursor && showListCursor)
		lines = append(lines, line)
	}

	if detailH > 0 {
		listContent := fixedHeight(strings.Join(lines, "\n"), listH)
		divider := shared.SectionDividerStyle.Render(strings.Repeat("─", w))
		detail := fixedHeight(m.detailVP.View(), detailH)
		content := listContent + "\n" + divider + "\n" + detail
		return style.Render(content)
	}

	content := strings.Join(lines, "\n")
	return style.Render(content)
}

func (m Model) renderItem(item FlatItem, selected bool) string {
	w := m.width
	if w < 1 {
		w = 1
	}

	var line string

	switch item.Kind {
	case SectionSpacer:
		return ""

	case FeatureHeader:
		line = m.renderSectionHeader("Features", item.Label, shared.StagedSectionStyle)

	case FeatureItem:
		line = m.renderFeature(item.Feature)

	case SessionHeader:
		title := "Session"
		if item.Session != nil {
			title = fmt.Sprintf("Session #%d", item.Session.Number)
		}
		line = m.renderSectionHeader(title, item.Label, shared.StagedSectionStyle)

	case HandoffItem:
		parts := strings.SplitN(item.Label, "  ", 2)
		if len(parts) == 2 {
			line = "  " + shared.CommitDetailLabelStyle.Render(parts[0]) + " " + shared.CommitDetailMsgStyle.Render(truncate(parts[1], w-10))
		} else {
			line = "  " + shared.CommitDetailMsgStyle.Render(truncate(item.Label, w-4))
		}

	case QualityHeader:
		line = m.renderSectionHeader("Quality ("+item.Label+")", "", shared.ConductorWarningHeaderStyle)

	case QualityItem:
		label := truncate(item.Label, w-6)
		line = "  " + shared.ConductorWarningTextStyle.Render("\u26a0 "+label)

	case MemoryHeader:
		suffix := ""
		if item.Label != "" {
			suffix = item.Label
		}
		line = m.renderSectionHeader("Memories", suffix, shared.DimFileStyle)

	case MemoryItem:
		name := truncate(item.Memory.Name, w-4)
		line = "  " + shared.DimFileStyle.Render(name)
	}

	// Apply cursor highlight
	if selected {
		line = shared.CursorStyle.Width(w).Render(line)
	} else {
		// Pad to width for consistent look
		lineLen := lipgloss.Width(line)
		if lineLen < w {
			line += strings.Repeat(" ", w-lineLen)
		}
	}

	return line
}

// renderSectionHeader builds a section header: "▼ Title ──── suffix"
func (m Model) renderSectionHeader(title, suffix string, titleStyle lipgloss.Style) string {
	w := m.width
	collapsed := false
	switch {
	case strings.HasPrefix(title, "Features"):
		collapsed = m.collapsed[FeatureHeader]
	case strings.HasPrefix(title, "Session"):
		collapsed = m.collapsed[SessionHeader]
	case strings.HasPrefix(title, "Quality"):
		collapsed = m.collapsed[QualityHeader]
	case strings.HasPrefix(title, "Memories"):
		collapsed = m.collapsed[MemoryHeader]
	}

	chevron := "▼"
	if collapsed {
		chevron = "▶"
	}

	prefixText := chevron + " " + title + " "
	prefixWidth := lipgloss.Width(prefixText)

	suffixText := ""
	suffixWidth := 0
	if suffix != "" {
		suffixText = " " + suffix
		suffixWidth = lipgloss.Width(suffixText)
	}

	dividerLen := w - prefixWidth - suffixWidth
	if dividerLen < 1 {
		dividerLen = 1
	}

	divider := strings.Repeat("─", dividerLen)

	return shared.DimFileStyle.Render(chevron) + " " +
		titleStyle.Render(title) + " " +
		shared.SectionDividerStyle.Render(divider) +
		shared.DimFileStyle.Render(suffixText)
}

func (m Model) renderFeature(f *conductor.Feature) string {
	w := m.width

	var indicator string
	var descStyle lipgloss.Style

	switch f.Status {
	case "passed":
		indicator = "  " + shared.StagedFileStyle.Render("✓")
		descStyle = shared.StagedFileStyle
	case "in_progress":
		indicator = "  " + shared.UnstagedFileStyle.Render("●")
		descStyle = shared.UnstagedFileStyle
	case "failed":
		indicator = "  " + shared.ErrorStyle.Render("✗")
		descStyle = shared.ErrorStyle
	case "blocked":
		indicator = "  " + shared.DimFileStyle.Render("◌")
		descStyle = shared.DimFileStyle
	default: // pending
		indicator = "  " + shared.DimFileStyle.Render("○")
		descStyle = shared.DimFileStyle
	}

	descW := w - 5
	badges := ""
	if f.Status == "failed" && f.AttemptCount > 1 {
		badge := fmt.Sprintf(" [x%d]", f.AttemptCount)
		descW -= len(badge)
		badges = " " + shared.ErrorStyle.Render(fmt.Sprintf("[x%d]", f.AttemptCount))
	}
	if f.Status == "in_progress" {
		badge := " [active]"
		descW -= len(badge)
		badges += " " + shared.ConductorActiveBadge.Render("active")
	}
	if descW < 5 {
		descW = 5
	}

	desc := truncate(f.Description, descW)
	return indicator + " " + descStyle.Render(desc) + badges
}

// --- Detail section ---

func (m Model) renderDetail(w, h int) string {
	if len(m.flatItems) == 0 || m.cursor < 0 || m.cursor >= len(m.flatItems) {
		return shared.DimFileStyle.Render("  No selection")
	}

	item := m.flatItems[m.cursor]
	label := shared.CommitDetailLabelStyle

	var b strings.Builder

	switch item.Kind {
	case FeatureItem:
		f := item.Feature
		b.WriteString("\n")

		// Full description with word wrap
		descLines := wordWrap(f.Description, w-12)
		b.WriteString(label.Render("  desc   ") + " " + descLines[0] + "\n")
		for _, dl := range descLines[1:] {
			b.WriteString("           " + dl + "\n")
		}

		// Status with colored indicator
		var statusStyled string
		switch f.Status {
		case "passed":
			statusStyled = shared.StagedFileStyle.Render(f.Status)
		case "in_progress":
			statusStyled = shared.UnstagedFileStyle.Render(f.Status)
		case "failed":
			statusStyled = shared.ErrorStyle.Render(f.Status)
		default:
			statusStyled = shared.DimFileStyle.Render(f.Status)
		}
		b.WriteString(label.Render("  status ") + " " + statusStyled + "\n")

		b.WriteString(label.Render("  phase  ") + " " + shared.DimFileStyle.Render(fmt.Sprintf("%d", f.Phase)) + "\n")
		b.WriteString(label.Render("  cat    ") + " " + shared.DimFileStyle.Render(f.Category) + "\n")

		if f.AttemptCount > 1 {
			b.WriteString(label.Render("  tries  ") + " " + shared.ErrorStyle.Render(fmt.Sprintf("%d", f.AttemptCount)) + "\n")
		}
		if f.CommitHash != "" {
			hash := f.CommitHash
			if len(hash) > 12 {
				hash = hash[:12]
			}
			b.WriteString(label.Render("  commit ") + " " + shared.CommitDetailHashStyle.Render(hash) + "\n")
		}
		if f.LastError != "" {
			errLines := wordWrap(f.LastError, w-12)
			b.WriteString(label.Render("  error  ") + " " + shared.ErrorStyle.Render(errLines[0]) + "\n")
			for _, el := range errLines[1:] {
				b.WriteString("           " + shared.ErrorStyle.Render(el) + "\n")
			}
		}

	case MemoryItem:
		mem := item.Memory
		b.WriteString("\n")
		b.WriteString(label.Render("  name   ") + " " + shared.CommitDetailMsgStyle.Render(mem.Name) + "\n")
		if len(mem.Tags) > 0 {
			b.WriteString(label.Render("  tags   ") + " " + shared.DimFileStyle.Render(strings.Join(mem.Tags, ", ")) + "\n")
		}
		b.WriteString("\n")

		// Content — viewport handles scrolling
		contentLines := strings.Split(mem.Content, "\n")
		for _, cl := range contentLines {
			if len(cl) > w-4 {
				cl = cl[:w-7] + "..."
			}
			b.WriteString("  " + cl + "\n")
		}

	case QualityItem:
		q := item.Quality
		b.WriteString("\n")
		b.WriteString(label.Render("  type   ") + " " + shared.DimFileStyle.Render(q.ReflectionType) + "\n")
		b.WriteString("\n")
		// Show the full quality issue text with wrapping
		fullText := item.Label
		lines := wordWrap(fullText, w-6)
		for _, l := range lines {
			b.WriteString("  " + shared.ConductorWarningTextStyle.Render(l) + "\n")
		}

	case HandoffItem:
		b.WriteString("\n")
		// Show full handoff text with wrapping
		lines := wordWrap(item.Label, w-4)
		for _, l := range lines {
			b.WriteString("  " + shared.CommitDetailMsgStyle.Render(l) + "\n")
		}

		// Also show full handoff context if available
		if item.Handoff != nil {
			h := item.Handoff
			b.WriteString("\n")
			if h.CurrentTask != "" {
				b.WriteString(label.Render("  task   ") + " " + h.CurrentTask + "\n")
			}
			if len(h.NextSteps) > 0 {
				b.WriteString(label.Render("  next   ") + " " + strings.Join(h.NextSteps, "\n           ") + "\n")
			}
			if len(h.Blockers) > 0 {
				b.WriteString(label.Render("  blocks ") + " " + shared.ErrorStyle.Render(strings.Join(h.Blockers, ", ")) + "\n")
			}
		}

	case SessionHeader:
		if item.Session != nil {
			s := item.Session
			b.WriteString("\n")
			b.WriteString(label.Render("  session") + " " + fmt.Sprintf("#%d", s.Number) + "\n")
			b.WriteString(label.Render("  status ") + " " + shared.DimFileStyle.Render(s.Status) + "\n")
			if s.ProgressNotes != "" {
				noteLines := wordWrap(s.ProgressNotes, w-12)
				b.WriteString(label.Render("  notes  ") + " " + noteLines[0] + "\n")
				for _, nl := range noteLines[1:] {
					b.WriteString("           " + nl + "\n")
				}
			}
		}

	case FeatureHeader:
		if m.data != nil {
			b.WriteString("\n")
			b.WriteString(label.Render("  total  ") + " " + fmt.Sprintf("%d features", m.data.Total) + "\n")
			b.WriteString(label.Render("  passed ") + " " + shared.StagedFileStyle.Render(fmt.Sprintf("%d", m.data.Passed)) + "\n")
			remaining := m.data.Total - m.data.Passed
			if remaining > 0 {
				b.WriteString(label.Render("  remain ") + " " + shared.UnstagedFileStyle.Render(fmt.Sprintf("%d", remaining)) + "\n")
			}
			// Show counts by status
			counts := make(map[string]int)
			for _, f := range m.data.Features {
				counts[f.Status]++
			}
			if n := counts["in_progress"]; n > 0 {
				b.WriteString(label.Render("  active ") + " " + shared.UnstagedFileStyle.Render(fmt.Sprintf("%d", n)) + "\n")
			}
			if n := counts["failed"]; n > 0 {
				b.WriteString(label.Render("  failed ") + " " + shared.ErrorStyle.Render(fmt.Sprintf("%d", n)) + "\n")
			}
			if n := counts["blocked"]; n > 0 {
				b.WriteString(label.Render("  blocked") + " " + shared.DimFileStyle.Render(fmt.Sprintf("%d", n)) + "\n")
			}
		}

	case QualityHeader:
		if m.data != nil {
			b.WriteString("\n")
			b.WriteString(label.Render("  issues ") + " " + shared.ConductorWarningTextStyle.Render(fmt.Sprintf("%d unresolved", len(m.data.Quality))) + "\n")
			// Show summary of quality categories
			var shortcuts, tests, limits, deferred, debt int
			for _, q := range m.data.Quality {
				shortcuts += len(q.ShortcutsTaken)
				tests += len(q.TestsSkipped)
				limits += len(q.KnownLimitations)
				deferred += len(q.DeferredWork)
				debt += len(q.TechnicalDebt)
			}
			if shortcuts > 0 {
				b.WriteString(label.Render("  short  ") + " " + fmt.Sprintf("%d shortcuts", shortcuts) + "\n")
			}
			if tests > 0 {
				b.WriteString(label.Render("  tests  ") + " " + fmt.Sprintf("%d skipped", tests) + "\n")
			}
			if limits > 0 {
				b.WriteString(label.Render("  limits ") + " " + fmt.Sprintf("%d known", limits) + "\n")
			}
			if debt > 0 {
				b.WriteString(label.Render("  debt   ") + " " + fmt.Sprintf("%d items", debt) + "\n")
			}
		}

	case MemoryHeader:
		if m.data != nil {
			b.WriteString("\n")
			b.WriteString(label.Render("  saved  ") + " " + fmt.Sprintf("%d memories", len(m.data.Memories)) + "\n")
			// Show all memory names
			for _, mem := range m.data.Memories {
				tags := ""
				if len(mem.Tags) > 0 {
					tags = " " + shared.DimFileStyle.Render("["+strings.Join(mem.Tags, ",")+"]")
				}
				b.WriteString("  " + shared.CommitDetailMsgStyle.Render(truncate(mem.Name, w-8)) + tags + "\n")
			}
		}

	default:
		b.WriteString(shared.DimFileStyle.Render("  Select an item for details"))
	}

	return b.String()
}

// --- Helpers ---

func truncate(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// wordWrap breaks text into lines of at most width characters, splitting at spaces.
func wordWrap(s string, width int) []string {
	if width < 5 {
		width = 5
	}
	if len(s) <= width {
		return []string{s}
	}

	var result []string
	remaining := s
	for len(remaining) > 0 {
		if len(remaining) <= width {
			result = append(result, remaining)
			break
		}
		// Find a space to break at
		breakAt := width
		for i := width; i > width/2; i-- {
			if remaining[i] == ' ' {
				breakAt = i
				break
			}
		}
		result = append(result, remaining[:breakAt])
		remaining = strings.TrimLeft(remaining[breakAt:], " ")
	}
	return result
}

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
