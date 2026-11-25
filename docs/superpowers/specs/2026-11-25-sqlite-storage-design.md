# SQLite persistent storage (short design)

## Intent
Todos currently live in an in-memory slice and are reseeded on every boot. Make them survive restarts.

## Goals / non-goals / anti-goals
- Goals: persistence across restarts; handlers and templates unchanged where possible; tests stay fast.
- Non-goals: multi-instance deployment, migration framework, read/write pool split.
- Anti-goals: cgo requirement; SQLite-specific SQL beyond PRAGMAs (keeps a later Postgres move cheap).

## Approaches
- modernc.org/sqlite (pure Go) vs mattn/go-sqlite3 (cgo). Chose modernc: no cgo, simple build/cross-compile; costs binary size and some speed.
- Single connection vs split read/write pools. Chose single connection (user decision): simplest, no lock errors, `:memory:` works. Split pools are a contained later change inside Open().

## Design
- `model.Open(path)` returns `*Store` wrapping `*sql.DB`; WAL, busy_timeout, `SetMaxOpenConns(1)`.
- Schema via `CREATE TABLE IF NOT EXISTS todos(id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, done INTEGER NOT NULL DEFAULT 0, created TIMESTAMP NOT NULL)`.
- `NewEmptyStore()` becomes an in-memory Open for tests. Seeding only when the DB file is newly created.
- DB path from `DB_PATH`, default `todos.db`; `*.db*` gitignored; README documents it.
- Store methods return errors; handlers log them and respond 500 (user decision).
