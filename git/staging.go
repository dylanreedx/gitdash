package git

func StageFile(repoPath, filePath string) error {
	_, err := RunGit(repoPath, "add", "--", filePath)
	return err
}

func UnstageFile(repoPath, filePath string) error {
	_, err := RunGit(repoPath, "restore", "--staged", "--", filePath)
	return err
}

func StageAll(repoPath string) error {
	_, err := RunGit(repoPath, "add", "-A")
	return err
}

func UnstageAll(repoPath string) error {
	_, err := RunGit(repoPath, "reset", "HEAD")
	return err
}
