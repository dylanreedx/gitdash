package git

func Commit(repoPath, message string) error {
	_, err := RunGit(repoPath, "commit", "-m", message)
	return err
}
