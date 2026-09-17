# lazytask — Progress & Delivery Notes

## Status Overview

| # | Chunk | Status | Objective |
|---|---|---|---|
| 1 | `chore/scaffold` | **Committed** (`88c7686`) | [go.mod](file:///home/tylerkilburn/Git/lazytask/go.mod), [cmd/lazytask/main.go](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go), minimal Bubble Tea [`model`](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go#L9-L11), [README.md](file:///home/tylerkilburn/Git/lazytask/README.md), [.gitignore](file:///home/tylerkilburn/Git/lazytask/.gitignore). |
| 2 | `internal/taskwarrior/client` | **Completed** (Awaiting Commit) | Thin wrapper around `task export` returning parsed `[]Task` (JSON decode only). Unit + integration tests. No UI. |
| 3 | `internal/taskwarrior/mutations` | Planned (Next) | `Add`, `Done`, `Delete` wrapper functions over `task` CLI. Unit + integration tests. No UI. |
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

### Chunk 2: `internal/taskwarrior/client`
- **Objective**: Thin wrapper around `exec.Command("task", "export", ...)` returning parsed `[]Task` structs (JSON decode only). Unit + integration tests. No UI.
- **Files Created**:
  - [internal/taskwarrior/task.go](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/task.go): [`Task`](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/task.go#L10-L23) struct definition and [`ParseTasks`](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/task.go#L27-L43) JSON decoder function.
  - [internal/taskwarrior/client.go](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/client.go): [`TaskReader`](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/client.go#L12-L14) interface, [`Client`](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/client.go#L17-L22) struct with options ([`WithBinary`](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/client.go#L30-L34), [`WithTaskData`](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/client.go#L37-L41), [`WithTaskRC`](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/client.go#L44-L48), [`WithEnviron`](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/client.go#L51-L55)), and [`Client.Export`](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/client.go#L95-L121) execution.
  - [internal/taskwarrior/task_test.go](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/task_test.go): Table-driven unit tests for [`TestParseTasks`](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/task_test.go#L10-L95) covering empty input, whitespace, single task, multi-task, and invalid JSON.
  - [internal/taskwarrior/client_test.go](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/client_test.go): Unit tests for client options and errors, plus [`TestClient_Export_Integration`](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/client_test.go#L48-L98) with isolated `TASKDATA`/`TASKRC` temp directories.
- **Verification**:
  - `go build ./...` succeeded.
  - `go test -v ./...` passed (all unit and integration tests passed).
- **Manual QA**:
  - Run `go test -v -run TestClient_Export_Integration ./internal/taskwarrior`.
  - Confirm test runs against isolated temporary directory and passes.

---

## Up Next

### Chunk 3: `internal/taskwarrior/mutations`
- **Objective**: `Add`, `Done`, `Delete` wrapper functions over `task` CLI. Unit + integration tests. No UI.
- **Rules & Guardrails**:
  - Do not build UI in Chunk 3.
  - Integration tests must isolate taskwarrior via temporary `TASKDATA` and `TASKRC` environment variables.
  - Bounded diff: ~150–250 lines changed, <= 4–6 files.
