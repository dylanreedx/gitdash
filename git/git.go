package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func RunGit(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath

	out, err := cmd.CombinedOutput()
	output := strings.TrimRight(string(out), " \t\r\n")
	if err != nil {
		return output, fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), output, err)
	}
	return output, nil
}
