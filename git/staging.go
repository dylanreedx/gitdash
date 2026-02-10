package git

import (
	"strconv"
	"strings"
)

// GetStagedDiffStats returns per-file add/delete counts for staged changes.
func GetStagedDiffStats(repoPath string) ([]CommitFileStat, error) {
	out, err := RunGit(repoPath, "diff", "--cached", "--numstat")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	var stats []CommitFileStat
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: added<tab>deleted<tab>path
		// Binary files show "-" for both counts
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			continue
		}
		added, _ := strconv.Atoi(fields[0])
		deleted, _ := strconv.Atoi(fields[1])
		path := fields[2]
		path = resolveRenamePath(path)
		stats = append(stats, CommitFileStat{
			Path:    path,
			Added:   added,
			Deleted: deleted,
		})
	}
	return stats, nil
}

func StageFile(repoPath, filePath string) error {
	_, err := RunGit(repoPath, "add", "--", filePath)
	return err
}

func UnstageFile(repoPath, filePath string) error {
	_, err := RunGit(repoPath, "restore", "--staged", "--", filePath)
	return err
}

func StageAll(repoPath string) error {
	_, err := RunGit(repoPath, "add", "-A")
	return err
}

func UnstageAll(repoPath string) error {
	_, err := RunGit(repoPath, "reset", "HEAD")
	return err
}
