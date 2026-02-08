---
name: conductor-status
description: Show progress status for all projects
---

# Conductor Status

Display current progress for all projects.

## Usage

/conductor-status [project-name]

## Output

- Progress percentage per project
- Current phase
- Features passed/total
- Recent session activity
- Any blocked features

## Implementation

```
if projectName:
  status = mcp__conductor__get_project_status(projectName)
  print_project_details(status)
else:
  status = mcp__conductor__get_project_status()
  for project in status:
    print(project.name, ":", project.progress.percentage, "%")
    print("  ", project.progress.passed, "/", project.progress.total, "features")
```
