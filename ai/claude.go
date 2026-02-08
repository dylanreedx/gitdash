package ai

import (
	"fmt"
	"os/exec"
	"strings"
)

func GenerateCommitMessage(diff string) (string, error) {
	cmd := exec.Command("claude", "--print", "-p",
		"Generate a concise conventional commit message for this diff. Return only the message, no explanation.")
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
	return msg, nil
}
