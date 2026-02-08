package git

func Push(repoPath, branch string) error {
	_, err := RunGit(repoPath, "push", "origin", branch)
	return err
}
