package ai

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// FeatureBrief is a lightweight feature representation for AI prompting.
type FeatureBrief struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Status      string `json:"status"`
	Phase       int    `json:"phase"`
}

// SuggestFeatureLinks calls Claude CLI to rank which features a commit likely implements.
// Returns a ranked list of feature IDs, or nil,nil if Claude CLI is absent or fails.
func SuggestFeatureLinks(commitMsg string, features []FeatureBrief) ([]string, error) {
	if _, err := exec.LookPath("claude"); err != nil {
		return nil, nil
	}

	if len(features) == 0 {
		return nil, nil
	}

	featJSON, err := json.Marshal(features)
	if err != nil {
		return nil, nil
	}

	prompt := fmt.Sprintf(
		"Given this commit message:\n%s\n\n"+
			"And these project features:\n%s\n\n"+
			"Return a JSON array of feature IDs that this commit most likely implements, "+
			"ranked by relevance (most relevant first). Only include features that are genuinely related. "+
			"Return only the JSON array, no explanation.",
		commitMsg, string(featJSON))

	cmd := exec.Command("claude", "--print", "-p", prompt)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, nil // graceful degradation
	}

	result := strings.TrimSpace(string(out))
	result = stripCodeFences(result)

	var ids []string
	if err := json.Unmarshal([]byte(result), &ids); err != nil {
		return nil, nil
	}

	return ids, nil
}
