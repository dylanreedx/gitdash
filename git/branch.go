package git

import "strings"

type BranchInfo struct {
	Name      string
	IsCurrent bool
	Upstream  string
}

func ListBranches(repoPath string) ([]BranchInfo, error) {
	out, err := RunGit(repoPath, "branch", "--format=%(refname:short)|%(HEAD)|%(upstream:short)")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var branches []BranchInfo
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		branches = append(branches, BranchInfo{
			Name:      strings.TrimSpace(parts[0]),
			IsCurrent: strings.TrimSpace(parts[1]) == "*",
			Upstream:  strings.TrimSpace(parts[2]),
		})
	}
	return branches, nil
}

func SwitchBranch(repoPath, branchName string) error {
	_, err := RunGit(repoPath, "switch", branchName)
	return err
}

func CreateBranch(repoPath, branchName string) error {
	_, err := RunGit(repoPath, "switch", "-c", branchName)
	return err
}
