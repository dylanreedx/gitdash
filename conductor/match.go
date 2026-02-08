package conductor

import (
	"path/filepath"
	"sort"
	"strings"
)

// MatchFeature scores features against a commit message and changed files.
// Returns a sorted list with the best match first.
func (d *DB) MatchFeature(commitMsg string, changedFiles []string) ([]FeatureMatch, error) {
	features, err := d.GetFeatures("")
	if err != nil {
		return nil, err
	}

	// Only match against active features
	var active []Feature
	for _, f := range features {
		if f.Status == "in_progress" || f.Status == "pending" || f.Status == "failed" {
			active = append(active, f)
		}
	}

	if len(active) == 0 {
		return nil, nil
	}

	msgTokens := tokenize(commitMsg)
	changedSet := make(map[string]bool)
	for _, f := range changedFiles {
		changedSet[filepath.Base(f)] = true
		changedSet[f] = true
	}

	var matches []FeatureMatch
	for _, f := range active {
		score := scoreFeature(d, f, msgTokens, changedSet, commitMsg)
		if score > 0 {
			matches = append(matches, FeatureMatch{Feature: f, Score: score})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	// Cap score at 1.0
	for i := range matches {
		if matches[i].Score > 1.0 {
			matches[i].Score = 1.0
		}
	}

	return matches, nil
}

func scoreFeature(d *DB, f Feature, msgTokens []string, changedSet map[string]bool, commitMsg string) float64 {
	var score float64

	// In-progress bonus: usually only one feature is active
	if f.Status == "in_progress" {
		score += 0.5
	}

	// Keyword match: tokenize feature description and compare
	descTokens := tokenize(f.Description)
	overlap := tokenOverlap(msgTokens, descTokens)
	score += overlap * 0.3

	// Category match: conventional commit prefix → feature category
	if categoryMatch(commitMsg, f.Category) {
		score += 0.1
	}

	// File overlap: compare changed files with prior commits/handoffs
	if len(changedSet) > 0 {
		featureFiles, _ := d.GetCommitFiles(f.ID)
		handoffFiles, _ := d.GetHandoffFiles()
		allFeatureFiles := append(featureFiles, handoffFiles...)

		if len(allFeatureFiles) > 0 {
			matchCount := 0
			for _, ff := range allFeatureFiles {
				if changedSet[ff] || changedSet[filepath.Base(ff)] {
					matchCount++
				}
			}
			if matchCount > 0 {
				ratio := float64(matchCount) / float64(len(changedSet))
				if ratio > 1 {
					ratio = 1
				}
				score += ratio * 0.2
			}
		}
	}

	return score
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	// Remove common prefixes
	for _, prefix := range []string{"feat:", "fix:", "refactor:", "chore:", "test:", "docs:", "perf:", "style:", "ci:", "build:"} {
		s = strings.TrimPrefix(s, prefix)
	}
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	// Filter stop words and short words
	var result []string
	stop := map[string]bool{"the": true, "a": true, "an": true, "and": true, "or": true,
		"to": true, "in": true, "for": true, "of": true, "with": true, "is": true, "on": true}
	for _, w := range words {
		if len(w) > 2 && !stop[w] {
			result = append(result, w)
		}
	}
	return result
}

func tokenOverlap(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	bSet := make(map[string]bool)
	for _, w := range b {
		bSet[w] = true
	}
	matches := 0
	for _, w := range a {
		if bSet[w] {
			matches++
		}
	}
	return float64(matches) / float64(len(a))
}

func categoryMatch(commitMsg, category string) bool {
	msg := strings.ToLower(commitMsg)
	cat := strings.ToLower(category)
	prefixMap := map[string][]string{
		"feat":     {"feature", "feat"},
		"fix":      {"bugfix", "fix", "bug"},
		"test":     {"test", "testing"},
		"refactor": {"refactor", "refactoring"},
		"docs":     {"docs", "documentation"},
	}
	for prefix, cats := range prefixMap {
		if strings.HasPrefix(msg, prefix+":") {
			for _, c := range cats {
				if strings.Contains(cat, c) {
					return true
				}
			}
		}
	}
	return false
}
