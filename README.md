# GitDash

A keyboard-driven multi-repo git dashboard for the terminal. Built with [Bubbletea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), and [Bubbles](https://github.com/charmbracelet/bubbles).

Monitor staged/unstaged files across multiple repositories, view diffs, commit, manage branches, explore commit history with an ASCII graph, and generate commit messages with AI — all without leaving the terminal.

## Features

- **Multi-repo dashboard** — See file changes across all your repos at a glance
- **File staging** — Stage/unstage individual files or entire repos
- **Inline diffs** — View diffs without leaving the TUI
- **Commit** — Write and submit commit messages in-app
- **Branch management** — List, switch, and create branches with prefix suggestions (feat/, fix/, chore/, etc.)
- **Commit graph** — Side-by-side ASCII commit graph with commit details and expandable file diffs
- **AI commit messages** — Generate conventional commit messages from staged diffs via the Claude CLI
- **Context export** — Copy a markdown summary of recent commits across all repos to clipboard
- **Neovim integration** — Open files in Neovim (tmux-aware: splits pane if inside tmux)
- **File priority tiers** — Source files highlighted brighter than config files, docs dimmed
- **Folder grouping** — Collapsible folder and doc sections
- **Nerd Font icons** — Optional file/directory icons with unicode fallbacks
- **Fully themeable** — Vesper-inspired defaults, every color configurable via TOML

## Install

Requires Go 1.24+.

```bash
go build -o gitdash .
```

## Usage

```bash
# Uses ~/.config/gitdash/config.toml by default
./gitdash

# Or specify a config
./gitdash -config workspace.toml
```

### Optional dependencies

| Dependency | Purpose |
|---|---|
| [Claude CLI](https://docs.anthropic.com/en/docs/claude-cli) | AI commit message generation (`tab` in commit view) |
| Neovim | Open files with `enter` |
| tmux | Neovim opens in a split pane instead of replacing the TUI |
| Nerd Font | Richer file/directory icons |

## Keybindings

### Dashboard

| Key | Action |
|---|---|
| `j` / `k` | Move up/down (skips section headers) |
| `Tab` / `Shift+Tab` | Next/previous repo |
| `Enter` | Open file in Neovim, or toggle collapse on headers |
| `s` / `u` | Stage/unstage file |
| `S` / `U` | Stage/unstage all files in repo |
| `d` | View diff |
| `c` | Commit staged files |
| `b` | Branch picker |
| `g` | Toggle commit graph pane |
| `Ctrl+X` | Export context summary to clipboard |
| `?` | Help |
| `q` | Quit |

### Graph pane

| Key | Action |
|---|---|
| `Ctrl+L` | Focus graph pane |
| `Ctrl+H` / `Esc` | Focus dashboard |
| `Ctrl+J` / `Ctrl+K` | Switch between graph and file sections |
| `j` / `k` | Navigate commits |
| `Enter` | Toggle file diff |
| `PgUp` / `PgDn` | Scroll |

### Commit view

| Key | Action |
|---|---|
| `Tab` | Generate commit message with AI |
| `Enter` | Submit commit |
| `Esc` | Cancel |

### Diff view

| Key | Action |
|---|---|
| `j` / `k` | Scroll |
| `s` / `u` | Stage/unstage while viewing |
| `q` / `Esc` | Close |

### Branch picker

| Key | Action |
|---|---|
| `j` / `k` | Navigate branches |
| `Enter` | Switch to branch (or create in create mode) |
| `n` | New branch mode |
| `Tab` | Cycle prefix (feat/, fix/, chore/, refactor/) |
| `Esc` | Close |

## Configuration

Config is TOML. Place it at `~/.config/gitdash/config.toml` or pass `-config path/to/file.toml`.

### Minimal example

```toml
[[workspace]]
name = "my-project"

[[workspace.repo]]
path = "~/code/backend"

[[workspace.repo]]
path = "~/code/frontend"
```

### Full example

```toml
[[workspace]]
name = "dev"

[[workspace.repo]]
path = "~/code/api"
ignore_patterns = ["vendor/", "node_modules/"]

[[workspace.repo]]
path = "~/code/web"

[display]
icons = true
nerd_fonts = true
group_folders = true
group_docs = true
graph_max_commits = 50
show_graph = true

[[display.priority]]
tier = 1
extensions = [".go", ".ts", ".tsx", ".js", ".jsx", ".svelte"]
directories = ["src", "lib", "components", "routes"]

[[display.priority]]
tier = 2
extensions = [".json", ".toml", ".yaml", ".css", ".scss"]

[[display.priority]]
tier = 3
extensions = [".md"]
directories = ["scripts"]

[theme]
bg = "#101010"
fg = "#ffffff"
accent = "#ffc799"
accent2 = "#99ffe4"
muted = "#505050"
staged = "#99ffe4"
unstaged = "#ff8080"
diff_add = "#99ffe4"
diff_remove = "#ff8080"
diff_hunk = "#ffc799"
repo_header = "#ffffff"
branch = "#ffc799"
status_bar_bg = "#1a1a1a"
status_bar_fg = "#a0a0a0"
error = "#ff8080"
cursor_bg = "#2a2a2a"
graph_colors = ["#6699ff", "#ffc799", "#ff99cc", "#99ffe4", "#cc99ff", "#ffff99"]

[theme.folder_colors]
src = "#ffc799"
components = "#99ffe4"
test = "#cc99ff"
docs = "#606060"

[theme.prefix_colors.feat]
fg = "#7aa2f7"
bg = "#1a1b2e"

[theme.prefix_colors.fix]
fg = "#e0af68"
bg = "#2a2215"
```

### Config reference

**Display options**

| Field | Type | Default | Description |
|---|---|---|---|
| `icons` | bool | `false` | Show unicode file icons |
| `nerd_fonts` | bool | `false` | Use Nerd Font icons (requires a patched font) |
| `group_folders` | bool | `false` | Group files under collapsible folder headers |
| `group_docs` | bool | `false` | Group .md files under a collapsible docs section |
| `graph_max_commits` | int | `50` | Max commits shown in the graph pane |
| `show_graph` | bool | `true` | Show graph pane on startup |

**Priority rules** — Files matching tier 1 are highlighted brightest, tier 3 are dimmed. Unmatched files display normally.

**Theme** — All color values are hex strings. Unset fields fall back to the Vesper-inspired defaults. `graph_colors` is a rotating palette for branch lines. `folder_colors` maps directory names to colors. `prefix_colors` styles conventional commit prefixes (feat, fix, etc.) in the graph.

## AI Features

### Commit message generation

In the commit view, press `Tab` to generate a conventional commit message from your staged diff. Requires the [Claude CLI](https://docs.anthropic.com/en/docs/claude-cli) (`claude`) to be installed and on your PATH.

The staged diff is piped to Claude, which returns a single-line conventional commit message. The message is pre-filled into the text input — edit it if needed, then press `Enter` to commit.

### Context summary export

From the dashboard, press `Ctrl+X` to gather the last 7 days of commits across all configured repos into a markdown summary and copy it to your clipboard. Useful for pasting into a coding agent or AI assistant to give it context about recent work.

Output format:

```markdown
# Development Context (last 7 days)

## backend (main)
- abc1234 feat: add login page (3 files) - 2 days ago
- def5678 fix: resolve null check (1 file) - 3 days ago

## frontend (develop)
- 1a2b3c4 refactor: extract form component (5 files) - 1 day ago
```

## Architecture

```
main.go              Entry point, flag parsing, Bubbletea program
config/              TOML config loading and defaults
git/                 Git operations via os/exec (status, diff, staging, log, branches)
ai/                  Claude CLI wrapper, context summary builder, clipboard
nvim/                Neovim integration (tmux-aware)
tui/
  app.go             Main Bubbletea model, routing, commands
  shared/            Shared styles, keys, messages (avoids import cycles)
  dashboard/         Multi-repo file listing with priority tiers
  diffview/          Scrollable diff viewport
  commitview/        Commit message input with AI generation
  graphpane/         3-section commit graph (graph, detail, files)
  branchpicker/      Branch list/create overlay
  help/              Help overlay
  icons/             File/directory icon mappings
```
