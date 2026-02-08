package shared

// LoaderOp identifies an async operation that can show a spinner.
type LoaderOp string

const (
	OpPush     LoaderOp = "push"
	OpGenerate LoaderOp = "generate"
	OpFetch    LoaderOp = "fetch"
	OpExport   LoaderOp = "export"
)

// LoaderStartMsg starts an animated spinner for an operation.
type LoaderStartMsg struct {
	Op    LoaderOp
	Label string
}

// LoaderStopMsg stops the spinner for an operation.
type LoaderStopMsg struct {
	Op LoaderOp
}
