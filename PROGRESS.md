# lazytask — Progress & Delivery Notes

## Status Overview

| # | Chunk | Status | Objective |
|---|---|---|---|
| 1 | `chore/scaffold` | **Committed** (`88c7686`) | [go.mod](file:///home/tylerkilburn/Git/lazytask/go.mod), [cmd/lazytask/main.go](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go), minimal Bubble Tea [`model`](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go#L9-L11), [README.md](file:///home/tylerkilburn/Git/lazytask/README.md), [.gitignore](file:///home/tylerkilburn/Git/lazytask/.gitignore). |
| 2 | `internal/taskwarrior/client` | **Committed** (`3015c2b`) | Thin wrapper around `task export` returning parsed `[]Task` (JSON decode only). Unit + integration tests. No UI. |
| 3 | `internal/taskwarrior/mutations` | **Committed** (`aa950b5`) | `Add`, `Done`, `Delete` wrapper functions over `task` CLI. Unit + integration tests. No UI. |
| 4 | `internal/ui/tasklist` | **In Progress** | Bubble Tea model rendering task list panel (fixture-driven), lazygit-style bordered pane, selection navigation. No live taskwarrior wiring. |
| 5 | `wire: list panel to real data` | **Committed** | Connect chunk 2 client to chunk 4 panel on startup; `r` key refresh. |
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

### Chunk 3: `internal/taskwarrior/mutations`
- **Objective**: `Add`, `Done`, `Delete` wrapper functions over `task` CLI. Unit + integration tests. No UI.
- **Files Created**:
  - [internal/taskwarrior/mutations.go](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/mutations.go): `TaskMutator` interface, shared `Client.run` helper, and `Client.Add`/`Client.Done`/`Client.Delete` methods wrapping `task add`/`done`/`delete` (via `rc.confirmation=off`), parsing the created task ID from Taskwarrior's "Created task N." output.
  - [internal/taskwarrior/mutations_test.go](file:///home/tylerkilburn/Git/lazytask/internal/taskwarrior/mutations_test.go): Unit tests for created-task-ID regexp parsing, invalid binary, and empty-ID validation, plus `TestClient_Mutations_Integration` exercising add/done/delete against an isolated `TASKDATA`/`TASKRC` temp directory (deletes by UUID to avoid pending-ID renumbering after a completion).
- **Verification**:
  - `go build ./...` succeeded.
  - `go vet ./...` succeeded.
  - `go test ./...` passed (all unit and integration tests passed).
- **Manual QA**:
  - Run `go test -v -run TestClient_Mutations_Integration ./internal/taskwarrior`.
  - Or exercise the real CLI by hand in a sandbox:
    ```bash
    export TASKDATA=$(mktemp -d)
    export TASKRC=$TASKDATA/.taskrc
    echo "confirmation=off" > "$TASKRC"
    task add "Buy groceries" project:Home priority:H   # "Created task 1."
    task 1 done
    task export status:completed                       # confirm it shows completed
    ```

### Chunk 4: `internal/ui/tasklist`
- **Objective**: Bubble Tea model rendering the task list panel (static data / fixture-driven), lazygit-style bordered pane, up/down selection. No live taskwarrior wiring yet.
- **Files Created**:
  - [internal/ui/tasklist/model.go](file:///home/tylerkilburn/Git/lazytask/internal/ui/tasklist/model.go): `Model` (Bubble Tea model) holding `[]taskwarrior.Task` + cursor + size; `New`, `SetTasks`, `Selected`, `Init`, `Update` (up/down/j/k navigation, clamped at bounds; handles `tea.WindowSizeMsg`), `View` (Lip Gloss `RoundedBorder` pane, fixed-width columns for ID/Description/Project/Priority/Due, reverse-video highlight on the selected row).
  - [internal/ui/tasklist/model_test.go](file:///home/tylerkilburn/Git/lazytask/internal/ui/tasklist/model_test.go): table-driven tests for navigation (up/down/j/k, clamping at both ends), `SetTasks` cursor clamping, and `View` content checks.
  - [internal/ui/tasklist/example_test.go](file:///home/tylerkilburn/Git/lazytask/internal/ui/tasklist/example_test.go): runnable `Example` demonstrating construction with fixture tasks and initial selection.
  - [cmd/tasklist-demo/main.go](file:///home/tylerkilburn/Git/lazytask/cmd/tasklist-demo/main.go): throwaway manual-QA harness — runs the `tasklist.Model` standalone against fixture data so a human can visually verify the bordered pane and navigation without any taskwarrior wiring.
- **Files Changed**:
  - [go.mod](file:///home/tylerkilburn/Git/lazytask/go.mod): promoted `github.com/charmbracelet/lipgloss` (already in the dependency tree, per requirements.md §2) from indirect to direct — no new third-party dependency added.
- **Verification**:
  - `go build ./...` succeeded.
  - `go vet ./...` succeeded.
  - `go test ./...` passed (all packages, including new `internal/ui/tasklist` unit tests and the `Example`).
- **Manual QA**:
  - Run `go run ./cmd/tasklist-demo`.
  - Confirm a bordered pane titled "Tasks" appears with 4 fixture rows (Buy groceries, Write quarterly report, Water plants, Fix leaky faucet) and column headers ID/Description/Project/Priority/Due.
  - Press `down`/`j` and `up`/`k` — confirm the highlighted (reverse-video) row moves accordingly and stops at the first/last row instead of wrapping.
  - Press `q` or `ctrl+c` to quit cleanly.
- **Notes**: Reuses `taskwarrior.Task` (from chunk 2) as the row data type rather than defining a duplicate UI-local struct, since chunk 5 will need to feed real `[]taskwarrior.Task` into this same `Model`. No taskwarrior process is invoked anywhere in this package or the demo — fixture data only. `cmd/tasklist-demo` is intentionally separate from `cmd/lazytask/main.go`, which chunk 5 will wire up for real.

### Chunk 5: `wire: list panel to real data`
- **Objective**: Connect chunk 2's `taskwarrior.Client` to chunk 4's `tasklist.Model` on startup in `cmd/lazytask/main.go`; a `r` key refresh.
- **Files Changed**:
  - [cmd/lazytask/main.go](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main.go): `model` now holds a `TaskReader` (small interface satisfied by `taskwarrior.Client`) and a `tasklist.Model`. `Init` dispatches a `fetchTasks` command running `Export(ctx, "status:pending")`. New `tasksLoadedMsg`/`tasksErrMsg` results feed the list or store an error. `r` re-triggers the fetch; other key/window messages are forwarded to `tasklist.Model.Update`. `View` renders the panel, an error line (if any), and a `(r) refresh  (q) quit` hint.
  - [cmd/lazytask/main_test.go](file:///home/tylerkilburn/Git/lazytask/cmd/lazytask/main_test.go): rewritten with a `stubReader` test double (no real `task` process invoked) covering `Init` fetch dispatch, `tasksLoadedMsg`/`tasksErrMsg` handling, `r`-triggered refresh, quit behavior, and view rendering (quitting/error/hint).
- **Verification**:
  - `go build ./...` succeeded.
  - `go vet ./...` succeeded.
  - `go test ./...` passed (all packages).
- **Manual QA**:
  - Run `go run ./cmd/lazytask` against a real (or sandboxed `TASKDATA`/`TASKRC`) taskwarrior install with pending tasks.
  - Confirm the "Tasks" panel shows real pending tasks, not fixtures.
  - Add/complete a task in another terminal, press `r` in the app, confirm the list refreshes.
  - If `task` is missing/misconfigured, confirm an `error loading tasks: ...` line appears instead of a crash.
  - Press `q`/`ctrl+c` to quit cleanly.
- **Status**: Verified working by user; commit pending (user commits per standing instructions).

---

## Up Next

### Chunk 6: `internal/ui/addform`
- **Objective**: Input panel for new task description, wired to chunk 3's `Add`.
