package git

func Commit(repoPath, message string) error {
	_, err := RunGit(repoPath, "commit", "-m", message)
	return err
}

func CommitAmend(repoPath, message string) error {
	_, err := RunGit(repoPath, "commit", "--amend", "-m", message)
	return err
}

func LastCommitMessage(repoPath string) (string, error) {
	return RunGit(repoPath, "log", "-1", "--format=%B")
}

func UndoLastCommit(repoPath string) (string, error) {
	hash, _ := GetHeadHash(repoPath)
	_, err := RunGit(repoPath, "reset", "--soft", "HEAD~1")
	return hash, err
}
