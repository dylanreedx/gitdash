package shared

import "github.com/dylan/gitdash/git"

type StatusRefreshedMsg struct {
	Repos []git.RepoStatus
}

type FileStageToggledMsg struct{}
type AllStagedMsg struct{}
type AllUnstagedMsg struct{}

type DiffFetchedMsg struct {
	Content string
	File    string
	Err     error
}

type CommitCompleteMsg struct {
	Err error
}

type CloseDiffMsg struct{}
type CloseCommitMsg struct{}

type GraphFetchedMsg struct {
	Lines    []git.GraphLine
	RepoPath string
	Err      error
}

type BranchesFetchedMsg struct {
	Branches []git.BranchInfo
	RepoPath string
	Err      error
}

type BranchSwitchedMsg struct {
	Branch string
	Err    error
}

type BranchCreatedMsg struct {
	Branch string
	Err    error
}

type CloseBranchPickerMsg struct{}

type CommitDetailFetchedMsg struct {
	Detail   git.CommitDetail
	RepoPath string
	Hash     string
	Err      error
}

type CommitFileDiffFetchedMsg struct {
	FilePath string
	Diff     string
	Hash     string
	Err      error
}

type AICommitMsgMsg struct {
	Message string
	Err     error
}

type ContextSummaryCopiedMsg struct {
	Summary    string
	NumCommits int
	NumRepos   int
	Err        error
}
