package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func GetDiff(repoPath, filePath string, staged bool) (string, error) {
	if staged {
		return RunGit(repoPath, "diff", "--cached", "--", filePath)
	}
	return RunGit(repoPath, "diff", "--", filePath)
}

func GetDiffOrContent(repoPath, filePath string, entry FileEntry) (string, error) {
	if entry.Status == StatusUntracked {
		fullPath := filepath.Join(repoPath, filePath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("reading untracked file: %w", err)
		}
		lines := strings.Split(string(data), "\n")
		var b strings.Builder
		fmt.Fprintf(&b, "--- /dev/null\n")
		fmt.Fprintf(&b, "+++ b/%s\n", filePath)
		fmt.Fprintf(&b, "@@ -0,0 +1,%d @@\n", len(lines))
		for _, line := range lines {
			fmt.Fprintf(&b, "+%s\n", line)
		}
		return b.String(), nil
	}

	return GetDiff(repoPath, filePath, entry.StagingState == Staged)
}
