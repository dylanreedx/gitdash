---
name: conductor-start
description: Start an autonomous coding session for a project
---

# Conductor Start

Start an autonomous coding session.

## Usage

/conductor-start <project-name> [workspace-path]

## Examples

- `/conductor-start my-backend`
- `/conductor-start my-ui ./my-workspace`

## Behavior

1. Check if conductor MCP service is connected
2. Call `mcp__conductor__check_dependencies` to verify dependencies
3. Call `mcp__conductor__start_session` to begin a new session
4. Call `mcp__conductor__get_next_feature` to get first feature
5. Spawn conductor-coder subagent for each feature
6. Report progress after each feature
7. Continue until interrupted or complete

## Implementation

```
# Start session
session = mcp__conductor__start_session(projectName, workspacePath)

while true:
  # Get next feature
  feature = mcp__conductor__get_next_feature(projectName)

  if feature.complete:
    break

  # Spawn subagent to implement
  Task({
    subagent_type: "conductor-coder",
    prompt: `Implement feature: ${feature.description}\n\nSteps:\n${feature.steps.join('\n')}`
  })

  # Show progress
  status = mcp__conductor__get_project_status(projectName)
  print(status)
```
