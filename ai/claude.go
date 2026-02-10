package ai

import (
	"fmt"
	"os/exec"
	"strings"
)

func stripCodeFences(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) >= 2 && strings.HasPrefix(lines[0], "```") && strings.HasPrefix(lines[len(lines)-1], "```") {
		return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
	}
	return s
}

func GenerateCommitMessage(diff string) (string, error) {
	cmd := exec.Command("claude", "--print", "-p",
		"Generate a short commit message for this diff. Format:\n"+
			"type(scope): subject\n\n"+
			"- point 1\n"+
			"- point 2\n\n"+
			"Keep it to 1-2 bullet points max. No prose. Return only the message.")
	cmd.Stdin = strings.NewReader(diff)

	out, err := cmd.CombinedOutput()
	if err != nil {
		if _, lookErr := exec.LookPath("claude"); lookErr != nil {
			return "", fmt.Errorf("claude CLI not found — install it to use AI features")
		}
		return "", fmt.Errorf("claude: %s: %w", strings.TrimSpace(string(out)), err)
	}

	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return "", fmt.Errorf("claude returned empty response")
	}
	// Strip markdown fences if the model wrapped the message
	msg = stripCodeFences(msg)
	return msg, nil
}
