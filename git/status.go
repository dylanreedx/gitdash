package git

import (
	"path/filepath"
	"strconv"
	"strings"
)

type FileStatus int

const (
	StatusModified  FileStatus = iota
	StatusAdded
	StatusDeleted
	StatusRenamed
	StatusCopied
	StatusUntracked
)

func (s FileStatus) String() string {
	switch s {
	case StatusModified:
		return "modified"
	case StatusAdded:
		return "added"
	case StatusDeleted:
		return "deleted"
	case StatusRenamed:
		return "renamed"
	case StatusCopied:
		return "copied"
	case StatusUntracked:
		return "untracked"
	default:
		return "unknown"
	}
}

type StagingState int

const (
	Staged   StagingState = iota
	Unstaged
)

type FileEntry struct {
	Path         string
	Status       FileStatus
	StagingState StagingState
	OrigPath     string // for renames
}

type RepoStatus struct {
	Path   string
	Name   string
	Branch string
	Files  []FileEntry
	Ahead  int
	Behind int
	Error  error
}

func GetBranch(repoPath string) (string, error) {
	return RunGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
}

func GetStatus(repoPath string, ignorePatterns []string) ([]FileEntry, error) {
	out, err := RunGit(repoPath, "status", "--porcelain", "-uall")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var entries []FileEntry
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}

		indexStatus := line[0]
		worktreeStatus := line[1]
		path := line[3:]

		var origPath string
		if arrowIdx := strings.Index(path, " -> "); arrowIdx != -1 {
			origPath = path[:arrowIdx]
			path = path[arrowIdx+4:]
		}

		if shouldIgnore(path, ignorePatterns) {
			continue
		}

		// Index (staged) changes
		if indexStatus != ' ' && indexStatus != '?' {
			status := parseStatusChar(indexStatus)
			entries = append(entries, FileEntry{
				Path:         path,
				Status:       status,
				StagingState: Staged,
				OrigPath:     origPath,
			})
		}

		// Worktree (unstaged) changes
		if worktreeStatus != ' ' {
			if worktreeStatus == '?' {
				entries = append(entries, FileEntry{
					Path:         path,
					Status:       StatusUntracked,
					StagingState: Unstaged,
				})
			} else {
				status := parseStatusChar(worktreeStatus)
				entries = append(entries, FileEntry{
					Path:         path,
					Status:       status,
					StagingState: Unstaged,
					OrigPath:     origPath,
				})
			}
		}
	}

	return entries, nil
}

func GetRepoStatus(repoPath, name string, ignorePatterns []string) RepoStatus {
	rs := RepoStatus{
		Path: repoPath,
		Name: name,
	}

	branch, err := GetBranch(repoPath)
	if err != nil {
		rs.Error = err
		return rs
	}
	rs.Branch = branch

	ahead, behind := getAheadBehind(repoPath)
	rs.Ahead = ahead
	rs.Behind = behind

	files, err := GetStatus(repoPath, ignorePatterns)
	if err != nil {
		rs.Error = err
		return rs
	}
	rs.Files = files

	return rs
}

func parseStatusChar(c byte) FileStatus {
	switch c {
	case 'M':
		return StatusModified
	case 'A':
		return StatusAdded
	case 'D':
		return StatusDeleted
	case 'R':
		return StatusRenamed
	case 'C':
		return StatusCopied
	default:
		return StatusModified
	}
}

func getAheadBehind(repoPath string) (ahead, behind int) {
	out, err := RunGit(repoPath, "rev-list", "--count", "--left-right", "@{upstream}...HEAD")
	if err != nil {
		// No upstream tracking branch (e.g. new local branch).
		// Count commits not reachable from any remote branch.
		out, err = RunGit(repoPath, "rev-list", "--count", "HEAD", "--not", "--remotes")
		if err != nil {
			return 0, 0
		}
		ahead, _ = strconv.Atoi(strings.TrimSpace(out))
		return ahead, 0
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0
	}
	behind, _ = strconv.Atoi(parts[0])
	ahead, _ = strconv.Atoi(parts[1])
	return ahead, behind
}

func shouldIgnore(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}
