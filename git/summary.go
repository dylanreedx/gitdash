package git

import (
	"fmt"
	"strconv"
	"strings"
)

type RecentCommitInfo struct {
	Hash         string
	Author       string
	Date         string
	RelativeDate string
	Message      string
	FilesChanged int
}

func GetRecentCommits(repoPath string, days int) ([]RecentCommitInfo, error) {
	since := fmt.Sprintf("--since=%d days ago", days)
	out, err := RunGit(repoPath, "log", since, "--format=%h|%an|%ai|%ar|%s", "--shortstat")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	var commits []RecentCommitInfo
	lines := strings.Split(out, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			// This is a shortstat line, attach to previous commit
			if len(commits) > 0 {
				commits[len(commits)-1].FilesChanged = parseFilesChanged(line)
			}
			continue
		}

		commits = append(commits, RecentCommitInfo{
			Hash:         parts[0],
			Author:       parts[1],
			Date:         parts[2],
			RelativeDate: parts[3],
			Message:      parts[4],
		})
	}

	return commits, nil
}

// GetRecentCommitsByCount returns the last N commits for a repo.
func GetRecentCommitsByCount(repoPath string, count int) ([]RecentCommitInfo, error) {
	out, err := RunGit(repoPath, "log", fmt.Sprintf("-n%d", count), "--format=%h|%an|%ai|%ar|%s")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	var commits []RecentCommitInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			continue
		}
		commits = append(commits, RecentCommitInfo{
			Hash:         parts[0],
			Author:       parts[1],
			Date:         parts[2],
			RelativeDate: parts[3],
			Message:      parts[4],
		})
	}
	return commits, nil
}

func parseFilesChanged(stat string) int {
	// e.g. " 3 files changed, 10 insertions(+), 2 deletions(-)"
	parts := strings.Fields(stat)
	if len(parts) >= 1 {
		n, err := strconv.Atoi(parts[0])
		if err == nil {
			return n
		}
	}
	return 0
}
