# ATM Architecture

ATM is a Go terminal application with three layers:

1. Process discovery finds known local agent CLIs from `ps`.
2. Agent adapters enrich those processes with local product state.
3. The TUI renders a live task-manager view over the resulting snapshot.

## Discovery

The discovery layer reads the process table with:

```sh
ps -axo pid=,ppid=,etime=,command=
```

For each known agent command, ATM resolves the current working directory with:

macOS uses `lsof -a -p <pid> -d cwd -Fn`. Linux uses the native
`/proc/<pid>/cwd` symlink.

This keeps the base view useful even when ATM does not yet understand an
agent's private session format.

## Adapters

Codex is the first deep adapter. It reads recent files under
`~/.codex/sessions`, matches sessions by cwd, and extracts the latest meaningful
activity from the tail of the JSONL session file.

Other agents currently use the generic process-only source. They still appear in
the task manager with PID, runtime, cwd, project, command, and `unknown` health.

## TUI

The TUI is implemented with Bubble Tea, Bubbles, and Lip Gloss. It refreshes on a
two-second interval, preserves selection by stable `agent:pid` IDs, and keeps the
CLI `list` and `inspect` commands available for scripts and debugging.

Health is derived from the latest known activity time:

- `active`: activity within two minutes
- `idle`: activity between two and fifteen minutes
- `stale`: activity older than fifteen minutes
- `unknown`: no adapter activity signal
