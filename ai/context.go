package ai

import (
	"fmt"
	"strings"

	"github.com/dylan/gitdash/git"
)

type ContextRepo struct {
	Name   string
	Path   string
	Branch string
}

func BuildContextSummary(repos []ContextRepo, days int) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "# Development Context (last %d days)\n\n", days)

	totalCommits := 0
	reposWithCommits := 0

	for _, repo := range repos {
		commits, err := git.GetRecentCommits(repo.Path, days)
		if err != nil {
			continue
		}
		if len(commits) == 0 {
			continue
		}
		reposWithCommits++
		totalCommits += len(commits)

		fmt.Fprintf(&b, "## %s (%s)\n", repo.Name, repo.Branch)
		for _, c := range commits {
			fmt.Fprintf(&b, "- %s %s (%d files) - %s\n", c.Hash, c.Message, c.FilesChanged, c.RelativeDate)
		}
		b.WriteString("\n")
	}

	if totalCommits == 0 {
		return "", fmt.Errorf("no commits found in the last %d days", days)
	}

	summary := fmt.Sprintf("<!-- %d commits across %d repos -->\n", totalCommits, reposWithCommits)
	return summary + b.String(), nil
}
