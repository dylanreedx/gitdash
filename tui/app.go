package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dylan/gitdash/ai"
	"github.com/dylan/gitdash/config"
	"github.com/dylan/gitdash/git"
	"github.com/dylan/gitdash/nvim"
	"github.com/dylan/gitdash/tui/branchpicker"
	"github.com/dylan/gitdash/tui/commitview"
	"github.com/dylan/gitdash/tui/dashboard"
	"github.com/dylan/gitdash/tui/diffview"
	"github.com/dylan/gitdash/tui/graphpane"
	"github.com/dylan/gitdash/tui/help"
	"github.com/dylan/gitdash/tui/icons"
	"github.com/dylan/gitdash/tui/shared"
)

type ActiveView int

const (
	DashboardView ActiveView = iota
	DiffView
	CommitView
	BranchPickerView
)

type App struct {
	cfg        config.Config
	activeView ActiveView
	showHelp   bool
	statusMsg  string

	dashboard    dashboard.Model
	diffView     diffview.Model
	commitView   commitview.Model
	helpView     help.Model
	graphPane    graphpane.Model
	branchPicker branchpicker.Model

	showGraph       bool
	graphFocused    bool
	graphRepo       string // repo path of last graph fetch
	lastDetailHash  string // hash of last fetched commit detail

	width  int
	height int
}

func NewApp(cfg config.Config) App {
	shared.InitStyles(cfg.ResolvedTheme(), cfg.ResolvedGraphColors())
	icons.SetNerdFonts(cfg.Display.NerdFonts)

	gp := graphpane.New()
	gp.SetShowIcons(cfg.Display.Icons || cfg.Display.NerdFonts)

	return App{
		cfg:          cfg,
		activeView:   DashboardView,
		dashboard:    dashboard.New(cfg.ResolvedPriorityRules(), cfg.Display),
		diffView:     diffview.New(),
		commitView:   commitview.New(),
		helpView:     help.New(),
		graphPane:    gp,
		branchPicker: branchpicker.New(),
		showGraph:    cfg.ResolvedShowGraph(),
	}
}

func (a App) Init() tea.Cmd {
	return refreshAllStatus(a.cfg)
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
		return a, nil

	case shared.StatusRefreshedMsg:
		a.dashboard.SetRepos(msg.Repos)
		a.statusMsg = ""
		return a, a.maybeRefreshGraph()

	case shared.FileStageToggledMsg, shared.AllStagedMsg, shared.AllUnstagedMsg:
		return a, refreshAllStatus(a.cfg)

	case shared.DiffFetchedMsg:
		if msg.Err != nil {
			a.statusMsg = "Error: " + msg.Err.Error()
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
		a.statusMsg = "Committed successfully"
		return a, refreshAllStatus(a.cfg)

	case shared.AICommitMsgMsg:
		a.commitView.SetGenerating(false)
		if msg.Err != nil {
			a.commitView.SetError(msg.Err)
		} else {
			a.commitView.SetAIMessage(msg.Message)
		}
		return a, nil

	case shared.PushCompleteMsg:
		if msg.Err != nil {
			a.statusMsg = "Push failed: " + msg.Err.Error()
		} else {
			a.statusMsg = "Pushed " + msg.Branch + " to origin"
		}
		return a, nil

	case shared.ContextSummaryCopiedMsg:
		if msg.Err != nil {
			a.statusMsg = "Error: " + msg.Err.Error()
		} else {
			a.statusMsg = fmt.Sprintf("Context copied to clipboard (%d commits across %d repos)", msg.NumCommits, msg.NumRepos)
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
			a.statusMsg = "Error: " + msg.Err.Error()
			return a, nil
		}
		a.branchPicker.SetBranches(msg.Branches, msg.RepoPath)
		a.activeView = BranchPickerView
		return a, nil

	case shared.BranchSwitchedMsg:
		if msg.Err != nil {
			a.statusMsg = "Error: " + msg.Err.Error()
		} else {
			a.statusMsg = "Switched to " + msg.Branch
		}
		a.activeView = DashboardView
		a.graphRepo = "" // force graph refresh
		return a, refreshAllStatus(a.cfg)

	case shared.BranchCreatedMsg:
		if msg.Err != nil {
			a.statusMsg = "Error: " + msg.Err.Error()
		} else {
			a.statusMsg = "Created " + msg.Branch
		}
		a.activeView = DashboardView
		a.graphRepo = "" // force graph refresh
		return a, refreshAllStatus(a.cfg)

	case shared.CloseBranchPickerMsg:
		a.activeView = DashboardView
		return a, nil

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
	// When graph is focused, route keys to the graph pane
	if a.graphFocused {
		switch {
		case key.Matches(msg, shared.Keys.FocusLeft), key.Matches(msg, shared.Keys.Escape):
			a.graphFocused = false
			return a, nil
		case key.Matches(msg, shared.Keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, shared.Keys.ToggleGraph):
			a.showGraph = false
			a.graphFocused = false
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
		}
		return a, nil

	case key.Matches(msg, shared.Keys.ToggleGraph):
		a.showGraph = !a.showGraph
		a.graphFocused = false
		a.layoutSizes()
		if a.showGraph {
			a.graphRepo = "" // force refresh
			return a, a.maybeRefreshGraph()
		}
		return a, nil

	case key.Matches(msg, shared.Keys.Push):
		repo, ok := a.dashboard.SelectedRepo()
		if !ok {
			return a, nil
		}
		a.statusMsg = "Pushing " + repo.Branch + "..."
		return a, pushCmd(repo.Path, repo.Branch)

	case key.Matches(msg, shared.Keys.ContextSummary):
		a.statusMsg = "Exporting context..."
		return a, exportContextCmd(a.cfg, 7)

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
			a.statusMsg = "No staged files to commit"
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
		return a, generateCommitMsgCmd(repo.Path)

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

	var view string

	contentH := a.height - 1 // reserve 1 for status bar
	if contentH < 1 {
		contentH = 1
	}

	switch a.activeView {
	case DashboardView:
		dashView := a.dashboard.View()
		if a.showGraph {
			// Lock dashboard to fixed height so it doesn't shift when right pane scrolls
			dashW := a.width - a.width/2
			dashView = lipgloss.NewStyle().Width(dashW).Height(contentH).MaxHeight(contentH).Render(dashView)

			var graphView string
			if a.graphFocused {
				graphView = a.graphPane.ViewFocused()
			} else {
				graphView = a.graphPane.View()
			}
			view = lipgloss.JoinHorizontal(lipgloss.Top, dashView, graphView)
		} else {
			view = dashView
		}
		view += a.renderStatusBar()
	case BranchPickerView:
		dashView := a.dashboard.View()
		if a.showGraph {
			dashW := a.width - a.width/2
			dashView = lipgloss.NewStyle().Width(dashW).Height(contentH).MaxHeight(contentH).Render(dashView)
			view = lipgloss.JoinHorizontal(lipgloss.Top, dashView, a.graphPane.View())
		} else {
			view = dashView
		}
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

	if a.showGraph && a.width > 40 {
		// Side-by-side: dashboard left ~50%, graph right ~50%
		graphW := a.width / 2
		dashW := a.width - graphW
		a.dashboard.SetSize(dashW, contentH)
		// graphPane width accounts for left border (1 char)
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
	if repo.Path == a.graphRepo {
		return nil
	}
	a.graphRepo = repo.Path
	maxCommits := a.cfg.ResolvedGraphMaxCommits()
	return fetchGraphCmd(repo.Path, maxCommits)
}

func (a App) renderStatusBar() string {
	name := a.cfg.WorkspaceName()

	// Show current branch if available
	parts := []string{name}
	if repo, ok := a.dashboard.SelectedRepo(); ok && repo.Branch != "" {
		parts = append(parts, repo.Branch)
	}

	status := strings.Join(parts, " │ ")
	if a.statusMsg != "" {
		status += " │ " + a.statusMsg
	}
	status += " │ ? for help"

	return "\n" + shared.StatusBarStyle.Width(a.width).Render(status)
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
