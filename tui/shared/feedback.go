package shared

import "time"

// FeedbackLevel controls styling and auto-clear duration.
type FeedbackLevel int

const (
	FeedbackInfo    FeedbackLevel = iota // transient, auto-clears 4s
	FeedbackSuccess                      // green styled, auto-clears 4s
	FeedbackWarning                      // yellow, auto-clears 8s
	FeedbackError                        // red, auto-clears 12s
	FeedbackFatal                        // red modal overlay, requires keypress
)

// FeedbackTTL returns the auto-clear duration for a given level.
func FeedbackTTL(level FeedbackLevel) time.Duration {
	switch level {
	case FeedbackInfo, FeedbackSuccess:
		return 4 * time.Second
	case FeedbackWarning:
		return 8 * time.Second
	case FeedbackError:
		return 12 * time.Second
	default:
		return 0 // FeedbackFatal never auto-clears
	}
}

// Feedback represents a user-facing feedback message.
type Feedback struct {
	Level     FeedbackLevel
	Message   string
	Detail    string // full error text for modal/AI piping
	Timestamp time.Time
	Op        LoaderOp // which operation produced this
}

// FeedbackMsg delivers a feedback message to the app.
type FeedbackMsg struct {
	Feedback Feedback
}

// DismissFeedbackMsg clears the current feedback (e.g. from keypress on fatal overlay).
type DismissFeedbackMsg struct{}
