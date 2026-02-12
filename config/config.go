package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Theme      ThemeConfig       `toml:"theme"`
	Workspace  WorkspaceInfo     `toml:"workspace"`
	Projects   []ProjectConfig   `toml:"project"`
	Display    DisplayConfig     `toml:"display"`
}

type WorkspaceInfo struct {
	Name     string `toml:"name"`
	ScanRoot string `toml:"scan_root,omitempty"` // root dir for project manager fuzzy finder
}

type ProjectConfig struct {
	Name  string       `toml:"name"`
	Path  string       `toml:"path"` // project root (conductor.db lives here)
	Repos []RepoConfig `toml:"repo"`
}

type ThemeConfig struct {
	BG          string   `toml:"bg,omitempty"`
	FG          string   `toml:"fg,omitempty"`
	Accent      string   `toml:"accent,omitempty"`
	Accent2     string   `toml:"accent2,omitempty"`
	Muted       string   `toml:"muted,omitempty"`
	Dim         string   `toml:"dim,omitempty"`
	Staged      string   `toml:"staged,omitempty"`
	Unstaged    string   `toml:"unstaged,omitempty"`
	DiffAdd     string   `toml:"diff_add,omitempty"`
	DiffRemove  string   `toml:"diff_remove,omitempty"`
	DiffHunk    string   `toml:"diff_hunk,omitempty"`
	RepoHeader  string   `toml:"repo_header,omitempty"`
	Branch      string   `toml:"branch,omitempty"`
	StatusBarBG string   `toml:"status_bar_bg,omitempty"`
	StatusBarFG string   `toml:"status_bar_fg,omitempty"`
	Error       string   `toml:"error,omitempty"`
	CursorBG    string   `toml:"cursor_bg,omitempty"`
	GraphColors []string `toml:"graph_colors,omitempty"`

	// Brutalist styling
	PathDirFG          string            `toml:"path_dir_fg,omitempty"`
	PathFileFG         string            `toml:"path_file_fg,omitempty"`
	StatAddBG          string            `toml:"stat_add_bg,omitempty"`
	StatDelBG          string            `toml:"stat_del_bg,omitempty"`
	CommitDetailLabelFG string           `toml:"commit_detail_label_fg,omitempty"`
	SyncPushFG          string            `toml:"sync_push_fg,omitempty"`
	SyncPushBG          string            `toml:"sync_push_bg,omitempty"`
	SyncPullFG          string            `toml:"sync_pull_fg,omitempty"`
	SyncPullBG          string            `toml:"sync_pull_bg,omitempty"`
	SpinnerFG           string            `toml:"spinner_fg,omitempty"`
	SpinnerType         string            `toml:"spinner_type,omitempty"`
	FeedbackSuccessFG   string            `toml:"feedback_success_fg,omitempty"`
	FeedbackSuccessBG   string            `toml:"feedback_success_bg,omitempty"`
	FeedbackWarningFG   string            `toml:"feedback_warning_fg,omitempty"`
	FeedbackWarningBG   string            `toml:"feedback_warning_bg,omitempty"`
	FeedbackErrorFG     string            `toml:"feedback_error_fg,omitempty"`
	FeedbackErrorBG     string            `toml:"feedback_error_bg,omitempty"`
	FolderColors       map[string]string `toml:"folder_colors,omitempty"`
	PrefixColors       map[string]PrefixColor `toml:"prefix_colors,omitempty"`
}

type PrefixColor struct {
	FG string `toml:"fg"`
	BG string `toml:"bg"`
}

type RepoConfig struct {
	Path           string   `toml:"path"`
	IgnorePatterns []string `toml:"ignore_patterns"`
}

type DisplayConfig struct {
	Icons           bool           `toml:"icons,omitempty"`
	NerdFonts       bool           `toml:"nerd_fonts,omitempty"`
	GroupFolders    bool           `toml:"group_folders,omitempty"`
	GroupDocs       bool           `toml:"group_docs,omitempty"`
	Priority        []PriorityRule `toml:"priority,omitempty"`
	GraphMaxCommits int            `toml:"graph_max_commits,omitempty"`
	ShowGraph       *bool          `toml:"show_graph,omitempty"`
	ShowConductor   *bool          `toml:"show_conductor,omitempty"`
	DashboardWidth  int            `toml:"dashboard_width,omitempty"` // percentage, default 25 (with conductor) or 50 (without)
}

type PriorityRule struct {
	Tier        int      `toml:"tier"`
	Extensions  []string `toml:"extensions"`
	Directories []string `toml:"directories"`
}

// DefaultPriorityRules returns the built-in 3-tier file priority rules.
func DefaultPriorityRules() []PriorityRule {
	return []PriorityRule{
		{Tier: 1, Extensions: []string{".js", ".ts", ".jsx", ".tsx", ".svelte", ".go"}, Directories: []string{"src", "lib", "components", "routes", "models", "resolvers"}},
		{Tier: 2, Extensions: []string{".json", ".toml", ".yaml", ".yml", ".css", ".scss"}},
		{Tier: 3, Extensions: []string{".md"}},
		{Tier: 3, Directories: []string{"scripts"}},
	}
}

// ResolvedPriorityRules returns config rules if set, otherwise defaults.
func (c Config) ResolvedPriorityRules() []PriorityRule {
	if len(c.Display.Priority) > 0 {
		return c.Display.Priority
	}
	return DefaultPriorityRules()
}

// ResolvedScanRoot returns the configured scan_root (with ~ expanded), or ~/Documents as default.
func (c Config) ResolvedScanRoot() string {
	root := c.Workspace.ScanRoot
	if root != "" {
		if strings.HasPrefix(root, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				return filepath.Join(home, root[2:])
			}
		}
		return root
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Documents")
	}
	return "."
}

// DefaultConfigPath returns ~/.config/gitdash/config.toml.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.toml"
	}
	return filepath.Join(home, ".config", "gitdash", "config.toml")
}

func Load(path string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	configDir := filepath.Dir(path)
	absConfigDir, err := filepath.Abs(configDir)
	if err != nil {
		return cfg, fmt.Errorf("resolving config directory: %w", err)
	}

	for pi := range cfg.Projects {
		proj := &cfg.Projects[pi]

		// Resolve project path
		if proj.Path != "" {
			if strings.HasPrefix(proj.Path, "~/") {
				if home, err := os.UserHomeDir(); err == nil {
					proj.Path = filepath.Join(home, proj.Path[2:])
				}
			}
			if !filepath.IsAbs(proj.Path) {
				proj.Path = filepath.Join(absConfigDir, proj.Path)
			}
			info, err := os.Stat(proj.Path)
			if err != nil {
				return cfg, fmt.Errorf("project path %q: %w", proj.Path, err)
			}
			if !info.IsDir() {
				return cfg, fmt.Errorf("project path %q is not a directory", proj.Path)
			}
		}

		for ri := range proj.Repos {
			repo := &proj.Repos[ri]

			// Expand ~ prefix
			if strings.HasPrefix(repo.Path, "~/") {
				if home, err := os.UserHomeDir(); err == nil {
					repo.Path = filepath.Join(home, repo.Path[2:])
				}
			}

			// Resolve relative repo paths against project path (or config dir)
			if !filepath.IsAbs(repo.Path) {
				if proj.Path != "" {
					repo.Path = filepath.Join(proj.Path, repo.Path)
				} else {
					repo.Path = filepath.Join(absConfigDir, repo.Path)
				}
			}

			info, err := os.Stat(repo.Path)
			if err != nil {
				return cfg, fmt.Errorf("repo path %q: %w", repo.Path, err)
			}
			if !info.IsDir() {
				return cfg, fmt.Errorf("repo path %q is not a directory", repo.Path)
			}
		}
	}

	return cfg, nil
}

// AllRepos returns all repos across all projects.
func (c Config) AllRepos() []RepoConfig {
	var repos []RepoConfig
	for _, proj := range c.Projects {
		repos = append(repos, proj.Repos...)
	}
	return repos
}

// WorkspaceName returns the workspace name, or "GitDash" as fallback.
func (c Config) WorkspaceName() string {
	if c.Workspace.Name != "" {
		return c.Workspace.Name
	}
	return "GitDash"
}

// DefaultTheme returns the Vesper color palette.
func DefaultTheme() ThemeConfig {
	return ThemeConfig{
		BG:          "#101010",
		FG:          "#ffffff",
		Accent:      "#ffc799",
		Accent2:     "#99ffe4",
		Muted:       "#505050",
		Dim:         "#a0a0a0",
		Staged:      "#99ffe4",
		Unstaged:    "#ff8080",
		DiffAdd:     "#99ffe4",
		DiffRemove:  "#ff8080",
		DiffHunk:    "#ffc799",
		RepoHeader:  "#ffffff",
		Branch:      "#ffc799",
		StatusBarBG: "#1a1a1a",
		StatusBarFG: "#a0a0a0",
		Error:       "#ff8080",
		CursorBG:    "#2a2a2a",

		PathDirFG:          "#606060",
		PathFileFG:         "#ffffff",
		StatAddBG:          "#1a3a2a",
		StatDelBG:          "#3a1a1a",
		CommitDetailLabelFG: "#606060",
		SyncPushFG:          "#99ffe4",
		SyncPushBG:          "#1a2520",
		SyncPullFG:          "#ffc799",
		SyncPullBG:          "#1a1a28",
		SpinnerFG:           "#ffc799",
		SpinnerType:         "minidot",
		FeedbackSuccessFG:   "#99ffe4",
		FeedbackSuccessBG:   "#1a3a2a",
		FeedbackWarningFG:   "#ffc799",
		FeedbackWarningBG:   "#2a2215",
		FeedbackErrorFG:     "#ff8080",
		FeedbackErrorBG:     "#3a1a1a",
	}
}

// DefaultPrefixColors returns the default conventional commit prefix colors.
func DefaultPrefixColors() map[string]PrefixColor {
	return map[string]PrefixColor{
		"feat":     {FG: "#7aa2f7", BG: "#1a1b2e"},
		"fix":      {FG: "#e0af68", BG: "#2a2215"},
		"test":     {FG: "#bb9af7", BG: "#231a2e"},
		"refactor": {FG: "#73daca", BG: "#1a2825"},
		"perf":     {FG: "#d4b07b", BG: "#2a2518"},
		"chore":    {FG: "#a0a0a0", BG: "#1a1a1a"},
		"docs":     {FG: "#a0a0a0", BG: "#1a1a1a"},
		"style":    {FG: "#a0a0a0", BG: "#1a1a1a"},
		"ci":       {FG: "#a0a0a0", BG: "#1a1a1a"},
		"build":    {FG: "#a0a0a0", BG: "#1a1a1a"},
	}
}

// DefaultFolderColors returns the default folder-name → hex-color map.
func DefaultFolderColors() map[string]string {
	return map[string]string{
		"src": "#ffc799", "lib": "#ffc799", "pkg": "#ffc799",
		"cmd": "#ffc799", "internal": "#ffc799", "api": "#ffc799",
		"components": "#99ffe4", "routes": "#99ffe4",
		"test": "#cc99ff", "tests": "#cc99ff",
		"docs": "#606060", "scripts": "#606060",
	}
}

// ResolvedTheme merges config theme with defaults for any unset fields.
func (c Config) ResolvedTheme() ThemeConfig {
	d := DefaultTheme()
	t := ThemeConfig{
		BG:          pick(c.Theme.BG, d.BG),
		FG:          pick(c.Theme.FG, d.FG),
		Accent:      pick(c.Theme.Accent, d.Accent),
		Accent2:     pick(c.Theme.Accent2, d.Accent2),
		Muted:       pick(c.Theme.Muted, d.Muted),
		Dim:         pick(c.Theme.Dim, d.Dim),
		Staged:      pick(c.Theme.Staged, d.Staged),
		Unstaged:    pick(c.Theme.Unstaged, d.Unstaged),
		DiffAdd:     pick(c.Theme.DiffAdd, d.DiffAdd),
		DiffRemove:  pick(c.Theme.DiffRemove, d.DiffRemove),
		DiffHunk:    pick(c.Theme.DiffHunk, d.DiffHunk),
		RepoHeader:  pick(c.Theme.RepoHeader, d.RepoHeader),
		Branch:      pick(c.Theme.Branch, d.Branch),
		StatusBarBG: pick(c.Theme.StatusBarBG, d.StatusBarBG),
		StatusBarFG: pick(c.Theme.StatusBarFG, d.StatusBarFG),
		Error:       pick(c.Theme.Error, d.Error),
		CursorBG:    pick(c.Theme.CursorBG, d.CursorBG),

		PathDirFG:          pick(c.Theme.PathDirFG, d.PathDirFG),
		PathFileFG:         pick(c.Theme.PathFileFG, d.PathFileFG),
		StatAddBG:          pick(c.Theme.StatAddBG, d.StatAddBG),
		StatDelBG:          pick(c.Theme.StatDelBG, d.StatDelBG),
		CommitDetailLabelFG: pick(c.Theme.CommitDetailLabelFG, d.CommitDetailLabelFG),
		SyncPushFG:          pick(c.Theme.SyncPushFG, d.SyncPushFG),
		SyncPushBG:          pick(c.Theme.SyncPushBG, d.SyncPushBG),
		SyncPullFG:          pick(c.Theme.SyncPullFG, d.SyncPullFG),
		SyncPullBG:          pick(c.Theme.SyncPullBG, d.SyncPullBG),
		SpinnerFG:           pick(c.Theme.SpinnerFG, d.SpinnerFG),
		SpinnerType:         pick(c.Theme.SpinnerType, d.SpinnerType),
		FeedbackSuccessFG:   pick(c.Theme.FeedbackSuccessFG, d.FeedbackSuccessFG),
		FeedbackSuccessBG:   pick(c.Theme.FeedbackSuccessBG, d.FeedbackSuccessBG),
		FeedbackWarningFG:   pick(c.Theme.FeedbackWarningFG, d.FeedbackWarningFG),
		FeedbackWarningBG:   pick(c.Theme.FeedbackWarningBG, d.FeedbackWarningBG),
		FeedbackErrorFG:     pick(c.Theme.FeedbackErrorFG, d.FeedbackErrorFG),
		FeedbackErrorBG:     pick(c.Theme.FeedbackErrorBG, d.FeedbackErrorBG),
	}

	// Merge folder colors: defaults first, then config overrides per-key
	t.FolderColors = DefaultFolderColors()
	for k, v := range c.Theme.FolderColors {
		t.FolderColors[k] = v
	}

	// Merge prefix colors: defaults first, then config overrides per-key
	t.PrefixColors = DefaultPrefixColors()
	for k, v := range c.Theme.PrefixColors {
		t.PrefixColors[k] = v
	}

	return t
}

// DefaultGraphColors returns the default 6-color rotating palette for git graph lines.
func DefaultGraphColors() []string {
	return []string{"#6699ff", "#ffc799", "#ff99cc", "#99ffe4", "#cc99ff", "#ffff99"}
}

// ResolvedGraphColors returns config graph colors if set, otherwise defaults.
func (c Config) ResolvedGraphColors() []string {
	if len(c.Theme.GraphColors) > 0 {
		return c.Theme.GraphColors
	}
	return DefaultGraphColors()
}

// ResolvedGraphMaxCommits returns the configured max commits or 50 as default.
func (c Config) ResolvedGraphMaxCommits() int {
	if c.Display.GraphMaxCommits > 0 {
		return c.Display.GraphMaxCommits
	}
	return 50
}

// ResolvedShowGraph returns the configured show_graph or true as default.
func (c Config) ResolvedShowGraph() bool {
	if c.Display.ShowGraph != nil {
		return *c.Display.ShowGraph
	}
	return true
}

// ResolvedShowConductor returns the configured show_conductor or false as default.
func (c Config) ResolvedShowConductor() bool {
	if c.Display.ShowConductor != nil {
		return *c.Display.ShowConductor
	}
	return false
}

// ResolvedDashboardWidth returns the configured dashboard width percentage or 25 as default.
func (c Config) ResolvedDashboardWidth() int {
	if c.Display.DashboardWidth > 0 && c.Display.DashboardWidth < 80 {
		return c.Display.DashboardWidth
	}
	return 25
}

func pick(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// saveable types for writing config back to TOML without internal fields

type saveableConfig struct {
	Theme     ThemeConfig       `toml:"theme,omitempty"`
	Workspace WorkspaceInfo     `toml:"workspace"`
	Projects  []saveableProject `toml:"project,omitempty"`
	Display   DisplayConfig     `toml:"display,omitempty"`
}

type saveableProject struct {
	Name  string         `toml:"name"`
	Path  string         `toml:"path,omitempty"`
	Repos []saveableRepo `toml:"repo,omitempty"`
}

type saveableRepo struct {
	Path           string   `toml:"path"`
	IgnorePatterns []string `toml:"ignore_patterns,omitempty"`
}

// Save writes the config back to a TOML file, converting absolute paths to relative.
func Save(path string, cfg Config) error {
	configDir := filepath.Dir(path)
	absConfigDir, err := filepath.Abs(configDir)
	if err != nil {
		return fmt.Errorf("resolving config directory: %w", err)
	}

	sc := saveableConfig{
		Theme:     cfg.Theme,
		Workspace: cfg.Workspace,
		Display:   cfg.Display,
	}

	for _, proj := range cfg.Projects {
		sp := saveableProject{
			Name: proj.Name,
		}

		// Convert project path to relative
		if proj.Path != "" {
			rel, err := filepath.Rel(absConfigDir, proj.Path)
			if err == nil {
				sp.Path = rel
			} else {
				sp.Path = proj.Path
			}
		}

		for _, repo := range proj.Repos {
			sr := saveableRepo{
				IgnorePatterns: repo.IgnorePatterns,
			}

			// Convert repo path to relative (against project path if set, else config dir)
			base := absConfigDir
			if proj.Path != "" {
				base = proj.Path
			}
			rel, err := filepath.Rel(base, repo.Path)
			if err == nil {
				sr.Path = rel
			} else {
				sr.Path = repo.Path
			}

			sp.Repos = append(sp.Repos, sr)
		}

		sc.Projects = append(sc.Projects, sp)
	}

	data, err := toml.Marshal(sc)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}
