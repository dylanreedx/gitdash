package conductor

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// DB wraps a connection to a conductor SQLite database.
type DB struct {
	db *sql.DB
}

var (
	cache   = make(map[string]*DB)
	cacheMu sync.Mutex
)

// Open opens (or returns a cached) conductor database for a repo.
// Returns nil, nil if no .conductor/conductor.db exists.
func Open(repoPath string) (*DB, error) {
	dbPath := filepath.Join(repoPath, ".conductor", "conductor.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()

	if db, ok := cache[dbPath]; ok {
		return db, nil
	}

	sqlDB, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}

	db := &DB{db: sqlDB}
	cache[dbPath] = db
	return db, nil
}

// GetFeatures returns all features, optionally filtered by status.
func (d *DB) GetFeatures(status string) ([]Feature, error) {
	query := `SELECT id, category, description, status, phase, attempt_count,
	          COALESCE(commit_hash, ''), COALESCE(last_error, '')
	          FROM features WHERE 1=1`
	var args []any
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY phase ASC, category ASC`

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var features []Feature
	for rows.Next() {
		var f Feature
		if err := rows.Scan(&f.ID, &f.Category, &f.Description, &f.Status,
			&f.Phase, &f.AttemptCount, &f.CommitHash, &f.LastError); err != nil {
			return nil, err
		}
		features = append(features, f)
	}
	return features, rows.Err()
}

// GetActiveSession returns the most recent active or completed session.
func (d *DB) GetActiveSession() (*Session, error) {
	row := d.db.QueryRow(`SELECT id, session_number, status,
		COALESCE(progress_notes, ''),
		COALESCE(started_at, 0), COALESCE(completed_at, 0)
		FROM sessions ORDER BY session_number DESC LIMIT 1`)

	var s Session
	err := row.Scan(&s.ID, &s.Number, &s.Status, &s.ProgressNotes,
		&s.StartedAt, &s.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetLatestHandoff returns the most recent handoff.
func (d *DB) GetLatestHandoff() (*Handoff, error) {
	row := d.db.QueryRow(`SELECT id, COALESCE(current_task, ''),
		COALESCE(next_steps, '[]'), COALESCE(blockers, '[]'),
		COALESCE(files_modified, '[]')
		FROM handoffs ORDER BY created_at DESC LIMIT 1`)

	var h Handoff
	var nextSteps, blockers, files string
	err := row.Scan(&h.ID, &h.CurrentTask, &nextSteps, &blockers, &files)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(nextSteps), &h.NextSteps)
	json.Unmarshal([]byte(blockers), &h.Blockers)
	json.Unmarshal([]byte(files), &h.FilesModified)
	return &h, nil
}

// GetQualityIssues returns unresolved quality reflections.
func (d *DB) GetQualityIssues() ([]QualityReflection, error) {
	rows, err := d.db.Query(`SELECT id, reflection_type,
		COALESCE(shortcuts_taken, '[]'), COALESCE(tests_skipped, '[]'),
		COALESCE(known_limitations, '[]'), COALESCE(deferred_work, '[]'),
		COALESCE(technical_debt, '[]'), COALESCE(resolved, 0)
		FROM quality_reflections WHERE resolved = 0
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []QualityReflection
	for rows.Next() {
		var q QualityReflection
		var shortcuts, tests, limitations, deferred, debt string
		var resolved int
		if err := rows.Scan(&q.ID, &q.ReflectionType,
			&shortcuts, &tests, &limitations, &deferred, &debt, &resolved); err != nil {
			return nil, err
		}
		q.Resolved = resolved != 0
		json.Unmarshal([]byte(shortcuts), &q.ShortcutsTaken)
		json.Unmarshal([]byte(tests), &q.TestsSkipped)
		json.Unmarshal([]byte(limitations), &q.KnownLimitations)
		json.Unmarshal([]byte(deferred), &q.DeferredWork)
		json.Unmarshal([]byte(debt), &q.TechnicalDebt)
		results = append(results, q)
	}
	return results, rows.Err()
}

// GetRecentMemories returns the N most recent memories.
func (d *DB) GetRecentMemories(limit int) ([]Memory, error) {
	rows, err := d.db.Query(`SELECT id, name, content, COALESCE(tags, '[]')
		FROM memories ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Memory
	for rows.Next() {
		var m Memory
		var tags string
		if err := rows.Scan(&m.ID, &m.Name, &m.Content, &tags); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(tags), &m.Tags)
		results = append(results, m)
	}
	return results, rows.Err()
}

// RecordCommit writes a commit link to the conductor database.
func (d *DB) RecordCommit(featureID, hash, message string, files []string) error {
	filesJSON, _ := json.Marshal(files)

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO commits (id, feature_id, commit_hash, message, files_changed, created_at)
		VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, strftime('%s', 'now'))`,
		featureID, hash, message, string(filesJSON))
	if err != nil {
		return err
	}

	_, err = tx.Exec(`UPDATE features SET commit_hash = ? WHERE id = ?`, hash, featureID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetAllData fetches all conductor data for display.
func (d *DB) GetAllData() (*ConductorData, error) {
	features, err := d.GetFeatures("")
	if err != nil {
		return nil, err
	}

	session, err := d.GetActiveSession()
	if err != nil {
		return nil, err
	}

	handoff, err := d.GetLatestHandoff()
	if err != nil {
		return nil, err
	}

	quality, err := d.GetQualityIssues()
	if err != nil {
		return nil, err
	}

	memories, err := d.GetRecentMemories(10)
	if err != nil {
		return nil, err
	}

	var passed, total int
	for _, f := range features {
		total++
		if f.Status == "passed" {
			passed++
		}
	}

	return &ConductorData{
		Features: features,
		Session:  session,
		Handoff:  handoff,
		Quality:  quality,
		Memories: memories,
		Passed:   passed,
		Total:    total,
	}, nil
}

// GetCommitContext returns enriched conductor context for a commit hash.
// Returns nil, nil if the commit is not tracked in the conductor DB.
func (d *DB) GetCommitContext(hash string) (*CommitContext, error) {
	// Look up commit by short hash prefix match
	var featureID, sessionID sql.NullString
	err := d.db.QueryRow(`SELECT feature_id, session_id FROM commits WHERE commit_hash LIKE ?||'%' LIMIT 1`, hash).
		Scan(&featureID, &sessionID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	ctx := &CommitContext{}

	// Fetch feature
	if featureID.Valid && featureID.String != "" {
		row := d.db.QueryRow(`SELECT id, category, description, status, phase, attempt_count,
			COALESCE(commit_hash, ''), COALESCE(last_error, '')
			FROM features WHERE id = ?`, featureID.String)
		var f Feature
		if err := row.Scan(&f.ID, &f.Category, &f.Description, &f.Status,
			&f.Phase, &f.AttemptCount, &f.CommitHash, &f.LastError); err == nil {
			ctx.Feature = &f
		}

		// Fetch errors for this feature
		rows, err := d.db.Query(`SELECT error, error_type, attempt_number
			FROM feature_errors WHERE feature_id = ? ORDER BY attempt_number`, featureID.String)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var fe FeatureError
				if err := rows.Scan(&fe.Error, &fe.ErrorType, &fe.AttemptNumber); err == nil {
					ctx.Errors = append(ctx.Errors, fe)
				}
			}
		}

		// Fetch memories matching feature category
		if ctx.Feature != nil && ctx.Feature.Category != "" {
			memRows, err := d.db.Query(`SELECT id, name, content, COALESCE(tags, '[]')
				FROM memories WHERE tags LIKE '%' || ? || '%' LIMIT 3`, ctx.Feature.Category)
			if err == nil {
				defer memRows.Close()
				for memRows.Next() {
					var m Memory
					var tags string
					if err := memRows.Scan(&m.ID, &m.Name, &m.Content, &tags); err == nil {
						json.Unmarshal([]byte(tags), &m.Tags)
						ctx.Memories = append(ctx.Memories, m)
					}
				}
			}
		}
	}

	// Fetch session
	if sessionID.Valid && sessionID.String != "" {
		row := d.db.QueryRow(`SELECT id, session_number, status,
			COALESCE(progress_notes, ''),
			COALESCE(started_at, 0), COALESCE(completed_at, 0)
			FROM sessions WHERE id = ?`, sessionID.String)
		var s Session
		if err := row.Scan(&s.ID, &s.Number, &s.Status, &s.ProgressNotes,
			&s.StartedAt, &s.CompletedAt); err == nil {
			ctx.Session = &s
		}
	}

	return ctx, nil
}

// GetCommitFiles returns files_changed from prior commits on a feature.
func (d *DB) GetCommitFiles(featureID string) ([]string, error) {
	rows, err := d.db.Query(`SELECT COALESCE(files_changed, '[]')
		FROM commits WHERE feature_id = ?`, featureID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allFiles []string
	for rows.Next() {
		var filesJSON string
		if err := rows.Scan(&filesJSON); err != nil {
			continue
		}
		var files []string
		json.Unmarshal([]byte(filesJSON), &files)
		allFiles = append(allFiles, files...)
	}
	return allFiles, rows.Err()
}

// GetHandoffFiles returns files_modified from handoffs.
func (d *DB) GetHandoffFiles() ([]string, error) {
	rows, err := d.db.Query(`SELECT COALESCE(files_modified, '[]')
		FROM handoffs ORDER BY created_at DESC LIMIT 5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allFiles []string
	for rows.Next() {
		var filesJSON string
		if err := rows.Scan(&filesJSON); err != nil {
			continue
		}
		var files []string
		json.Unmarshal([]byte(filesJSON), &files)
		allFiles = append(allFiles, files...)
	}
	return allFiles, rows.Err()
}
