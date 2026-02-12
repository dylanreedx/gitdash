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
	"github.com/dylan/gitdash/tui/projectmanager"
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
	ProjectManagerView
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
	configPath string
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
	projectManager projectmanager.Model

	showGraph       bool
	showConductor   bool
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

func NewApp(cfg config.Config, configPath string) App {
	shared.InitStyles(cfg.ResolvedTheme(), cfg.ResolvedGraphColors())
	icons.SetNerdFonts(cfg.Display.NerdFonts)

	gp := graphpane.New()
	gp.SetShowIcons(cfg.Display.Icons || cfg.Display.NerdFonts)

	dash := dashboard.New(cfg.ResolvedPriorityRules(), cfg.Display)
	dash.SetProjects(cfg.Projects)

	return App{
		cfg:            cfg,
		configPath:     configPath,
		activeView:     DashboardView,
		dashboard:      dash,
		diffView:       diffview.New(),
		commitView:     commitview.New(),
		helpView:       help.New(),
		graphPane:      gp,
		branchPicker:   branchpicker.New(),
		conductorPane:  conductorpane.New(),
		featureLinker:  featurelinker.New(),
		projectManager: projectmanager.New(filepath.Dir(configPath), cfg.ResolvedScanRoot()),
		showGraph:      cfg.ResolvedShowGraph(),
		showConductor:  cfg.ResolvedShowConductor(),
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
		a.projectManager.SetSize(msg.Width, msg.Height)
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
		if s, ok := a.spinners[shared.OpAISuggest]; ok {
			a.featureLinker.SetAISpinner(s.View())
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
		// Try to match commit to conductor feature using project-aware path
		if repo, ok := a.dashboard.SelectedRepo(); ok {
			commitMsg := a.commitView.Value()
			conductorPath := a.conductorPathForActiveProject(repo.Path)
			cmds = append(cmds, matchFeaturesCmd(conductorPath, msg.Hash, commitMsg, nil))
		}
		return a, tea.Batch(cmds...)

	case shared.CommitContextFetchedMsg:
		if msg.Err == nil {
			a.commitView.SetContextData(msg.StagedStats, msg.RecentCommits, msg.FeatureSuggestions)
		}
		return a, nil

	case shared.AICommitMsgMsg:
		a.stopLoader(shared.OpGenerate)
		a.commitView.SetGenerating(false)
		if msg.Err != nil {
			a.commitView.SetError(msg.Err)
		} else {
			a.commitView.SetAIMessage(msg.Message)
		}
		return a, nil

	case shared.UndoCommitCompleteMsg:
		if msg.Err != nil {
			a.setFeedback(shared.FeedbackError, "Undo failed: "+msg.Err.Error(), msg.Err.Error(), "")
			return a, nil
		}
		a.setFeedback(shared.FeedbackSuccess, "Undid commit "+msg.Hash+", changes staged", "", "")
		return a, refreshAllStatus(a.cfg)

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
		a.updateLinkedFeatures(msg.Data)
		// Update project conductor summary for all-projects view
		if msg.Data != nil {
			for pi, proj := range a.cfg.Projects {
				path := proj.Path
				if path == "" && len(proj.Repos) > 0 {
					path = proj.Repos[0].Path
				}
				if path == msg.RepoPath {
					summary := shared.ConductorPassedBadge.Render(fmt.Sprintf("%d/%d", msg.Data.Passed, msg.Data.Total))
					if len(msg.Data.Quality) > 0 {
						summary += " " + shared.ConductorQualityBadge.Render(fmt.Sprintf("\u26a0%d", len(msg.Data.Quality)))
					}
					a.dashboard.SetProjectConductorSummary(pi, summary)
					break
				}
			}
		}
		return a, nil

	case featureMatchMsg:
		// Show overlay even if scored matches are empty (user can search all features)
		if len(msg.Matches) > 0 || len(msg.AllFeatures) > 0 {
			a.featureLinker.Show(msg.Matches, msg.CommitHash, msg.CommitMsg,
				msg.AllFeatures, msg.ConductorData)
			// Fire async AI suggestion
			a.featureLinker.SetAIPending(true)
			spinCmd := a.startLoader(shared.OpAISuggest, "Analyzing features")
			return a, tea.Batch(spinCmd, aiSuggestFeaturesCmd(msg.CommitMsg, msg.AllFeatures))
		}
		return a, nil

	case shared.AIFeatureSuggestMsg:
		a.stopLoader(shared.OpAISuggest)
		if a.featureLinker.IsVisible() {
			a.featureLinker.SetAISuggestions(msg.RankedIDs)
		}
		return a, nil

	case shared.FeatureLinkedMsg:
		if msg.Err == nil {
			a.setFeedback(shared.FeedbackSuccess, "Linked to: "+msg.Description, "", "")
			// Refresh conductor data (will also rebuild linked features)
			if repo, ok := a.dashboard.SelectedRepo(); ok {
				a.conductorRepo = "" // force refresh
				conductorPath := a.conductorPathForActiveProject(repo.Path)
				return a, refreshConductorCmd(conductorPath)
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
			// Fetch conductor context for this commit
			if a.conductorRepo != "" {
				return a, fetchCommitContextCmd(a.conductorRepo, msg.Hash)
			}
		}
		return a, nil

	case commitContextMsg:
		a.graphPane.SetCommitContext(msg.Context)
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
			// Refresh conductor data on the same tick (project-aware)
			if a.conductorRepo != "" {
				cmds = append(cmds, refreshConductorCmd(a.conductorRepo))
			} else if repo, ok := a.dashboard.SelectedRepo(); ok {
				conductorPath := a.conductorPathForActiveProject(repo.Path)
				cmds = append(cmds, refreshConductorCmd(conductorPath))
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
	case ProjectManagerView:
		var cmd tea.Cmd
		a.projectManager, cmd = a.projectManager.Update(msg)
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
	case ProjectManagerView:
		return a.handleProjectManagerKey(msg)
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
			a.stopLoader(shared.OpAISuggest)
			if result.Feature != nil {
				if repo, ok := a.dashboard.SelectedRepo(); ok {
					conductorPath := a.conductorPathForActiveProject(repo.Path)
					return a, linkFeatureCmd(conductorPath, result.Feature.Feature.ID,
						a.featureLinker.CommitHash(), a.featureLinker.CommitMsg(), nil)
				}
			}
			return a, nil
		case featurelinker.ActionSkip:
			a.featureLinker.Hide()
			a.stopLoader(shared.OpAISuggest)
			return a, nil
		}
		// In search mode, forward non-navigation keys to textinput
		if a.featureLinker.InSearchMode() {
			var cmd tea.Cmd
			a.featureLinker, cmd = a.featureLinker.Update(msg)
			return a, cmd
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
		case key.Matches(msg, shared.Keys.ToggleConductor):
			a.showConductor = false
			a.focusPanel = FocusDashboard
			a.graphFocused = false
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
			if a.showGraph && a.showConductor && a.width > 80 {
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
		case key.Matches(msg, shared.Keys.ToggleConductor):
			a.showConductor = !a.showConductor
			a.layoutSizes()
			if a.showConductor {
				a.conductorRepo = ""
				return a, a.maybeRefreshConductor()
			}
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

	// All-projects mode: limited key set
	if a.dashboard.ActiveProject() == -1 && len(a.cfg.Projects) > 0 {
		switch {
		case key.Matches(msg, shared.Keys.Quit):
			return a, tea.Quit

		case key.Matches(msg, shared.Keys.Down):
			a.dashboard.MoveDown()
			return a, a.maybeRefreshGraph()

		case key.Matches(msg, shared.Keys.Up):
			a.dashboard.MoveUp()
			return a, a.maybeRefreshGraph()

		case key.Matches(msg, shared.Keys.Open):
			a.dashboard.EnterProject()
			a.graphRepo = ""     // force refresh
			a.conductorRepo = "" // force refresh
			return a, a.maybeRefreshGraph()

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
				a.graphRepo = ""
				a.conductorRepo = ""
				cmds := []tea.Cmd{a.maybeRefreshGraph(), a.maybeRefreshConductor()}
				return a, tea.Batch(cmds...)
			}
			return a, nil

		case key.Matches(msg, shared.Keys.ToggleConductor):
			a.showConductor = !a.showConductor
			if !a.showConductor && a.focusPanel == FocusConductor {
				a.focusPanel = FocusDashboard
				a.graphFocused = false
			}
			a.layoutSizes()
			if a.showConductor {
				a.conductorRepo = ""
				return a, a.maybeRefreshConductor()
			}
			return a, nil

		case key.Matches(msg, shared.Keys.ContextSummary):
			spinCmd := a.startLoader(shared.OpExport, "Exporting context")
			return a, tea.Batch(spinCmd, exportContextCmd(a.cfg, 7))

		case key.Matches(msg, shared.Keys.ProjectManager):
			a.projectManager.SetSize(a.width, a.height)
			a.projectManager.SetProjects(a.cfg.Projects)
			a.activeView = ProjectManagerView
			return a, nil
		}

		return a, nil
	}

	// Project-detail mode (or no projects configured)
	switch {
	case key.Matches(msg, shared.Keys.Quit):
		return a, tea.Quit

	case key.Matches(msg, shared.Keys.Escape):
		// If inside a project, go back to all-projects view
		if a.dashboard.ActiveProject() >= 0 {
			a.dashboard.ExitProject()
			a.graphRepo = ""     // force refresh
			a.conductorRepo = "" // force refresh
			return a, a.maybeRefreshGraph()
		}
		return a, nil

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

	case key.Matches(msg, shared.Keys.ToggleConductor):
		a.showConductor = !a.showConductor
		// If conductor was focused and is now hidden, return focus to dashboard
		if !a.showConductor && a.focusPanel == FocusConductor {
			a.focusPanel = FocusDashboard
			a.graphFocused = false
		}
		a.layoutSizes()
		if a.showConductor {
			a.conductorRepo = "" // force refresh
			return a, a.maybeRefreshConductor()
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

	case key.Matches(msg, shared.Keys.UndoCommit):
		repo, ok := a.dashboard.SelectedRepo()
		if !ok {
			return a, nil
		}
		return a, undoCommitCmd(repo.Path)

	case key.Matches(msg, shared.Keys.ContextSummary):
		spinCmd := a.startLoader(shared.OpExport, "Exporting context")
		return a, tea.Batch(spinCmd, exportContextCmd(a.cfg, 7))

	case key.Matches(msg, shared.Keys.ProjectManager):
		a.projectManager.SetSize(a.width, a.height)
		a.projectManager.SetProjects(a.cfg.Projects)
		a.activeView = ProjectManagerView
		return a, nil

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
		conductorPath := a.conductorPathForActiveProject(item.Repo.Path)
		return a, fetchCommitViewContextCmd(item.Repo.Path, conductorPath)

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

	case key.Matches(msg, shared.Keys.CycleType):
		a.commitView.CycleTypeForward()
		return a, nil

	case key.Matches(msg, shared.Keys.SubmitCommit):
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

	// Pass through to textarea (Enter inserts newlines)
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

func (a App) handleProjectManagerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If in input mode, let textinput handle the key first
	if a.projectManager.InInputMode() {
		result := a.projectManager.HandleKey(msg)
		if result.Action == projectmanager.ActionNone {
			// Forward to textinput for character input
			var cmd tea.Cmd
			a.projectManager, cmd = a.projectManager.Update(msg)
			return a, cmd
		}
		return a.processProjectManagerResult(result)
	}

	result := a.projectManager.HandleKey(msg)
	return a.processProjectManagerResult(result)
}

func (a App) processProjectManagerResult(result projectmanager.KeyResult) (tea.Model, tea.Cmd) {
	if result.Action != projectmanager.ActionClose {
		return a, nil
	}

	a.activeView = DashboardView

	if !result.Changed {
		return a, nil
	}

	// Save config, reload, and refresh
	a.cfg.Projects = result.Projects
	if err := config.Save(a.configPath, a.cfg); err != nil {
		a.setFeedback(shared.FeedbackError, "Save failed: "+err.Error(), err.Error(), "")
		return a, nil
	}

	newCfg, err := config.Load(a.configPath)
	if err != nil {
		a.setFeedback(shared.FeedbackError, "Reload failed: "+err.Error(), err.Error(), "")
		return a, nil
	}

	a.cfg = newCfg
	a.dashboard.SetProjects(a.cfg.Projects)
	a.setFeedback(shared.FeedbackSuccess, "Config saved", "", "")
	return a, refreshAllStatus(a.cfg)
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
	case ProjectManagerView:
		view = a.projectManager.View()
	}

	return view
}

func (a *App) layoutSizes() {
	contentH := a.height - 1 // 1 for status bar
	if contentH < 3 {
		contentH = 3
	}

	if a.showGraph && a.showConductor && a.width > 80 {
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
		// 2-column layout: dashboard | graph
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

	// Don't re-fetch graph while user is interacting with files section
	if a.focusPanel == FocusGraph && a.graphPane.ActiveSection() == graphpane.FilesSection {
		return nil
	}

	var cmds []tea.Cmd

	// In all-projects mode: use first repo of highlighted project for graph
	if a.dashboard.ActiveProject() == -1 && len(a.cfg.Projects) > 0 {
		item, ok := a.dashboard.SelectedItem()
		if !ok || item.Kind != dashboard.ProjectHeader {
			return nil
		}
		repo, ok := a.dashboard.FirstRepoInProject(item.ProjectIndex)
		if !ok {
			return nil
		}
		a.graphRepo = repo.Path
		maxCommits := a.cfg.ResolvedGraphMaxCommits()
		cmds = append(cmds, fetchGraphCmd(repo.Path, maxCommits))
		// Conductor: use project path if available
		conductorPath := a.conductorPathForProject(item.ProjectIndex)
		if conductorPath != a.conductorRepo {
			a.conductorRepo = conductorPath
			cmds = append(cmds, fetchConductorCmd(conductorPath))
		}
		return tea.Batch(cmds...)
	}

	// Project-detail mode: same as before
	repo, ok := a.dashboard.SelectedRepo()
	if !ok {
		return nil
	}
	a.graphRepo = repo.Path
	maxCommits := a.cfg.ResolvedGraphMaxCommits()
	cmds = append(cmds, fetchGraphCmd(repo.Path, maxCommits))

	conductorPath := a.conductorPathForActiveProject(repo.Path)
	if conductorPath != a.conductorRepo {
		a.conductorRepo = conductorPath
		cmds = append(cmds, fetchConductorCmd(conductorPath))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// conductorPathForProject returns the conductor lookup path for a given project index.
// Uses project.Path if set, otherwise falls back to first repo path.
func (a *App) conductorPathForProject(projectIndex int) string {
	if projectIndex < 0 || projectIndex >= len(a.cfg.Projects) {
		return ""
	}
	proj := a.cfg.Projects[projectIndex]
	if proj.Path != "" {
		return proj.Path
	}
	// Fallback: use first repo path
	if len(proj.Repos) > 0 {
		return proj.Repos[0].Path
	}
	return ""
}

// conductorPathForActiveProject returns the conductor lookup path based on current active project.
// When inside a project with a Path, uses that; otherwise uses the repo path.
func (a *App) conductorPathForActiveProject(repoPath string) string {
	if proj, ok := a.dashboard.ActiveProjectConfig(); ok && proj.Path != "" {
		return proj.Path
	}
	return repoPath
}

func (a App) renderStatusBar() string {
	name := a.cfg.WorkspaceName()

	// Show project name when drilled into a project
	parts := []string{name}
	if projName := a.dashboard.ProjectName(); projName != "" {
		parts = append(parts, projName)
	}

	// Show current branch if available (only in project-detail mode)
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
	conductorPath := a.conductorRepo
	if conductorPath != "" {
		if data, exists := a.conductorData[conductorPath]; exists && data != nil {
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

	if a.showGraph && a.showConductor && a.width > 80 {
		// 3-column layout: dashboard | graph | conductor
		dashPct := a.cfg.ResolvedDashboardWidth()
		dashW := a.width * dashPct / 100
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

		return lipgloss.JoinHorizontal(lipgloss.Top, dashView, graphView, condView)
	}

	if a.showGraph && a.width > 40 {
		// 2-column layout: dashboard | graph
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
	if !a.showConductor {
		return nil
	}

	var conductorPath string

	// In all-projects mode: use project path
	if a.dashboard.ActiveProject() == -1 && len(a.cfg.Projects) > 0 {
		item, ok := a.dashboard.SelectedItem()
		if !ok || item.Kind != dashboard.ProjectHeader {
			return nil
		}
		conductorPath = a.conductorPathForProject(item.ProjectIndex)
	} else {
		repo, ok := a.dashboard.SelectedRepo()
		if !ok {
			return nil
		}
		conductorPath = a.conductorPathForActiveProject(repo.Path)
	}

	if conductorPath == "" || conductorPath == a.conductorRepo {
		return nil
	}
	a.conductorRepo = conductorPath
	return fetchConductorCmd(conductorPath)
}

// updateLinkedFeatures builds a hash->description map from conductor features
// and passes it to the graph pane for display in commit detail.
func (a *App) updateLinkedFeatures(data *conductor.ConductorData) {
	lf := make(map[string]string)
	if data != nil {
		for _, f := range data.Features {
			if f.CommitHash != "" {
				lf[f.CommitHash] = f.Description
			}
		}
	}
	a.graphPane.SetLinkedFeatures(lf)
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

type commitContextMsg struct {
	Hash    string
	Context *conductor.CommitContext
}

func fetchCommitContextCmd(conductorPath, hash string) tea.Cmd {
	return func() tea.Msg {
		db, err := conductor.Open(conductorPath)
		if err != nil || db == nil {
			return commitContextMsg{Hash: hash}
		}
		ctx, _ := db.GetCommitContext(hash)
		return commitContextMsg{Hash: hash, Context: ctx}
	}
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
		data, _ := db.GetAllData()
		var allFeatures []conductor.Feature
		if data != nil {
			allFeatures = data.Features
		}
		return featureMatchMsg{
			RepoPath:      repoPath,
			Matches:       matches,
			AllFeatures:   allFeatures,
			ConductorData: data,
			CommitHash:    commitHash,
			CommitMsg:     commitMsg,
		}
	}
}

type featureMatchMsg struct {
	RepoPath      string
	Matches       []conductor.FeatureMatch
	AllFeatures   []conductor.Feature
	ConductorData *conductor.ConductorData
	CommitHash    string
	CommitMsg     string
}

func aiSuggestFeaturesCmd(commitMsg string, features []conductor.Feature) tea.Cmd {
	return func() tea.Msg {
		var briefs []ai.FeatureBrief
		for _, f := range features {
			if f.Status == "pending" || f.Status == "in_progress" || f.Status == "failed" {
				briefs = append(briefs, ai.FeatureBrief{
					ID:          f.ID,
					Description: f.Description,
					Category:    f.Category,
					Status:      f.Status,
					Phase:       f.Phase,
				})
			}
		}
		ranked, err := ai.SuggestFeatureLinks(commitMsg, briefs)
		return shared.AIFeatureSuggestMsg{RankedIDs: ranked, Err: err}
	}
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
		if err != nil {
			return shared.CommitCompleteMsg{Err: err}
		}
		hash, _ := git.GetHeadHash(repoPath)
		return shared.CommitCompleteMsg{Hash: hash}
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
		if err != nil {
			return shared.CommitCompleteMsg{Err: err}
		}
		hash, _ := git.GetHeadHash(repoPath)
		return shared.CommitCompleteMsg{Hash: hash}
	}
}

func pushCmd(repoPath, branch string) tea.Cmd {
	return func() tea.Msg {
		err := git.Push(repoPath, branch)
		return shared.PushCompleteMsg{Branch: branch, Err: err}
	}
}

func undoCommitCmd(repoPath string) tea.Cmd {
	return func() tea.Msg {
		hash, err := git.UndoLastCommit(repoPath)
		return shared.UndoCommitCompleteMsg{Hash: hash, Err: err}
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

func fetchCommitViewContextCmd(repoPath, conductorPath string) tea.Cmd {
	return func() tea.Msg {
		stats, _ := git.GetStagedDiffStats(repoPath)
		recent, _ := git.GetRecentCommitsByCount(repoPath, 5)

		var features []conductor.FeatureMatch
		db, err := conductor.Open(conductorPath)
		if err == nil && db != nil {
			// Get pending features as suggestions
			allFeatures, _ := db.GetFeatures("")
			for _, f := range allFeatures {
				if f.Status != "passed" {
					features = append(features, conductor.FeatureMatch{
						Feature: f,
						Score:   0,
					})
				}
			}
		}

		return shared.CommitContextFetchedMsg{
			StagedStats:        stats,
			RecentCommits:      recent,
			FeatureSuggestions: features,
		}
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
