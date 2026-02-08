---
name: conductor-resume
description: Resume an autonomous coding session from the last handoff
---

# Conductor Resume

Resume work from where the last session left off.

## Usage

/conductor-resume [project-name]

## Behavior

1. Call `mcp__conductor__resume_from_handoff` to get last session context
2. Display last session summary
3. Show next steps and any blockers
4. Continue the implementation loop

## Implementation

```
# Get handoff context
context = mcp__conductor__resume_from_handoff(projectName)

print("Resuming project:", context.project)
print("Progress:", context.progress.passed, "/", context.progress.total)
print("Last task:", context.handoff.currentTask)
print("Next steps:", context.handoff.nextSteps)
print("Blockers:", context.handoff.blockers)

# Start new session
session = mcp__conductor__start_session(projectName)

# Continue with next feature
feature = mcp__conductor__get_next_feature(projectName)
```
