# lazytask — Progress & Delivery Notes

## Status Overview

| # | Chunk | Status | Objective |
|---|---|---|---|
| 1 | `chore/scaffold` | **Committed** (`88c7686`) | [go.mod](file:///home/tylerkilburn/Git/lazytask/go.mod), [cmd/lazytask/main.go](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go), minimal Bubble Tea [`model`](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go#L9-L11), [README.md](file:///home/tylerkilburn/Git/lazytask/README.md), [.gitignore](file:///home/tylerkilburn/Git/lazytask/.gitignore). |
| 2 | `internal/taskwarrior/client` | Planned (Next) | Thin wrapper around `task export` returning parsed `[]Task` (JSON decode only). Unit + integration tests. No UI. |
| 3 | `internal/taskwarrior/mutations` | Planned | `Add`, `Done`, `Delete` wrapper functions over `task` CLI. Unit + integration tests. No UI. |
| 4 | `internal/ui/tasklist` | Planned | Bubble Tea model rendering task list panel (fixture-driven), lazygit-style bordered pane, selection navigation. No live taskwarrior wiring. |
| 5 | `wire: list panel to real data` | Planned | Connect chunk 2 client to chunk 4 panel on startup; `r` key refresh. |
| 6 | `internal/ui/addform` | Planned | Input panel for new task description, wired to chunk 3 `Add`. |
| 7 | `feature: complete/delete` | Planned | Keybindings (`d` done, `x` delete + confirm) wired to chunk 3. |
| 8 | `internal/editor` | Planned | Helper to write task to temp file, launch `$EDITOR`, read back changes; unit tests. |
| 9 | `feature: edit task` | Planned | Wire chunk 8 into task list (`e` key), re-import edited fields. |
| 10 | `polish: status bar & help` | Planned | Lazygit-style bottom bar showing active keybindings for current panel. |

---

## Completed Chunks

### Chunk 1: `chore/scaffold`
- **Objective**: Base project scaffolding per Section 7 of [requirements.md](file:///home/tylerkilburn/Git/lazytask/requirements.md).
- **Files Created**:
  - [.gitignore](file:///home/tylerkilburn/Git/lazytask/.gitignore): Ignores `/lazytask` binary, test binaries (`*.test`, `*.out`), IDE and OS files.
  - [README.md](file:///home/tylerkilburn/Git/lazytask/README.md): Project summary, prerequisites, build/run/test instructions.
  - [go.mod](file:///home/tylerkilburn/Git/lazytask/go.mod) & [go.sum](file:///home/tylerkilburn/Git/lazytask/go.sum): Initialized module `github.com/tkilb/lazytask` with `github.com/charmbracelet/bubbletea` and `github.com/stretchr/testify`.
  - [cmd/lazytask/main.go](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go): Root entrypoint implementing [`model`](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go#L9-L11), [`initialModel`](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go#L13-L15), [`model.Init`](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go#L17-L19), [`model.Update`](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go#L21-L32) (handles `q` and `ctrl+c`), [`model.View`](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go#L34-L39), and [`main`](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go#L41-L47).
  - [cmd/lazytask/main_test.go](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main_test.go): Table-driven unit tests [`TestModelUpdate`](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main_test.go#L10-L54) and [`TestModelView`](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main_test.go#L56-L75).
- **Verification**:
  - `go build ./...` succeeded.
  - `go test -v ./...` passed.
- **Manual QA**:
  - Run `go run ./cmd/lazytask`.
  - Verify banner `lazytask (scaffold) - press 'q' or 'ctrl+c' to quit`.
  - Press `q` or `Ctrl+C` to cleanly exit.

---

## Up Next

### Chunk 2: `internal/taskwarrior/client`
- **Objective**: Thin wrapper around `exec.Command("task", "export", ...)` returning parsed `[]Task` structs (JSON decode only). Unit + integration tests. No UI.
- **Rules & Guardrails**:
  - Do not peek ahead or build mutations/UI in Chunk 2.
  - Integration tests must isolate taskwarrior via temporary `TASKDATA` and `TASKRC` environment variables.
  - Integration tests must skip (`t.Skip`) if `task` CLI is not installed.
  - Bounded diff: ~150–250 lines changed, <= 4–6 files.
