package nvim

import (
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

type EditorFinishedMsg struct {
	Err error
}

func OpenFile(repoPath, filePath string) tea.Cmd {
	fullPath := filepath.Join(repoPath, filePath)

	if os.Getenv("TMUX") != "" {
		return func() tea.Msg {
			cmd := exec.Command("tmux", "split-window", "-h",
				"-c", repoPath,
				"nvim", filePath)
			err := cmd.Run()
			return EditorFinishedMsg{Err: err}
		}
	}

	c := exec.Command("nvim", fullPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return EditorFinishedMsg{Err: err}
	})
}
