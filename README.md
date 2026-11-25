# hello-go

A small todo app showing a hypermedia-style full-stack setup:

- **Go** (`net/http`) serves everything and owns all state.
- **[Templ](https://templ.guide)** renders type-safe HTML components.
- **[HTMX](https://htmx.org)** talks to the server and swaps HTML fragments.
- **[Alpine.js](https://alpinejs.dev)** handles small client-only UI state.
- **[Tailwind CSS](https://tailwindcss.com)** does all styling.

There is no JSON API and no JavaScript build step. The server returns HTML.

## Quick start

Prerequisites: Go, `templ`, and the **standalone** `tailwindcss` CLI.

```sh
go install github.com/a-h/templ/cmd/templ@latest   # installs to ~/go/bin
brew install tailwindcss

make run    # generate templates, build CSS, start on :8080 (override with PORT)
make dev    # live reload: open http://localhost:8080 (proxy to the app on :8081)
make build  # produces bin/hello-go
make test   # generate templates, then go test -race ./...
make vendor # re-download htmx/Alpine at the versions pinned in the Makefile
```

Generated files (`views/*_templ.go`, `static/app.css`) are git-ignored; the
`make` targets recreate them. A bare `go build` on a fresh clone will fail
until you run `make generate css`.

## Persistence

Todos are stored in SQLite (pure-Go `modernc.org/sqlite`, no cgo). The file path
comes from `DB_PATH` (default `todos.db` in the working directory); it is
created on first run and seeded with a few example todos once. The store uses
a single connection in WAL mode. Each request changes and re-reads state in one
transaction, so a failure returns HTTP 500 with nothing saved (retrying is
safe). SIGINT/SIGTERM drain requests and close the database cleanly.

```sh
DB_PATH=/var/lib/hello-go/todos.db make run
```

## Layout

```
main.go            routes + HTTP handlers
main_test.go       handler tests via httptest (no network)
AGENTS.md          guidance for coding agents: commands, conventions, gotchas
model/
  store.go         SQLite-backed todo store
  view.go          Todo and View types passed to templates
  store_test.go    store tests (persistence, transactions, concurrency)
views/
  layout.templ     HTML shell, loads HTMX/Alpine, dark-mode toggle
  todos.templ      Page, AddForm, Section, Item components
assets/input.css   Tailwind entry point (source)
static/app.css     Tailwind output (generated, git-ignored)
static/vendor/     vendored htmx and Alpine.js (committed)
```

## Architecture

```
Browser ──HTMX request──▶ handler (main.go) ──▶ Store (model/)
   ▲                           │
   └──── HTML fragment ◀── Templ component (views/)
```

**Request flow.** `GET /` renders the full page. Every other interaction is an
HTMX request whose handler mutates the store and returns the `Section`
fragment (list + footer), which HTMX swaps over `#todo-section` with
`hx-swap="outerHTML"`. One fragment keeps the list and the "N left" count
consistent after every change.

| Route                      | Action                              |
| -------------------------- | ----------------------------------- |
| `GET /`                    | Full page                           |
| `GET /todos`               | Section only (filter links)         |
| `POST /todos`              | Add a todo                          |
| `POST /todos/{id}/toggle`  | Toggle done                         |
| `PUT /todos/{id}`          | Rename                              |
| `DELETE /todos/{id}`       | Delete                              |
| `DELETE /todos`            | Clear completed                     |

**Filter state** (`all` / `active` / `done`) lives in the `?filter=` query
string, not in server or client state. Templates thread it into every
`hx-*` URL, so a mutation re-renders with the current filter, and filter
links use `hx-push-url` so the URL stays shareable and reloads correctly.

**Who owns what**

- *Server (Go + HTMX)*: anything that must persist or be consistent, such as
  adding, toggling, renaming, deleting and filtering.
- *Browser (Alpine)*: ephemeral UI state, with no round trip: the add-form
  character counter and disabled button, the double-click-to-edit toggle on
  each item, and the dark-mode switch (persisted in `localStorage`).

**Templ components.** `Page` wraps `Layout`; `Section` is the swappable
fragment and composes `Item`. Components take plain Go structs
(`model.View`), and templ escapes all output by default.

**Storage.** `model.Store` wraps a SQLite database (single connection, WAL).
Handlers change state through `app.mutate`, which runs the change and the
re-read of the view inside one `Store.InTx` transaction; a store error is
logged and returned as HTTP 500. The SQL is plain, so moving to another
database means rewriting `model/store.go` only.

**Styling.** Tailwind v4 scans the `.templ` files for utility classes and
writes `static/app.css`, served from `/static/`. Dark mode is class-based
(`dark:` variants, toggled on `<html>` by Alpine); an `htmx-request` variant
is defined for in-flight styling.

**Vendored scripts.** HTMX and Alpine are served from `static/vendor/`, so the
app works offline and has no runtime CDN dependency. To update, change
`HTMX_VERSION` / `ALPINE_VERSION` at the top of the `Makefile` (or run
`make vendor HTMX_VERSION=x.y.z`), then commit the new files.

**Dev loop.** `make dev` runs Tailwind in watch mode alongside
`templ generate --watch`, which regenerates code, restarts the server, and
refreshes the browser through its proxy.
