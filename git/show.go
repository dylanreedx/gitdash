package git

import (
	"fmt"
	"strconv"
	"strings"
)

type CommitFileStat struct {
	Path    string
	Added   int
	Deleted int
}

type CommitDetail struct {
	Hash     string
	Author   string
	Date     string
	Message  string
	Files    []CommitFileStat
	TotalAdd int
	TotalDel int
}

func GetCommitDetail(repoPath, hash string) (CommitDetail, error) {
	out, err := RunGit(repoPath, "show", "--stat", "--format=%H%n%an%n%ai%n%B", hash)
	if err != nil {
		return CommitDetail{}, err
	}

	lines := strings.Split(out, "\n")
	if len(lines) < 4 {
		return CommitDetail{}, fmt.Errorf("unexpected git show output")
	}

	detail := CommitDetail{
		Hash:   lines[0],
		Author: lines[1],
		Date:   lines[2],
	}

	// Message is everything until the first blank line after the body,
	// then stat lines follow. Find the stat separator.
	// The stat block ends with a summary line like " N files changed, ..."
	// Work backwards from the end to find the stat summary line.
	statSummaryIdx := -1
	for i := len(lines) - 1; i >= 3; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if strings.Contains(trimmed, "file changed") || strings.Contains(trimmed, "files changed") {
			statSummaryIdx = i
			break
		}
	}

	if statSummaryIdx == -1 {
		// No stat block — entire remainder is message
		detail.Message = strings.TrimSpace(strings.Join(lines[3:], "\n"))
		return detail, nil
	}

	// Find where stat lines start (they come before the summary).
	// Stat lines look like: " path/to/file | N ++++---"
	statStart := statSummaryIdx
	for i := statSummaryIdx - 1; i >= 3; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			break
		}
		if strings.Contains(lines[i], "|") {
			statStart = i
		}
	}

	// Message is between line 3 and the blank line before stats
	msgEnd := statStart
	for msgEnd > 3 && strings.TrimSpace(lines[msgEnd-1]) == "" {
		msgEnd--
	}
	detail.Message = strings.TrimSpace(strings.Join(lines[3:msgEnd], "\n"))

	// Parse stat lines
	for i := statStart; i < statSummaryIdx; i++ {
		fs := parseStatLine(lines[i])
		if fs.Path != "" {
			detail.Files = append(detail.Files, fs)
			detail.TotalAdd += fs.Added
			detail.TotalDel += fs.Deleted
		}
	}

	return detail, nil
}

func parseStatLine(line string) CommitFileStat {
	// Format: " path/to/file | 5 ++---"
	// or:     " path/to/file | Bin 0 -> 1234 bytes"
	// or:     " src/{old => new}/file.go | 5 ++---"  (rename)
	parts := strings.SplitN(line, "|", 2)
	if len(parts) != 2 {
		return CommitFileStat{}
	}

	path := strings.TrimSpace(parts[0])
	stats := strings.TrimSpace(parts[1])

	// Resolve rename notation to the new path
	path = resolveRenamePath(path)

	fs := CommitFileStat{Path: path}

	// Try to parse numeric changes
	fields := strings.Fields(stats)
	if len(fields) >= 1 {
		if _, err := strconv.Atoi(fields[0]); err == nil && len(fields) >= 2 {
			changes := fields[1]
			for _, ch := range changes {
				if ch == '+' {
					fs.Added++
				} else if ch == '-' {
					fs.Deleted++
				}
			}
		}
	}

	return fs
}

// resolveRenamePath converts git's rename notation to the new path.
//
//	"src/{old => new}/file.go" → "src/new/file.go"
//	"old.go => new.go"         → "new.go"
func resolveRenamePath(path string) string {
	if braceStart := strings.Index(path, "{"); braceStart >= 0 {
		braceEnd := strings.Index(path, "}")
		if braceEnd > braceStart {
			inner := path[braceStart+1 : braceEnd]
			if arrowIdx := strings.Index(inner, " => "); arrowIdx >= 0 {
				newPart := inner[arrowIdx+4:]
				return path[:braceStart] + newPart + path[braceEnd+1:]
			}
		}
	}
	if arrowIdx := strings.Index(path, " => "); arrowIdx >= 0 {
		return strings.TrimSpace(path[arrowIdx+4:])
	}
	return path
}

func GetCommitFileDiff(repoPath, hash, file string) (string, error) {
	out, err := RunGit(repoPath, "show", "--format=", hash, "--", file)
	if err != nil {
		return "", err
	}
	// Belt-and-suspenders: if header leaked through, strip to diff start
	if idx := strings.Index(out, "diff --git"); idx > 0 {
		out = out[idx:]
	}
	return out, nil
}
