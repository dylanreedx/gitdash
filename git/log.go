package git

import (
	"fmt"
	"strings"
)

type GraphLine struct {
	GraphChars string
	Hash       string
	Refs       string
	Message    string
	IsCommit   bool
}

func GetGraph(repoPath string, maxCount int) ([]GraphLine, error) {
	out, err := RunGit(repoPath, "log", "--graph", "--all", "--decorate=short",
		"--color=never", fmt.Sprintf("--format=COMMIT:%%h|%%d|%%s"), fmt.Sprintf("-n%d", maxCount))
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var lines []GraphLine
	for _, raw := range strings.Split(out, "\n") {
		gl := parseLine(raw)
		lines = append(lines, gl)
	}
	return lines, nil
}

func parseLine(line string) GraphLine {
	idx := strings.Index(line, "COMMIT:")
	if idx == -1 {
		return GraphLine{GraphChars: line, IsCommit: false}
	}

	graphChars := line[:idx]
	rest := line[idx+len("COMMIT:"):]

	parts := strings.SplitN(rest, "|", 3)
	gl := GraphLine{
		GraphChars: graphChars,
		IsCommit:   true,
	}
	if len(parts) >= 1 {
		gl.Hash = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		gl.Refs = strings.TrimSpace(parts[1])
	}
	if len(parts) >= 3 {
		gl.Message = strings.TrimSpace(parts[2])
	}
	return gl
}
