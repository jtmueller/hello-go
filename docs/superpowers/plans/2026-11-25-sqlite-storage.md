# SQLite Storage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist todos in SQLite so they survive restarts.

**Architecture:** `model.Store` wraps a `*sql.DB` (single connection, WAL) and keeps its current method set; handlers and templates are untouched. Schema is created on open; example todos are seeded only into a brand-new database.

**Tech Stack:** Go `database/sql`, `modernc.org/sqlite` (pure Go, no cgo).

**Spec:** docs/superpowers/specs/2026-11-25-sqlite-storage-design.md

## Global Constraints

- Driver: `modernc.org/sqlite`; no cgo.
- `SetMaxOpenConns(1)`; WAL; `busy_timeout`.
- DB path from `DB_PATH`, default `todos.db`; `*.db*` gitignored; README documents it.
- Plain SQL; no SQLite-specific features beyond PRAGMAs.
- No AI attribution in commit messages (user rule).

## Review Focus

- Data written, store closed, reopened on same file: todos still present (Task 1).
- Deleting all seeded todos then reopening must not reseed (Task 1).
- Toggle/Rename/Delete with a nonexistent id is a no-op, not an error (Task 1).
- IDs are never reused after deleting the highest id (AUTOINCREMENT) (Task 1).
- Concurrent Add/List under `-race` (Task 1).
- Store failure mid-request returns 500, not a half-rendered page (Task 2).
- Unwritable `DB_PATH` fails startup with a clear log message (Task 2).

---

### Task 1: SQLite-backed Store

**Files:**
- Modify: `model/store.go`, `model/store_test.go`, `go.mod`, `go.sum`. Existing store tests must be updated for the new return values (check errors with `t.Fatal`).

**Interfaces:**
- Produces: `func Open(path string) (*Store, error)`; `func (s *Store) Close() error`; `func NewEmptyStore() *Store` (in-memory, panics on error, for tests); `func NewStore() *Store` removed. Methods now return errors: `Add(title string) (Todo, error)`, `List(filter string) ([]Todo, error)`, `Remaining() (int, error)`, `Toggle(id int) error`, `Rename(id int, title string) error`, `Delete(id int) error`, `ClearDone() error`. Toggle/Rename/Delete of a missing id return nil (no-op).

- [ ] **Step 1: Write failing tests** in `model/store_test.go`, using `filepath.Join(t.TempDir(), "t.db")`:
  `TestPersistsAcrossReopen` (Add "a", Close, Open same path, List("all") titles == ["a"]);
  `TestSeedOnlyOnNewDB` (Open seeds 3; Delete all; Close; reopen -> 0 todos);
  `TestIDsNotReused` (Add, Add, Delete id 2, Add -> id 3);
  `TestMissingIDIsNoop` (Toggle/Rename/Delete(999) leave List unchanged);
  `TestConcurrentAddList` (20 goroutines Add+List, then len == 20).
  Replace `TestNewStoreSeeds` with an Open-based equivalent (3 todos, Remaining 3). Existing tests keep using `NewEmptyStore()`.
- [ ] **Step 2: Run** `go test -race ./model/` — expect FAIL (undefined: Open).
- [ ] **Step 3: Add dependency** `go get modernc.org/sqlite`.
- [ ] **Step 4: Implement** in `model/store.go`: `Open` uses DSN `file:<path>?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)`, `SetMaxOpenConns(1)`, runs `CREATE TABLE IF NOT EXISTS todos(id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, done INTEGER NOT NULL DEFAULT 0, created TIMESTAMP NOT NULL)`; seeds the 3 existing example titles only when the table was just created (detect via `sqlite_master` before creating, or a `PRAGMA user_version` marker so emptied lists never reseed). Drop the mutex and slice. `NewEmptyStore` opens `:memory:` without seeding. `List` orders by id and maps `done` to bool.
- [ ] **Step 5: Run** `go test -race ./model/` — expect PASS.
- [ ] **Step 6: Commit** `git add model go.mod go.sum && git commit -m "Back todo store with SQLite"`

### Task 2: Wire into main, docs, ignore

**Files:**
- Modify: `main.go`, `main_test.go`, `.gitignore`, `README.md`

**Interfaces:**
- Consumes: `model.Open(path string) (*model.Store, error)`, `(*Store).Close() error`, the error-returning methods from Task 1.
- Produces: `func (a *app) view(r *http.Request) (model.View, error)`; every handler that mutates or renders responds `500` (via `http.Error`, error logged with `log.Printf`) when a store call fails, and does not render a partial section.

- [ ] **Step 1:** In `main()`, read `DB_PATH` (default `todos.db`), call `model.Open`; on error `log.Fatalf("open database: %v", err)`; `defer store.Close()` is not reached after `log.Fatal`, so close on exit is best-effort (WAL is crash-safe). Replace `model.NewStore()`.
- [ ] **Step 2: Tests first** in `main_test.go`: `TestHandlerReturns500OnStoreError` — close the store (`store.Close()`), then GET `/todos` and POST `/todos` with `title=x` must return 500; existing tests updated and still pass. Then update handlers to propagate errors.
- [ ] **Step 2b:** Add `*.db`, `*.db-wal`, `*.db-shm` to `.gitignore`; add a README section on `DB_PATH` and persistence.
- [ ] **Step 3: Run** `make test` — expect PASS.
- [ ] **Step 4: Manual check:** `DB_PATH=/tmp/x.db PORT=8099 go run .`, add a todo via `curl -d title=hi localhost:8099/todos`, stop, restart, `curl localhost:8099/todos` shows "hi"; `DB_PATH=/nonexistent/x.db go run .` exits with the "open database" message.
- [ ] **Step 5: Commit** `git add main.go .gitignore README.md && git commit -m "Use SQLite store via DB_PATH"`
