package git

func Push(repoPath, branch string) error {
	_, err := RunGit(repoPath, "push", "-u", "origin", branch)
	return err
}
