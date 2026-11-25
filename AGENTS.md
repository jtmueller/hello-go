# AGENTS.md

Server-rendered Go todo app: Go `net/http` + Templ + HTMX + Alpine.js + Tailwind, backed by SQLite. The server returns HTML; there is no JSON API and no JS build step. See README.md for the architecture diagram and route table.

## Commands

```sh
make test      # templ generate, then go test -race ./...   (use this, not bare go test, on a fresh clone)
make run       # generate templates + CSS, serve on :8080 (PORT overrides)
make dev       # live reload on :8080 (app on :8081); needs tailwindcss + templ
make generate  # templ generate only
make css       # build static/app.css
make vendor    # re-download htmx/Alpine at the versions pinned in the Makefile
```

Prerequisites: Go, the `templ` CLI, and the **standalone** `tailwindcss` CLI (`brew install tailwindcss`).

Generated and git-ignored: `views/*_templ.go`, `static/app.css`, `bin/`, `*.db*`. A bare `go build`/`go vet` fails on a fresh clone until `make generate css` has run. Never edit `*_templ.go`; edit `views/*.templ` and regenerate.

## Layout

- `main.go` - routes, handlers, DB_PATH/PORT config, graceful shutdown.
- `model/store.go` - SQLite-backed `Store`; `model/view.go` - `Todo`/`View` types passed to templates.
- `views/*.templ` - Templ components (`Page`, `Section`, `Item`, `AddForm`).
- `main_test.go` (handlers via httptest), `model/store_test.go` (store, incl. persistence and concurrency).
- `docs/superpowers/{specs,plans}/` - design doc and plan for the SQLite work.
- `static/vendor/` - vendored htmx/Alpine, committed; bump versions in the Makefile, not by hand.

## Conventions and gotchas

- **Request flow:** every mutation handler changes state and returns the `Section` fragment, which HTMX swaps over `#todo-section` (`hx-swap="outerHTML"`). Keep the list and "N left" count in that one fragment. URLs rendered in templates must carry `?filter=` so the current filter survives each action (tests enforce this).
- **Mutations go through `app.mutate`** (`main.go`): it runs the change and the re-read of the view in one `Store.InTx` transaction, so a failure returns 500 with nothing saved and retries are safe. New mutating handlers should use it. Inside `InTx`, use only the `*Store` you are handed; nesting is an error.
- **Store errors** are returned, logged, and become HTTP 500 via `fail()`. Missing ids in Toggle/Rename/Delete are silent no-ops, not errors. Blank titles are ignored by handlers.
- **SQLite:** driver is pure-Go `modernc.org/sqlite` (no cgo; keep it that way). One connection (`SetMaxOpenConns(1)`), WAL, `busy_timeout`. Keep SQL plain; no SQLite-only features beyond PRAGMAs, so a later Postgres move stays cheap. Splitting into read/write pools was deliberately deferred.
- **DB path:** `DB_PATH`, default `todos.db`. The path is URI-escaped in `open()` (`?`, `#`, `%`); don't build the DSN another way. Seeding of the three example todos is gated by `PRAGMA user_version` and runs in one transaction; emptying the list must not reseed.
- **Tests:** use `model.NewEmptyStore()` (in-memory, unseeded) or `Open(t.TempDir()+...)`. `Open` seeds three todos; `NewEmptyStore` does not. Run with `-race`.
- **XSS:** titles are escaped by Templ; `TestAddEscapesHTML` and `TestFilterIsNotInjectable` guard this. Don't use `templ.Raw` on user input.
- Shutdown: SIGINT/SIGTERM drain requests then `store.Close()` (checkpoints the WAL). Don't replace with `log.Fatal` around `ListenAndServe` alone.
