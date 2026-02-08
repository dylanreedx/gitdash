package shared

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/dylan/gitdash/config"
)

var (
	// Repo header
	RepoHeaderStyle lipgloss.Style
	BranchStyle     lipgloss.Style

	// Section headers
	StagedSectionStyle   lipgloss.Style
	UnstagedSectionStyle lipgloss.Style

	// File entries
	StagedFileStyle   lipgloss.Style
	UnstagedFileStyle lipgloss.Style
	DimFileStyle      lipgloss.Style
	MutedFileStyle    lipgloss.Style

	// Cursor highlight
	CursorStyle lipgloss.Style

	// Diff styles
	DiffAddStyle    lipgloss.Style
	DiffRemoveStyle lipgloss.Style
	DiffHunkStyle   lipgloss.Style
	DiffMetaStyle   lipgloss.Style

	// Diff header/footer
	DiffHeaderStyle lipgloss.Style
	DiffFooterStyle lipgloss.Style

	// Status bar
	StatusBarStyle lipgloss.Style

	// Help styles
	HelpKeyStyle     lipgloss.Style
	HelpDescStyle    lipgloss.Style
	HelpOverlayStyle lipgloss.Style

	// Commit view
	CommitHeaderStyle lipgloss.Style
	CommitFileStyle   lipgloss.Style

	// Folder headers
	FolderAccentStyle lipgloss.Style
	FolderDimStyle    lipgloss.Style

	// Error
	ErrorStyle lipgloss.Style

	// Graph pane
	GraphHashStyle          lipgloss.Style
	GraphRefStyle           lipgloss.Style
	PrefixBadgeStyles       map[string]lipgloss.Style
	PrefixBadgeFallback     lipgloss.Style
	GraphBorderStyle        lipgloss.Style
	GraphBorderFocusedStyle lipgloss.Style
	GraphLineColors         []lipgloss.Style

	// Commit detail
	CommitDetailHashStyle   lipgloss.Style
	CommitDetailAuthorStyle lipgloss.Style
	CommitDetailDateStyle   lipgloss.Style
	CommitStatAddStyle      lipgloss.Style
	CommitStatDelStyle      lipgloss.Style
	CommitFileHeaderStyle   lipgloss.Style
	SectionDividerStyle     lipgloss.Style

	// Branch picker
	BranchPickerOverlayStyle lipgloss.Style
	BranchCurrentStyle       lipgloss.Style
	BranchItemStyle          lipgloss.Style
	BranchPrefixStyle        lipgloss.Style

	// Brutalist styling
	CommitDetailLabelStyle lipgloss.Style
	CommitDetailMsgStyle   lipgloss.Style
	PathDirStyle           lipgloss.Style
	PathFileStyle          lipgloss.Style
	StatAddBadge           lipgloss.Style
	StatDelBadge           lipgloss.Style
	FolderColorStyles      map[string]lipgloss.Style

	// Sync status badges
	SyncPushBadge lipgloss.Style
	SyncPullBadge lipgloss.Style

	// Spinner
	SpinnerStyle lipgloss.Style

	// Feedback
	FeedbackSuccessStyle lipgloss.Style
	FeedbackWarningStyle lipgloss.Style
	FeedbackErrorStyle   lipgloss.Style

	// Indicators
	StagedIndicator   string
	UnstagedIndicator string

	// Conductor pane
	ConductorBorderStyle        lipgloss.Style
	ConductorBorderFocusedStyle lipgloss.Style

	// Conductor status badges
	ConductorPassedBadge      lipgloss.Style
	ConductorActiveBadge      lipgloss.Style
	ConductorQualityBadge     lipgloss.Style
	ConductorWarningHeaderStyle lipgloss.Style
	ConductorWarningTextStyle   lipgloss.Style
)

// InitStyles configures all styles from a resolved theme.
// Optional graphColors overrides the default graph color palette.
func InitStyles(theme config.ThemeConfig, graphColors ...[]string) {
	RepoHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(theme.RepoHeader))

	BranchStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Branch))

	StagedSectionStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Staged))

	UnstagedSectionStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Unstaged))

	StagedFileStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Staged))

	UnstagedFileStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Unstaged))

	DimFileStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Dim))

	MutedFileStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted))

	CursorStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(theme.CursorBG))

	DiffAddStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.DiffAdd))

	DiffRemoveStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.DiffRemove))

	DiffHunkStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.DiffHunk))

	DiffMetaStyle = lipgloss.NewStyle().
		Bold(true)

	DiffHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(theme.FG)).
		Background(lipgloss.Color(theme.CursorBG)).
		Padding(0, 1)

	DiffFooterStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Dim)).
		Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.StatusBarFG)).
		Background(lipgloss.Color(theme.StatusBarBG)).
		Padding(0, 1)

	HelpKeyStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent))

	HelpDescStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Dim))

	HelpOverlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Muted)).
		Padding(1, 2)

	CommitHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(theme.Accent))

	CommitFileStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Staged))

	FolderAccentStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent)).
		Bold(true)

	FolderDimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Dim))

	ErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Error))

	GraphHashStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Dim))

	GraphRefStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent)).
		Bold(true)

	PrefixBadgeStyles = make(map[string]lipgloss.Style)
	for name, pc := range theme.PrefixColors {
		PrefixBadgeStyles[name] = lipgloss.NewStyle().
			Foreground(lipgloss.Color(pc.FG)).
			Background(lipgloss.Color(pc.BG)).
			Padding(0, 1)
	}
	PrefixBadgeFallback = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent2)).
		Background(lipgloss.Color("#1a1a1a")).
		Padding(0, 1)

	GraphBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(theme.Muted))

	GraphBorderFocusedStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(theme.Accent))

	gc := config.DefaultGraphColors()
	if len(graphColors) > 0 && len(graphColors[0]) > 0 {
		gc = graphColors[0]
	}
	GraphLineColors = make([]lipgloss.Style, len(gc))
	for i, c := range gc {
		GraphLineColors[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(c))
	}

	CommitDetailHashStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent))

	CommitDetailAuthorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.FG)).
		Bold(true)

	CommitDetailDateStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Dim))

	CommitStatAddStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.DiffAdd))

	CommitStatDelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.DiffRemove))

	CommitFileHeaderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.FG))

	SectionDividerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted))

	BranchPickerOverlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Accent)).
		Padding(1, 2)

	BranchCurrentStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent2)).
		Bold(true)

	BranchItemStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.FG))

	BranchPrefixStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent))

	// Brutalist styling
	CommitDetailLabelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.CommitDetailLabelFG))

	CommitDetailMsgStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.FG))

	PathDirStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.PathDirFG))

	PathFileStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.PathFileFG)).
		Bold(true)

	StatAddBadge = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.DiffAdd)).
		Background(lipgloss.Color(theme.StatAddBG)).
		Padding(0, 1)

	StatDelBadge = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.DiffRemove)).
		Background(lipgloss.Color(theme.StatDelBG)).
		Padding(0, 1)

	FolderColorStyles = make(map[string]lipgloss.Style)
	for name, hex := range theme.FolderColors {
		FolderColorStyles[name] = lipgloss.NewStyle().
			Foreground(lipgloss.Color(hex)).
			Bold(true)
	}

	SyncPushBadge = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.SyncPushFG)).
		Background(lipgloss.Color(theme.SyncPushBG)).
		Padding(0, 1)

	SyncPullBadge = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.SyncPullFG)).
		Background(lipgloss.Color(theme.SyncPullBG)).
		Padding(0, 1)

	SpinnerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.SpinnerFG))

	FeedbackSuccessStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.FeedbackSuccessFG)).
		Background(lipgloss.Color(theme.FeedbackSuccessBG)).
		Padding(0, 1)

	FeedbackWarningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.FeedbackWarningFG)).
		Background(lipgloss.Color(theme.FeedbackWarningBG)).
		Padding(0, 1)

	FeedbackErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.FeedbackErrorFG)).
		Background(lipgloss.Color(theme.FeedbackErrorBG)).
		Padding(0, 1)

	StagedIndicator = StagedFileStyle.Render("✓")
	UnstagedIndicator = UnstagedFileStyle.Render("○")

	// Conductor pane — reuse graph border pattern
	ConductorBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(theme.Muted))

	ConductorBorderFocusedStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(theme.Accent))

	ConductorPassedBadge = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Staged)).
		Background(lipgloss.Color(theme.StatAddBG)).
		Padding(0, 1)

	ConductorActiveBadge = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Unstaged)).
		Background(lipgloss.Color(theme.StatDelBG)).
		Padding(0, 1)

	ConductorQualityBadge = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.FeedbackWarningFG)).
		Background(lipgloss.Color(theme.FeedbackWarningBG)).
		Padding(0, 1)

	// Conductor warning styles — FG only for list items (no background/padding bloat)
	ConductorWarningHeaderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.FeedbackWarningFG))

	ConductorWarningTextStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.FeedbackWarningFG))
}

// RenderPath renders a file path with dim directories and bright filename.
// "src/components/Button.tsx" → dim("src/components/") + bright("Button.tsx")
func RenderPath(fullPath string) string {
	dir := filepath.Dir(fullPath)
	base := filepath.Base(fullPath)
	if dir == "." || dir == "" {
		return PathFileStyle.Render(base)
	}
	return PathDirStyle.Render(dir+string(filepath.Separator)) + PathFileStyle.Render(base)
}

// RenderPathWithStyle renders a file path, applying the given style to the filename
// instead of PathFileStyle. Used for tier-based styling in the dashboard.
func RenderPathWithStyle(fullPath string, fileStyle lipgloss.Style) string {
	dir := filepath.Dir(fullPath)
	base := filepath.Base(fullPath)
	if dir == "." || dir == "" {
		return fileStyle.Render(base)
	}
	return PathDirStyle.Render(dir+string(filepath.Separator)) + fileStyle.Render(base)
}

// FolderStyle returns the configured style for a folder name, falling back to FolderDimStyle.
func FolderStyle(dirName string) lipgloss.Style {
	if s, ok := FolderColorStyles[strings.ToLower(dirName)]; ok {
		return s
	}
	return FolderDimStyle
}

// ResolveSpinnerType maps a config string to a bubbles spinner type.
func ResolveSpinnerType(name string) spinner.Spinner {
	switch strings.ToLower(name) {
	case "dot":
		return spinner.Dot
	case "line":
		return spinner.Line
	case "minidot":
		return spinner.MiniDot
	case "pulse":
		return spinner.Pulse
	case "points":
		return spinner.Points
	case "meter":
		return spinner.Meter
	case "ellipsis":
		return spinner.Ellipsis
	default:
		return spinner.MiniDot
	}
}

func init() {
	// Initialize with defaults so styles work even without explicit InitStyles call
	InitStyles(config.DefaultTheme())
}
