# ATM

ATM is the Agent task manager: a terminal task manager for seeing which
agentic tools are running on the machine and what they appear to be doing.

ATM is built for people who run several local coding agents at once and need a
single live view of what is active, idle, stale, or only visible as a process.

The TUI opens by default. It discovers known agent CLI processes from `ps`, reads
their current working directory with `lsof`, and enriches Codex processes from
recent `~/.codex/sessions/*.jsonl` activity.

## Features

- Live task-manager table for local agent processes
- Activity health states: `active`, `idle`, `stale`, and `unknown`
- Codex session enrichment from local JSONL session logs
- Filtering across agent name, PID, project, activity, session ID, and source
- Sort cycling by activity, agent, project, and PID
- Detail panel with command, cwd, PID/PPID, session path, and activity summary
- JSON output for scripts and debugging

## Install

With Go:

```sh
go install github.com/artpar/atm@latest
```

Without Go, download the archive for your OS/architecture from
[GitHub Releases](https://github.com/artpar/atm/releases).

Linux users can also install release packages from the same page:

- `.deb`
- `.rpm`
- `.apk`

## Usage

```sh
atm
atm tui
atm list
atm list -watch 2s
atm list -json
atm inspect <pid>
atm inspect <pid> -json
```

TUI keys:

- `/` filter
- `s` cycle sort
- `r` refresh now
- `enter` toggle details
- `c` copy session path or command
- `q` quit

Known agents today:

- `codex`
- `claude`
- `gemini`
- `aider`
- `opencode`
- `goose`
- `amp`
- `cursor-agent`

Codex has the deepest adapter in the current version. Other agents are shown
from process data until adapter support is added for their local state.

## Development

```sh
go test ./...
go run .
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the current design and
[docs/ROADMAP.md](docs/ROADMAP.md) for planned work. See
[docs/RELEASE.md](docs/RELEASE.md) for release and packaging details.
