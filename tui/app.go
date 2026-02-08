package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dylan/gitdash/ai"
	"github.com/dylan/gitdash/config"
	"github.com/dylan/gitdash/git"
	"github.com/dylan/gitdash/nvim"
	"github.com/dylan/gitdash/conductor"
	"github.com/dylan/gitdash/tui/branchpicker"
	"github.com/dylan/gitdash/tui/commitview"
	"github.com/dylan/gitdash/tui/conductorpane"
	"github.com/dylan/gitdash/tui/dashboard"
	"github.com/dylan/gitdash/tui/diffview"
	"github.com/dylan/gitdash/tui/featurelinker"
	"github.com/dylan/gitdash/tui/graphpane"
	"github.com/dylan/gitdash/tui/help"
	"github.com/dylan/gitdash/tui/icons"
	"github.com/dylan/gitdash/tui/shared"
)

const pollInterval = 2 * time.Second

type pollTickMsg time.Time

type ActiveView int

const (
	DashboardView ActiveView = iota
	DiffView
	CommitView
	BranchPickerView
)

// FocusPanel tracks which column has focus in the 3-column layout.
type FocusPanel int

const (
	FocusDashboard  FocusPanel = iota
	FocusGraph
	FocusConductor
)

type App struct {
	cfg        config.Config
	activeView ActiveView
	showHelp   bool
	statusMsg  string
	statusTime time.Time

	dashboard      dashboard.Model
	diffView       diffview.Model
	commitView     commitview.Model
	helpView       help.Model
	graphPane      graphpane.Model
	branchPicker   branchpicker.Model
	conductorPane  conductorpane.Model
	featureLinker  featurelinker.Model

	showGraph       bool
	graphFocused    bool
	focusPanel      FocusPanel
	graphRepo       string // repo path of last graph fetch
	lastDetailHash  string // hash of last fetched commit detail
	conductorRepo   string // repo path of last conductor fetch

	// Conductor data cache (per repo)
	conductorData   map[string]*conductor.ConductorData

	// Animated loaders
	spinners      map[shared.LoaderOp]spinner.Model
	spinnerLabels map[shared.LoaderOp]string
	pushingRepoIdx int // repo index being pushed (-1 = none)

	// Feedback system
	feedback *shared.Feedback

	width  int
	height int
}

func NewApp(cfg config.Config) App {
	shared.InitStyles(cfg.ResolvedTheme(), cfg.ResolvedGraphColors())
	icons.SetNerdFonts(cfg.Display.NerdFonts)

	gp := graphpane.New()
	gp.SetShowIcons(cfg.Display.Icons || cfg.Display.NerdFonts)

	return App{
		cfg:            cfg,
		activeView:     DashboardView,
		dashboard:      dashboard.New(cfg.ResolvedPriorityRules(), cfg.Display),
		diffView:       diffview.New(),
		commitView:     commitview.New(),
		helpView:       help.New(),
		graphPane:      gp,
		branchPicker:   branchpicker.New(),
		conductorPane:  conductorpane.New(),
		featureLinker:  featurelinker.New(),
		showGraph:      cfg.ResolvedShowGraph(),
		focusPanel:     FocusDashboard,
		conductorData:  make(map[string]*conductor.ConductorData),
		spinners:       make(map[shared.LoaderOp]spinner.Model),
		spinnerLabels:  make(map[shared.LoaderOp]string),
		pushingRepoIdx: -1,
	}
}

func (a *App) setStatus(msg string) {
	a.statusMsg = msg
	a.statusTime = time.Now()
}

func (a *App) newSpinner() spinner.Model {
	theme := a.cfg.ResolvedTheme()
	s := spinner.New()
	s.Spinner = shared.ResolveSpinnerType(theme.SpinnerType)
	s.Style = shared.SpinnerStyle
	return s
}

func (a *App) startLoader(op shared.LoaderOp, label string) tea.Cmd {
	s := a.newSpinner()
	a.spinners[op] = s
	a.spinnerLabels[op] = label
	return s.Tick
}

func (a *App) stopLoader(op shared.LoaderOp) {
	delete(a.spinners, op)
	delete(a.spinnerLabels, op)
}

func (a *App) setFeedback(level shared.FeedbackLevel, message string, detail string, op shared.LoaderOp) {
	a.feedback = &shared.Feedback{
		Level:     level,
		Message:   message,
		Detail:    detail,
		Timestamp: time.Now(),
		Op:        op,
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(refreshAllStatus(a.cfg), pollTickCmd())
}

func pollTickCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(t time.Time) tea.Msg {
		return pollTickMsg(t)
	})
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.layoutSizes()
		a.diffView.SetSize(msg.Width, msg.Height)
		a.commitView.SetSize(msg.Width, msg.Height)
		a.helpView.SetSize(msg.Width, msg.Height)
		a.branchPicker.SetSize(msg.Width, msg.Height)
		a.featureLinker.SetSize(msg.Width, msg.Height)
		return a, nil

	case shared.LoaderStartMsg:
		cmd := a.startLoader(msg.Op, msg.Label)
		return a, cmd

	case shared.LoaderStopMsg:
		a.stopLoader(msg.Op)
		if msg.Op == shared.OpPush && a.pushingRepoIdx >= 0 {
			a.dashboard.ClearRepoPushing(a.pushingRepoIdx)
			a.pushingRepoIdx = -1
		}
		return a, nil

	case shared.FeedbackMsg:
		a.feedback = &msg.Feedback
		if a.feedback.Timestamp.IsZero() {
			a.feedback.Timestamp = time.Now()
		}
		return a, nil

	case shared.DismissFeedbackMsg:
		a.feedback = nil
		return a, nil

	case spinner.TickMsg:
		var cmds []tea.Cmd
		for op, s := range a.spinners {
			var cmd tea.Cmd
			s, cmd = s.Update(msg)
			a.spinners[op] = s
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		// Pass updated spinner views to child components
		if s, ok := a.spinners[shared.OpPush]; ok && a.pushingRepoIdx >= 0 {
			a.dashboard.SetRepoPushing(a.pushingRepoIdx, s.View())
		}
		if s, ok := a.spinners[shared.OpGenerate]; ok {
			a.commitView.SetSpinnerView(s.View())
		}
		return a, tea.Batch(cmds...)

	case shared.StatusRefreshedMsg:
		a.dashboard.SetRepos(msg.Repos)
		// Auto-clear legacy status messages after 4s
		if a.statusMsg != "" && time.Since(a.statusTime) > 4*time.Second {
			a.statusMsg = ""
		}
		// Auto-clear feedback based on TTL per level
		if a.feedback != nil && a.feedback.Level != shared.FeedbackFatal {
			ttl := shared.FeedbackTTL(a.feedback.Level)
			if ttl > 0 && time.Since(a.feedback.Timestamp) > ttl {
				a.feedback = nil
			}
		}
		return a, a.maybeRefreshGraph()

	case shared.FileStageToggledMsg, shared.AllStagedMsg, shared.AllUnstagedMsg:
		return a, refreshAllStatus(a.cfg)

	case shared.DiffFetchedMsg:
		if msg.Err != nil {
			a.setStatus("Error: " + msg.Err.Error())
			return a, nil
		}
		a.activeView = DiffView
		a.diffView.SetSize(a.width, a.height)
		item, _ := a.dashboard.SelectedItem()
		a.diffView.SetContent(msg.Content, item.File.Path, item.Repo.Path)
		return a, nil

	case shared.CommitCompleteMsg:
		if msg.Err != nil {
			a.commitView.SetError(msg.Err)
			return a, nil
		}
		a.activeView = DashboardView
		a.setFeedback(shared.FeedbackSuccess, "Committed successfully", "", "")
		cmds := []tea.Cmd{refreshAllStatus(a.cfg)}
		// Try to match commit to conductor feature
		if repo, ok := a.dashboard.SelectedRepo(); ok {
			commitMsg := a.commitView.Value()
			cmds = append(cmds, matchFeaturesCmd(repo.Path, "", commitMsg, nil))
		}
		return a, tea.Batch(cmds...)

	case shared.AICommitMsgMsg:
		a.stopLoader(shared.OpGenerate)
		a.commitView.SetGenerating(false)
		if msg.Err != nil {
			a.commitView.SetError(msg.Err)
		} else {
			a.commitView.SetAIMessage(msg.Message)
		}
		return a, nil

	case shared.PushCompleteMsg:
		a.stopLoader(shared.OpPush)
		if a.pushingRepoIdx >= 0 {
			a.dashboard.ClearRepoPushing(a.pushingRepoIdx)
			a.pushingRepoIdx = -1
		}
		if msg.Err != nil {
			a.setFeedback(shared.FeedbackError, "Push failed: "+msg.Err.Error(), msg.Err.Error(), shared.OpPush)
			return a, nil
		}
		a.setFeedback(shared.FeedbackSuccess, "Pushed "+msg.Branch+" to origin", "", shared.OpPush)
		return a, refreshAllStatus(a.cfg)

	case shared.ContextSummaryCopiedMsg:
		a.stopLoader(shared.OpExport)
		if msg.Err != nil {
			a.setFeedback(shared.FeedbackError, "Export failed: "+msg.Err.Error(), msg.Err.Error(), shared.OpExport)
		} else {
			a.setFeedback(shared.FeedbackSuccess, fmt.Sprintf("Context copied to clipboard (%d commits across %d repos)", msg.NumCommits, msg.NumRepos), "", shared.OpExport)
		}
		return a, nil

	case conductorDataMsg:
		a.conductorData[msg.RepoPath] = msg.Data
		a.conductorPane.SetData(msg.Data)
		return a, nil

	case featureMatchMsg:
		if len(msg.Matches) > 0 {
			a.featureLinker.Show(msg.Matches, msg.CommitHash, msg.CommitMsg)
		}
		return a, nil

	case shared.FeatureLinkedMsg:
		if msg.Err == nil {
			a.setFeedback(shared.FeedbackSuccess, "Linked to: "+msg.Description, "", "")
			// Refresh conductor data
			if repo, ok := a.dashboard.SelectedRepo(); ok {
				a.conductorRepo = "" // force refresh
				return a, refreshConductorCmd(repo.Path)
			}
		}
		return a, nil

	case nvim.EditorFinishedMsg:
		return a, refreshAllStatus(a.cfg)

	case shared.CloseDiffMsg:
		a.activeView = DashboardView
		return a, refreshAllStatus(a.cfg)

	case shared.CloseCommitMsg:
		a.activeView = DashboardView
		return a, nil

	case shared.GraphFetchedMsg:
		if msg.Err == nil {
			a.graphPane.SetGraph(msg.Lines, msg.RepoPath)
		}
		return a, nil

	case shared.CommitDetailFetchedMsg:
		if msg.Err == nil {
			a.graphPane.SetCommitDetail(msg.Detail)
			a.lastDetailHash = msg.Hash
		}
		return a, nil

	case shared.CommitFileDiffFetchedMsg:
		if msg.Err == nil {
			a.graphPane.SetFileDiff(msg.FilePath, msg.Diff)
		}
		return a, nil

	case shared.BranchesFetchedMsg:
		if msg.Err != nil {
			a.setStatus("Error: " + msg.Err.Error())
			return a, nil
		}
		a.branchPicker.SetBranches(msg.Branches, msg.RepoPath)
		a.activeView = BranchPickerView
		return a, nil

	case shared.BranchSwitchedMsg:
		if msg.Err != nil {
			a.setStatus("Error: " + msg.Err.Error())
		} else {
			a.setStatus("Switched to " + msg.Branch)
		}
		a.activeView = DashboardView
		a.graphRepo = "" // force graph refresh
		return a, refreshAllStatus(a.cfg)

	case shared.BranchCreatedMsg:
		if msg.Err != nil {
			a.setStatus("Error: " + msg.Err.Error())
		} else {
			a.setStatus("Created " + msg.Branch)
		}
		a.activeView = DashboardView
		a.graphRepo = "" // force graph refresh
		return a, refreshAllStatus(a.cfg)

	case shared.CloseBranchPickerMsg:
		a.activeView = DashboardView
		return a, nil

	case pollTickMsg:
		// Auto-clear feedback based on TTL (runs on every poll, even outside dashboard)
		if a.feedback != nil && a.feedback.Level != shared.FeedbackFatal {
			ttl := shared.FeedbackTTL(a.feedback.Level)
			if ttl > 0 && time.Since(a.feedback.Timestamp) > ttl {
				a.feedback = nil
			}
		}
		// Auto-clear legacy status messages
		if a.statusMsg != "" && time.Since(a.statusTime) > 4*time.Second {
			a.statusMsg = ""
		}
		// Only auto-refresh on the dashboard view to avoid disrupting other views
		if a.activeView == DashboardView || a.activeView == BranchPickerView {
			cmds := []tea.Cmd{refreshAllStatus(a.cfg), pollTickCmd()}
			// Refresh conductor data on the same tick
			if repo, ok := a.dashboard.SelectedRepo(); ok {
				cmds = append(cmds, refreshConductorCmd(repo.Path))
			}
			return a, tea.Batch(cmds...)
		}
		return a, pollTickCmd()

	case tea.KeyMsg:
		return a.handleKey(msg)
	}

	// Route updates to active view
	switch a.activeView {
	case DiffView:
		var cmd tea.Cmd
		a.diffView, cmd = a.diffView.Update(msg)
		return a, cmd
	case CommitView:
		var cmd tea.Cmd
		a.commitView, cmd = a.commitView.Update(msg)
		return a, cmd
	case BranchPickerView:
		var cmd tea.Cmd
		a.branchPicker, cmd = a.branchPicker.Update(msg)
		return a, cmd
	}

	return a, nil
}

func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Fatal feedback overlay: any key dismisses
	if a.feedback != nil && a.feedback.Level == shared.FeedbackFatal {
		a.feedback = nil
		return a, nil
	}

	// Help toggle is global
	if key.Matches(msg, shared.Keys.Help) {
		a.showHelp = !a.showHelp
		return a, nil
	}

	// If help is shown, any key closes it
	if a.showHelp {
		a.showHelp = false
		return a, nil
	}

	switch a.activeView {
	case DashboardView:
		return a.handleDashboardKey(msg)
	case DiffView:
		return a.handleDiffKey(msg)
	case CommitView:
		return a.handleCommitKey(msg)
	case BranchPickerView:
		return a.handleBranchPickerKey(msg)
	}

	return a, nil
}

func (a App) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Feature linker overlay takes priority
	if a.featureLinker.IsVisible() {
		result := a.featureLinker.HandleKey(msg)
		switch result.Action {
		case featurelinker.ActionLink:
			a.featureLinker.Hide()
			if result.Feature != nil {
				if repo, ok := a.dashboard.SelectedRepo(); ok {
					return a, linkFeatureCmd(repo.Path, result.Feature.Feature.ID,
						a.featureLinker.CommitHash(), a.featureLinker.CommitMsg(), nil)
				}
			}
			return a, nil
		case featurelinker.ActionSkip:
			a.featureLinker.Hide()
			return a, nil
		}
		return a, nil
	}

	// When conductor is focused, route keys to conductor pane
	if a.focusPanel == FocusConductor {
		switch {
		case key.Matches(msg, shared.Keys.FocusLeft):
			a.focusPanel = FocusGraph
			a.graphFocused = true
			return a, nil
		case key.Matches(msg, shared.Keys.Escape):
			// If in detail section, let conductor handle Escape (back to list)
			if a.conductorPane.ActiveSection() == conductorpane.DetailSection {
				var cmd tea.Cmd
				a.conductorPane, cmd = a.conductorPane.Update(msg)
				return a, cmd
			}
			a.focusPanel = FocusDashboard
			a.graphFocused = false
			return a, nil
		case key.Matches(msg, shared.Keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, shared.Keys.ToggleGraph):
			a.showGraph = false
			a.graphFocused = false
			a.focusPanel = FocusDashboard
			a.layoutSizes()
			return a, nil
		default:
			var cmd tea.Cmd
			a.conductorPane, cmd = a.conductorPane.Update(msg)
			return a, cmd
		}
	}

	// When graph is focused, route keys to the graph pane
	if a.graphFocused || a.focusPanel == FocusGraph {
		switch {
		case key.Matches(msg, shared.Keys.FocusLeft), key.Matches(msg, shared.Keys.Escape):
			a.graphFocused = false
			a.focusPanel = FocusDashboard
			return a, nil
		case key.Matches(msg, shared.Keys.FocusRight):
			if a.showGraph && a.width > 80 {
				a.focusPanel = FocusConductor
				a.graphFocused = false
				return a, nil
			}
			return a, nil
		case key.Matches(msg, shared.Keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, shared.Keys.ToggleGraph):
			a.showGraph = false
			a.graphFocused = false
			a.focusPanel = FocusDashboard
			a.layoutSizes()
			return a, nil
		default:
			// Pass j/k/ctrl+j/ctrl+k/enter/pgup/pgdn etc. to graph pane
			prevHash := a.graphPane.SelectedHash()
			var cmd tea.Cmd
			a.graphPane, cmd = a.graphPane.Update(msg)
			// Auto-fetch commit detail when cursor moves to new commit
			newHash := a.graphPane.SelectedHash()
			if newHash != "" && newHash != prevHash && newHash != a.lastDetailHash {
				detailCmd := fetchCommitDetailCmd(a.graphPane.RepoPath(), newHash)
				if cmd != nil {
					return a, tea.Batch(cmd, detailCmd)
				}
				return a, detailCmd
			}
			return a, cmd
		}
	}

	switch {
	case key.Matches(msg, shared.Keys.Quit):
		return a, tea.Quit

	case key.Matches(msg, shared.Keys.FocusRight):
		if a.showGraph {
			a.graphFocused = true
			a.focusPanel = FocusGraph
		}
		return a, nil

	case key.Matches(msg, shared.Keys.ToggleGraph):
		a.showGraph = !a.showGraph
		a.graphFocused = false
		a.focusPanel = FocusDashboard
		a.layoutSizes()
		if a.showGraph {
			a.graphRepo = ""     // force refresh
			a.conductorRepo = "" // force refresh
			cmds := []tea.Cmd{a.maybeRefreshGraph(), a.maybeRefreshConductor()}
			return a, tea.Batch(cmds...)
		}
		return a, nil

	case key.Matches(msg, shared.Keys.Push):
		item, ok := a.dashboard.SelectedItem()
		if !ok {
			return a, nil
		}
		repo := item.Repo
		a.pushingRepoIdx = item.RepoIndex
		spinCmd := a.startLoader(shared.OpPush, "Pushing "+repo.Branch+" to origin")
		return a, tea.Batch(spinCmd, pushCmd(repo.Path, repo.Branch))

	case key.Matches(msg, shared.Keys.ContextSummary):
		spinCmd := a.startLoader(shared.OpExport, "Exporting context")
		return a, tea.Batch(spinCmd, exportContextCmd(a.cfg, 7))

	case key.Matches(msg, shared.Keys.Branch):
		repo, ok := a.dashboard.SelectedRepo()
		if !ok {
			return a, nil
		}
		return a, fetchBranchesCmd(repo.Path)

	case key.Matches(msg, shared.Keys.Down):
		a.dashboard.MoveDown()
		return a, a.maybeRefreshGraph()

	case key.Matches(msg, shared.Keys.Up):
		a.dashboard.MoveUp()
		return a, a.maybeRefreshGraph()

	case key.Matches(msg, shared.Keys.NextRepo):
		a.dashboard.NextRepo()
		return a, a.maybeRefreshGraph()

	case key.Matches(msg, shared.Keys.PrevRepo):
		a.dashboard.PrevRepo()
		return a, a.maybeRefreshGraph()

	case key.Matches(msg, shared.Keys.Stage):
		item, ok := a.dashboard.SelectedItem()
		if !ok {
			return a, nil
		}
		if item.Kind == dashboard.RepoHeader {
			return a, stageAllCmd(item.Repo.Path)
		}
		if item.Kind != dashboard.File {
			return a, nil
		}
		return a, stageFileCmd(item.Repo.Path, item.File.Path)

	case key.Matches(msg, shared.Keys.Unstage):
		item, ok := a.dashboard.SelectedItem()
		if !ok {
			return a, nil
		}
		if item.Kind == dashboard.RepoHeader {
			return a, unstageAllCmd(item.Repo.Path)
		}
		if item.Kind != dashboard.File {
			return a, nil
		}
		return a, unstageFileCmd(item.Repo.Path, item.File.Path)

	case key.Matches(msg, shared.Keys.StageAll):
		repo, ok := a.dashboard.SelectedRepo()
		if !ok {
			return a, nil
		}
		return a, stageAllCmd(repo.Path)

	case key.Matches(msg, shared.Keys.UnstageAll):
		repo, ok := a.dashboard.SelectedRepo()
		if !ok {
			return a, nil
		}
		return a, unstageAllCmd(repo.Path)

	case key.Matches(msg, shared.Keys.Diff):
		item, ok := a.dashboard.SelectedItem()
		if !ok || item.Kind != dashboard.File {
			return a, nil
		}
		return a, fetchDiffCmd(item.Repo.Path, item.File.Path, *item.File)

	case key.Matches(msg, shared.Keys.Commit):
		item, ok := a.dashboard.SelectedItem()
		if !ok {
			return a, nil
		}
		if !a.dashboard.RepoHasStagedFiles(item.RepoIndex) {
			a.setStatus("No staged files to commit")
			return a, nil
		}
		a.activeView = CommitView
		a.commitView.SetRepo(item.Repo)
		return a, nil

	case key.Matches(msg, shared.Keys.Open):
		item, ok := a.dashboard.SelectedItem()
		if !ok {
			return a, nil
		}
		if item.Kind == dashboard.RepoHeader {
			a.dashboard.ToggleCollapse()
			return a, a.maybeRefreshGraph()
		}
		if item.Kind == dashboard.DocHeader {
			a.dashboard.ToggleDocsCollapse()
			return a, nil
		}
		if item.Kind == dashboard.FolderHeader {
			a.dashboard.ToggleFolderCollapse()
			return a, nil
		}
		if item.Kind != dashboard.File {
			return a, nil
		}
		return a, nvim.OpenFile(item.Repo.Path, item.File.Path)
	}

	return a, nil
}

func (a App) handleDiffKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.Quit), key.Matches(msg, shared.Keys.Escape):
		return a, func() tea.Msg { return shared.CloseDiffMsg{} }

	case key.Matches(msg, shared.Keys.Stage):
		item, ok := a.dashboard.SelectedItem()
		if !ok || item.Kind != dashboard.File {
			return a, nil
		}
		return a, stageFileCmd(item.Repo.Path, item.File.Path)

	case key.Matches(msg, shared.Keys.Unstage):
		item, ok := a.dashboard.SelectedItem()
		if !ok || item.Kind != dashboard.File {
			return a, nil
		}
		return a, unstageFileCmd(item.Repo.Path, item.File.Path)
	}

	// Pass through to viewport for scrolling
	var cmd tea.Cmd
	a.diffView, cmd = a.diffView.Update(msg)
	return a, cmd
}

func (a App) handleCommitKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, shared.Keys.Escape):
		return a, func() tea.Msg { return shared.CloseCommitMsg{} }

	case key.Matches(msg, shared.Keys.AmendToggle):
		repo, ok := a.dashboard.SelectedRepo()
		if !ok {
			return a, nil
		}
		a.commitView.ToggleAmend()
		if a.commitView.IsAmend() {
			// Pre-fill with last commit message
			lastMsg, err := git.LastCommitMessage(repo.Path)
			if err == nil {
				a.commitView.SetAmendMessage(lastMsg)
			}
		}
		return a, nil

	case key.Matches(msg, shared.Keys.GenerateMsg):
		repo, ok := a.dashboard.SelectedRepo()
		if !ok {
			return a, nil
		}
		a.commitView.SetGenerating(true)
		spinCmd := a.startLoader(shared.OpGenerate, "Generating commit message")
		return a, tea.Batch(spinCmd, generateCommitMsgCmd(repo.Path))

	case msg.Type == tea.KeyEnter:
		message := a.commitView.Value()
		if message == "" {
			return a, nil
		}
		repo, ok := a.dashboard.SelectedRepo()
		if !ok {
			return a, nil
		}
		if a.commitView.IsAmend() {
			return a, amendCmd(repo.Path, message)
		}
		return a, commitCmd(repo.Path, message)
	}

	// Pass through to text input
	var cmd tea.Cmd
	a.commitView, cmd = a.commitView.Update(msg)
	return a, cmd
}

func (a App) handleBranchPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	result := a.branchPicker.HandleKey(msg)
	switch result.Action {
	case branchpicker.ActionClose:
		return a, func() tea.Msg { return shared.CloseBranchPickerMsg{} }
	case branchpicker.ActionSwitch:
		repo, ok := a.dashboard.SelectedRepo()
		if !ok {
			return a, nil
		}
		return a, switchBranchCmd(repo.Path, result.BranchName)
	case branchpicker.ActionCreate:
		repo, ok := a.dashboard.SelectedRepo()
		if !ok {
			return a, nil
		}
		return a, createBranchCmd(repo.Path, result.BranchName)
	}
	return a, nil
}

func (a App) View() string {
	if a.showHelp {
		return a.helpView.View()
	}

	// Fatal overlay takes over the entire screen
	if a.feedback != nil && a.feedback.Level == shared.FeedbackFatal {
		return a.renderFatalOverlay("")
	}

	var view string

	contentH := a.height - 1 // reserve 1 for status bar
	if contentH < 1 {
		contentH = 1
	}

	switch a.activeView {
	case DashboardView:
		view = a.renderDashboardLayout(contentH)
		view += a.renderStatusBar()
		if a.featureLinker.IsVisible() {
			view = a.featureLinker.ViewOverlay(view, a.width, a.height)
		}
	case BranchPickerView:
		view = a.renderDashboardLayout(contentH)
		view += a.renderStatusBar()
		view = a.branchPicker.ViewOverlay(view, a.width, a.height)
	case DiffView:
		view = a.diffView.View()
	case CommitView:
		view = a.commitView.View()
	}

	return view
}

func (a *App) layoutSizes() {
	contentH := a.height - 1 // 1 for status bar
	if contentH < 3 {
		contentH = 3
	}

	if a.showGraph && a.width > 80 {
		// 3-column layout: dashboard | graph | conductor
		dashPct := a.cfg.ResolvedDashboardWidth()
		dashW := a.width * dashPct / 100
		conductorW := a.width * 30 / 100
		graphW := a.width - dashW - conductorW
		if graphW < 20 {
			graphW = 20
		}
		a.dashboard.SetSize(dashW, contentH)
		a.graphPane.SetSize(graphW-1, contentH)      // -1 for left border
		a.conductorPane.SetSize(conductorW-1, contentH) // -1 for left border
	} else if a.showGraph && a.width > 40 {
		// 2-column fallback for narrower terminals
		graphW := a.width / 2
		dashW := a.width - graphW
		a.dashboard.SetSize(dashW, contentH)
		a.graphPane.SetSize(graphW-1, contentH)
	} else {
		a.dashboard.SetSize(a.width, contentH)
	}
}

func (a *App) maybeRefreshGraph() tea.Cmd {
	if !a.showGraph {
		return nil
	}
	repo, ok := a.dashboard.SelectedRepo()
	if !ok {
		return nil
	}
	var cmds []tea.Cmd
	if repo.Path != a.graphRepo {
		a.graphRepo = repo.Path
		maxCommits := a.cfg.ResolvedGraphMaxCommits()
		cmds = append(cmds, fetchGraphCmd(repo.Path, maxCommits))
	}
	if repo.Path != a.conductorRepo {
		a.conductorRepo = repo.Path
		cmds = append(cmds, fetchConductorCmd(repo.Path))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (a App) renderStatusBar() string {
	name := a.cfg.WorkspaceName()

	// Show current branch if available
	parts := []string{name}
	if repo, ok := a.dashboard.SelectedRepo(); ok && repo.Branch != "" {
		parts = append(parts, repo.Branch)
	}

	status := strings.Join(parts, " │ ")

	// Show active spinners in status bar
	for op, s := range a.spinners {
		label := a.spinnerLabels[op]
		status += " │ " + s.View() + " " + label
	}

	// Show feedback or legacy status
	if a.feedback != nil {
		var styledMsg string
		switch a.feedback.Level {
		case shared.FeedbackSuccess:
			styledMsg = shared.FeedbackSuccessStyle.Render(a.feedback.Message)
		case shared.FeedbackWarning:
			styledMsg = shared.FeedbackWarningStyle.Render(a.feedback.Message)
		case shared.FeedbackError:
			styledMsg = shared.FeedbackErrorStyle.Render(a.feedback.Message)
		default:
			styledMsg = a.feedback.Message
		}
		status += " │ " + styledMsg
	} else if a.statusMsg != "" {
		status += " │ " + a.statusMsg
	}

	// Conductor summary in status bar
	if repo, ok := a.dashboard.SelectedRepo(); ok {
		if data, exists := a.conductorData[repo.Path]; exists && data != nil {
			status += " │ " + shared.ConductorPassedBadge.Render(fmt.Sprintf("%d/%d", data.Passed, data.Total))
			if data.Session != nil {
				status += " " + shared.CommitDetailDateStyle.Render(fmt.Sprintf("#%d", data.Session.Number))
			}
			if len(data.Quality) > 0 {
				status += " " + shared.ConductorQualityBadge.Render(fmt.Sprintf("\u26a0%d", len(data.Quality)))
			}
		}
	}

	status += " │ ? for help"

	return "\n" + shared.StatusBarStyle.Width(a.width).Render(status)
}

func (a App) renderFatalOverlay(base string) string {
	if a.feedback == nil || a.feedback.Level != shared.FeedbackFatal {
		return base
	}

	content := shared.FeedbackErrorStyle.Render("ERROR: "+a.feedback.Message) + "\n"
	if a.feedback.Detail != "" {
		content += "\n" + a.feedback.Detail + "\n"
	}
	content += "\n" + shared.HelpDescStyle.Render("Press any key to dismiss")

	overlay := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#ff8080")).
		Padding(1, 2).
		Width(a.width - 10).
		Render(content)

	// Center the overlay
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, overlay)
}

func (a App) renderDashboardLayout(contentH int) string {
	dashView := a.dashboard.View()

	if a.showGraph && a.width > 80 {
		// 3-column layout
		dashPct := a.cfg.ResolvedDashboardWidth()
		dashW := a.width * dashPct / 100
		conductorW := a.width * 30 / 100
		dashView = lipgloss.NewStyle().Width(dashW).Height(contentH).MaxHeight(contentH).Render(dashView)

		var graphView string
		if a.focusPanel == FocusGraph {
			graphView = a.graphPane.ViewFocused()
		} else {
			graphView = a.graphPane.View()
		}

		var condView string
		if a.focusPanel == FocusConductor {
			condView = a.conductorPane.ViewFocused()
		} else {
			condView = a.conductorPane.View()
		}
		_ = conductorW // used in layoutSizes

		return lipgloss.JoinHorizontal(lipgloss.Top, dashView, graphView, condView)
	}

	if a.showGraph && a.width > 40 {
		// 2-column fallback
		dashW := a.width - a.width/2
		dashView = lipgloss.NewStyle().Width(dashW).Height(contentH).MaxHeight(contentH).Render(dashView)

		var graphView string
		if a.focusPanel == FocusGraph {
			graphView = a.graphPane.ViewFocused()
		} else {
			graphView = a.graphPane.View()
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, dashView, graphView)
	}

	return dashView
}

func (a *App) maybeRefreshConductor() tea.Cmd {
	if !a.showGraph {
		return nil
	}
	repo, ok := a.dashboard.SelectedRepo()
	if !ok {
		return nil
	}
	if repo.Path == a.conductorRepo {
		return nil
	}
	a.conductorRepo = repo.Path
	return fetchConductorCmd(repo.Path)
}

func fetchConductorCmd(repoPath string) tea.Cmd {
	return func() tea.Msg {
		db, err := conductor.Open(repoPath)
		if err != nil {
			return shared.ConductorRefreshedMsg{RepoPath: repoPath, Err: err}
		}
		if db == nil {
			return shared.ConductorRefreshedMsg{RepoPath: repoPath}
		}
		data, err := db.GetAllData()
		if err != nil {
			return shared.ConductorRefreshedMsg{RepoPath: repoPath, Err: err}
		}
		return conductorDataMsg{RepoPath: repoPath, Data: data}
	}
}

type conductorDataMsg struct {
	RepoPath string
	Data     *conductor.ConductorData
}

func refreshConductorCmd(repoPath string) tea.Cmd {
	return func() tea.Msg {
		db, err := conductor.Open(repoPath)
		if err != nil || db == nil {
			return conductorDataMsg{RepoPath: repoPath}
		}
		data, _ := db.GetAllData()
		return conductorDataMsg{RepoPath: repoPath, Data: data}
	}
}

func matchFeaturesCmd(repoPath, commitHash, commitMsg string, changedFiles []string) tea.Cmd {
	return func() tea.Msg {
		db, err := conductor.Open(repoPath)
		if err != nil || db == nil {
			return featureMatchMsg{}
		}
		matches, _ := db.MatchFeature(commitMsg, changedFiles)
		return featureMatchMsg{
			RepoPath:   repoPath,
			Matches:    matches,
			CommitHash: commitHash,
			CommitMsg:  commitMsg,
		}
	}
}

type featureMatchMsg struct {
	RepoPath   string
	Matches    []conductor.FeatureMatch
	CommitHash string
	CommitMsg  string
}

func linkFeatureCmd(repoPath, featureID, commitHash, commitMsg string, files []string) tea.Cmd {
	return func() tea.Msg {
		db, err := conductor.Open(repoPath)
		if err != nil || db == nil {
			return shared.FeatureLinkedMsg{Err: fmt.Errorf("no conductor db")}
		}
		err = db.RecordCommit(featureID, commitHash, commitMsg, files)
		return shared.FeatureLinkedMsg{
			FeatureID:   featureID,
			CommitHash:  commitHash,
			Description: commitMsg,
			Err:         err,
		}
	}
}

// --- Commands ---

func refreshAllStatus(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		allRepos := cfg.AllRepos()
		repos := make([]git.RepoStatus, len(allRepos))
		for i, repo := range allRepos {
			name := filepath.Base(repo.Path)
			repos[i] = git.GetRepoStatus(repo.Path, name, repo.IgnorePatterns)
		}
		return shared.StatusRefreshedMsg{Repos: repos}
	}
}

func stageFileCmd(repoPath, filePath string) tea.Cmd {
	return func() tea.Msg {
		git.StageFile(repoPath, filePath)
		return shared.FileStageToggledMsg{}
	}
}

func unstageFileCmd(repoPath, filePath string) tea.Cmd {
	return func() tea.Msg {
		git.UnstageFile(repoPath, filePath)
		return shared.FileStageToggledMsg{}
	}
}

func stageAllCmd(repoPath string) tea.Cmd {
	return func() tea.Msg {
		git.StageAll(repoPath)
		return shared.AllStagedMsg{}
	}
}

func unstageAllCmd(repoPath string) tea.Cmd {
	return func() tea.Msg {
		git.UnstageAll(repoPath)
		return shared.AllUnstagedMsg{}
	}
}

func fetchDiffCmd(repoPath, filePath string, entry git.FileEntry) tea.Cmd {
	return func() tea.Msg {
		content, err := git.GetDiffOrContent(repoPath, filePath, entry)
		return shared.DiffFetchedMsg{Content: content, File: filePath, Err: err}
	}
}

func commitCmd(repoPath, message string) tea.Cmd {
	return func() tea.Msg {
		err := git.Commit(repoPath, message)
		return shared.CommitCompleteMsg{Err: err}
	}
}

func fetchGraphCmd(repoPath string, maxCount int) tea.Cmd {
	return func() tea.Msg {
		lines, err := git.GetGraph(repoPath, maxCount)
		return shared.GraphFetchedMsg{Lines: lines, RepoPath: repoPath, Err: err}
	}
}

func fetchBranchesCmd(repoPath string) tea.Cmd {
	return func() tea.Msg {
		branches, err := git.ListBranches(repoPath)
		return shared.BranchesFetchedMsg{Branches: branches, RepoPath: repoPath, Err: err}
	}
}

func switchBranchCmd(repoPath, branchName string) tea.Cmd {
	return func() tea.Msg {
		err := git.SwitchBranch(repoPath, branchName)
		return shared.BranchSwitchedMsg{Branch: branchName, Err: err}
	}
}

func createBranchCmd(repoPath, branchName string) tea.Cmd {
	return func() tea.Msg {
		err := git.CreateBranch(repoPath, branchName)
		return shared.BranchCreatedMsg{Branch: branchName, Err: err}
	}
}

func fetchCommitDetailCmd(repoPath, hash string) tea.Cmd {
	return func() tea.Msg {
		detail, err := git.GetCommitDetail(repoPath, hash)
		return shared.CommitDetailFetchedMsg{Detail: detail, RepoPath: repoPath, Hash: hash, Err: err}
	}
}

func amendCmd(repoPath, message string) tea.Cmd {
	return func() tea.Msg {
		err := git.CommitAmend(repoPath, message)
		return shared.CommitCompleteMsg{Err: err}
	}
}

func pushCmd(repoPath, branch string) tea.Cmd {
	return func() tea.Msg {
		err := git.Push(repoPath, branch)
		return shared.PushCompleteMsg{Branch: branch, Err: err}
	}
}

func generateCommitMsgCmd(repoPath string) tea.Cmd {
	return func() tea.Msg {
		diff, err := git.RunGit(repoPath, "diff", "--cached")
		if err != nil {
			return shared.AICommitMsgMsg{Err: fmt.Errorf("getting staged diff: %w", err)}
		}
		if strings.TrimSpace(diff) == "" {
			return shared.AICommitMsgMsg{Err: fmt.Errorf("no staged changes")}
		}
		msg, err := ai.GenerateCommitMessage(diff)
		return shared.AICommitMsgMsg{Message: msg, Err: err}
	}
}

func exportContextCmd(cfg config.Config, days int) tea.Cmd {
	return func() tea.Msg {
		allRepos := cfg.AllRepos()
		contextRepos := make([]ai.ContextRepo, len(allRepos))
		for i, repo := range allRepos {
			name := filepath.Base(repo.Path)
			branch, _ := git.RunGit(repo.Path, "rev-parse", "--abbrev-ref", "HEAD")
			contextRepos[i] = ai.ContextRepo{Name: name, Path: repo.Path, Branch: strings.TrimSpace(branch)}
		}

		summary, err := ai.BuildContextSummary(contextRepos, days)
		if err != nil {
			return shared.ContextSummaryCopiedMsg{Err: err}
		}

		// Count stats from the summary comment line
		var numCommits, numRepos int
		fmt.Sscanf(summary, "<!-- %d commits across %d repos -->", &numCommits, &numRepos)

		if err := ai.CopyToClipboard(summary); err != nil {
			return shared.ContextSummaryCopiedMsg{Err: fmt.Errorf("clipboard: %w", err)}
		}

		return shared.ContextSummaryCopiedMsg{Summary: summary, NumCommits: numCommits, NumRepos: numRepos}
	}
}
