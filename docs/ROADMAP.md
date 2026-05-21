# ATM Roadmap

ATM's first product goal is to be a reliable terminal task manager for local
agentic software.

## Near Term

- Add deeper adapters for Claude, Aider, OpenCode, Gemini, and Cursor Agent.
- Add CPU and memory columns without making OS resource usage the primary view.
- Add a non-destructive "open session path" action where the local environment
  supports it.
- Improve responsive table layouts for very small terminal widths.

## Later

- Add optional confirmation-based controls such as terminate process.
- Add session history search across recent agent activity.
- Add project grouping once the flat task-manager table is mature.
- Add package/release automation for Homebrew or direct GitHub releases.

## Non-Goals For Now

- No GUI in the first product milestone.
- No background daemon.
- No cloud sync.
- No write operations into agent session stores.

