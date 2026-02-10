package conductor

// Feature represents a conductor feature/task.
type Feature struct {
	ID           string
	Category     string
	Description  string
	Status       string // pending, in_progress, passed, failed, blocked
	Phase        int
	AttemptCount int
	CommitHash   string
	LastError    string
}

// Session represents a conductor coding session.
type Session struct {
	ID            string
	Number        int
	Status        string // pending, active, completed
	ProgressNotes string
	StartedAt     int64
	CompletedAt   int64
}

// Handoff represents a session handoff for context transfer.
type Handoff struct {
	ID            string
	CurrentTask   string
	NextSteps     []string
	Blockers      []string
	FilesModified []string
}

// QualityReflection represents a quality tracking entry.
type QualityReflection struct {
	ID               string
	ReflectionType   string // feature_complete, session_complete, handoff
	ShortcutsTaken   []string
	TestsSkipped     []string
	KnownLimitations []string
	DeferredWork     []string
	TechnicalDebt    []string
	Resolved         bool
}

// Memory represents a saved pattern or insight.
type Memory struct {
	ID      string
	Name    string
	Content string
	Tags    []string
}

// FeatureError represents a single error encountered while implementing a feature.
type FeatureError struct {
	Error         string
	ErrorType     string // build_error, test_failure, runtime_error, blocked, other
	AttemptNumber int
}

// CommitContext holds conductor context for a specific commit.
type CommitContext struct {
	Feature  *Feature
	Session  *Session
	Errors   []FeatureError
	Memories []Memory
}

// FeatureMatch represents a scored match between a commit and a feature.
type FeatureMatch struct {
	Feature Feature
	Score   float64
}

// ConductorData holds all conductor state for a repo.
type ConductorData struct {
	Features []Feature
	Session  *Session
	Handoff  *Handoff
	Quality  []QualityReflection
	Memories []Memory
	Passed   int
	Total    int
}
