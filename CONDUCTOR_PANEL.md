# Conductor Panel Integration

Three-column layout with conductor visibility + smart commit-to-feature auto-linking.

## Status: Implemented (all phases)

`go build` and `go vet` pass. Needs manual smoke test.

---

## Architecture

```
dashboard (25%) | graph (40%) | conductor (35%)
```

- `g` toggles both graph+conductor together
- `Ctrl+H`/`Ctrl+L` cycles focus: dashboard -> graph -> conductor
- `Esc` returns to dashboard from any panel
- Conductor reads from `<repo>/.conductor/conductor.db` (read-only, graceful nil when absent)

## New Files

| File | Purpose |
|------|---------|
| `conductor/types.go` | Feature, Session, Handoff, QualityReflection, Memory, FeatureMatch, ConductorData |
| `conductor/db.go` | SQLite queries via `modernc.org/sqlite` (pure Go). Open, GetFeatures, GetActiveSession, GetLatestHandoff, GetQualityIssues, GetRecentMemories, RecordCommit, GetAllData |
| `conductor/match.go` | MatchFeature: scores features against commit msg + changed files. In-progress bonus (+0.5), keyword overlap (+0.3), category match (+0.1), file overlap (+0.2) |
| `tui/conductorpane/model.go` | FlatItem pattern (same as dashboard). ItemKind: FeatureHeader, FeatureItem, SessionHeader, HandoffItem, QualityHeader, QualityItem, MemoryHeader, MemoryItem. Collapsible sections, cursor nav, scroll offset |
| `tui/featurelinker/model.go` | Overlay (same pattern as branchpicker). Shows after commit with scored feature matches. j/k nav, Enter to link, Esc to skip |

## Modified Files

| File | Changes |
|------|---------|
| `tui/app.go` | FocusPanel enum, conductorPane + featureLinker fields, 3-column layoutSizes, renderDashboardLayout, conductor fetch/refresh cmds, feature match/link cmds, status bar conductor summary, featureLinker overlay routing |
| `tui/shared/messages.go` | ConductorRefreshedMsg, FeatureLinkedMsg |
| `tui/shared/styles.go` | ConductorBorderStyle, ConductorBorderFocusedStyle, ConductorPassedBadge, ConductorActiveBadge, ConductorQualityBadge |
| `config/config.go` | DashboardWidth field in DisplayConfig, ResolvedDashboardWidth() |
| `go.mod` | Added `modernc.org/sqlite` + transitive deps |

## Conductor Panel Sections

```
 Features ─ 4/7 passed ───────────
  ✓ Add conductor panel            passed   (StagedFileStyle)
  ○ Fix layout ratios              pending  (DimFileStyle)
  ● Rate limiting  [active]        in_progress (UnstagedFileStyle)
  ✗ Auth middleware [x2]           failed   (ErrorStyle)

 Session #5 ──────────────────────
  completed - 3 features done

 Handoff ─────────────────────────
  Task:  Implementing panel
  Next:  Wire up linking
  Files: tui/app.go, ...

 Quality (2) ─────────────────────
  ⚠ Tests skipped: auth cases
  ⚠ Debt: hardcoded DB path

 Memories ────────────────────────
  bubbletea-patterns
  lipgloss-layout-tricks
```

## Feature Linker Flow

1. User commits (`CommitCompleteMsg`)
2. `matchFeaturesCmd` scores active features against commit msg
3. If matches found, `featureLinker.Show()` displays overlay
4. Best match pre-selected at cursor 0
5. Enter = link (writes to commits table + updates features.commit_hash)
6. Esc/s = skip

```
 Link commit to feature? ──────────
  → Add conductor panel     (87%)
    Fix layout ratios        (23%)
    Rate limiting             (12%)
    [skip]
```

## Status Bar

```
 dev | main | ↑2 | 4/7 | #5 | ⚠2 | ? for help
```

- `4/7` = features passed/total (ConductorPassedBadge)
- `#5` = session number (CommitDetailDateStyle)
- `⚠2` = unresolved quality issues (ConductorQualityBadge)
- Only shown when conductor.db exists for selected repo

## Config

```toml
[display]
dashboard_width = 25  # percentage, default 25
```

## Patterns Reused (DRY)

- **FlatItem + collapse**: same as dashboard/model.go
- **Border styles**: ConductorBorderStyle = GraphBorderStyle pattern
- **Overlay**: featurelinker = branchpicker ViewOverlay pattern
- **Status indicators**: ✓ = StagedIndicator, ○ = DimFileStyle, ● = UnstagedFileStyle, ✗ = ErrorStyle
- **Section labels**: CommitDetailLabelStyle + CommitDetailMsgStyle
- **Quality warnings**: FeedbackWarningStyle
- **Polling**: same 2s pollInterval tick as git status refresh

## Known Limitations

- Connection cache never closes DB handles (fine for TUI lifecycle)
- RecordCommit uses `hex(randomblob(16))` for ID, not UUID
- Feature matching for new features with no prior commits gets lower file-overlap scores
- 3-column layout only at width > 80; 2-column fallback at 40-80
