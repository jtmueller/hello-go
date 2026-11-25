package model

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// Store is a todo store backed by SQLite. The pool is limited to one
// connection, which serializes access and avoids "database is locked" errors.
type Store struct {
	q  queryer // the *sql.DB, or the open transaction inside InTx
	db *sql.DB // nil for a transaction-scoped Store
}

type queryer interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

const schema = `CREATE TABLE IF NOT EXISTS todos (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	title   TEXT NOT NULL,
	done    INTEGER NOT NULL DEFAULT 0,
	created TIMESTAMP NOT NULL
)`

// Open opens (creating if needed) the database at path. A brand-new
// database is seeded with a few example todos; an existing one never is.
func Open(path string) (*Store, error) {
	s, err := open(path)
	if err != nil {
		return nil, err
	}
	if err := s.seedIfNew(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// NewEmptyStore returns an unseeded in-memory store, for tests.
func NewEmptyStore() *Store {
	s, err := open(":memory:")
	if err != nil {
		panic(err)
	}
	return s
}

func open(path string) (*Store, error) {
	// Escape characters that the URI parsers would treat as syntax.
	esc := strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23").Replace(path)
	db, err := sql.Open("sqlite", "file:"+esc+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{q: db, db: db}, nil
}

// seedIfNew inserts example todos once, tracked by PRAGMA user_version so
// that emptying the list later does not bring them back.
func (s *Store) seedIfNew() error {
	var v int
	if err := s.q.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return err
	}
	if v != 0 {
		return nil
	}
	for _, title := range []string{"Read the Templ docs", "Wire up HTMX", "Sprinkle in some Alpine.js"} {
		if _, err := s.Add(title); err != nil {
			return err
		}
	}
	_, err := s.q.Exec("PRAGMA user_version = 1")
	return err
}

func (s *Store) Close() error {
	if s.db == nil {
		return errors.New("store: cannot close a transaction-scoped store")
	}
	return s.db.Close()
}

// InTx runs fn against a transaction-scoped Store. It commits if fn returns
// nil and rolls back otherwise, so a request can change and re-read state
// atomically. fn must use only the Store it is given; nesting is an error.
func (s *Store) InTx(fn func(tx *Store) error) error {
	if s.db == nil {
		return errors.New("store: InTx cannot be nested")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(&Store{q: tx}); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) Add(title string) (Todo, error) {
	t := Todo{Title: title, Created: time.Now().UTC()}
	res, err := s.q.Exec("INSERT INTO todos (title, done, created) VALUES (?, 0, ?)", title, t.Created)
	if err != nil {
		return Todo{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Todo{}, err
	}
	t.ID = int(id)
	return t, nil
}

// List returns todos filtered by "all", "active" or "done".
func (s *Store) List(filter string) ([]Todo, error) {
	q := "SELECT id, title, done, created FROM todos"
	switch filter {
	case "active":
		q += " WHERE done = 0"
	case "done":
		q += " WHERE done = 1"
	}
	rows, err := s.q.Query(q + " ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Todo{}
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Done, &t.Created); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) Remaining() (int, error) {
	var n int
	err := s.q.QueryRow("SELECT COUNT(*) FROM todos WHERE done = 0").Scan(&n)
	return n, err
}

func (s *Store) Toggle(id int) error {
	_, err := s.q.Exec("UPDATE todos SET done = 1 - done WHERE id = ?", id)
	return err
}

func (s *Store) Rename(id int, title string) error {
	_, err := s.q.Exec("UPDATE todos SET title = ? WHERE id = ?", title, id)
	return err
}

func (s *Store) Delete(id int) error {
	_, err := s.q.Exec("DELETE FROM todos WHERE id = ?", id)
	return err
}

func (s *Store) ClearDone() error {
	_, err := s.q.Exec("DELETE FROM todos WHERE done = 1")
	return err
}
