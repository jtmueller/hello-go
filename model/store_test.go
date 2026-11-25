package model

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func titles(todos []Todo) []string {
	out := make([]string, len(todos))
	for i, t := range todos {
		out[i] = t.Title
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func empty() *Store { return NewEmptyStore() }

func ok(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func add(t *testing.T, s *Store, title string) Todo {
	t.Helper()
	todo, err := s.Add(title)
	ok(t, err)
	return todo
}

func list(t *testing.T, s *Store, filter string) []Todo {
	t.Helper()
	todos, err := s.List(filter)
	ok(t, err)
	return todos
}

func remaining(t *testing.T, s *Store) int {
	t.Helper()
	n, err := s.Remaining()
	ok(t, err)
	return n
}

func dbPath(t *testing.T) string { return filepath.Join(t.TempDir(), "t.db") }

func TestOpenSeedsNewDB(t *testing.T) {
	s, err := Open(dbPath(t))
	ok(t, err)
	defer s.Close()
	if got := len(list(t, s, "all")); got != 3 {
		t.Fatalf("seeded todos = %d, want 3", got)
	}
	if remaining(t, s) != 3 {
		t.Fatalf("remaining = %d, want 3", remaining(t, s))
	}
}

func TestPersistsAcrossReopen(t *testing.T) {
	path := dbPath(t)
	s, err := Open(path)
	ok(t, err)
	add(t, s, "a")
	ok(t, s.Close())

	s, err = Open(path)
	ok(t, err)
	defer s.Close()
	got := titles(list(t, s, "all"))
	if len(got) != 4 || got[3] != "a" {
		t.Fatalf("titles after reopen = %v", got)
	}
}

func TestPathWithURISpecialChars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a?b#c%20d.db")
	s, err := Open(path)
	ok(t, err)
	defer s.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database not created at the literal path: %v", err)
	}
	var mode string
	ok(t, s.db.QueryRow("PRAGMA journal_mode").Scan(&mode))
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}

// A seeding that fails midway must leave neither todos nor the marker behind,
// otherwise the next Open would seed again and create duplicates.
func TestSeedIsAtomic(t *testing.T) {
	s := empty()
	boom := errors.New("boom")
	err := s.InTx(func(tx *Store) error {
		if err := seed(tx); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if n := len(list(t, s, "all")); n != 0 {
		t.Fatalf("%d todos left after rolled-back seed, want 0", n)
	}
	var v int
	ok(t, s.db.QueryRow("PRAGMA user_version").Scan(&v))
	if v != 0 {
		t.Fatalf("user_version = %d after rollback, want 0", v)
	}
}

func TestSeedOnlyOnNewDB(t *testing.T) {
	path := dbPath(t)
	s, err := Open(path)
	ok(t, err)
	ok(t, s.Delete(1))
	ok(t, s.Delete(2))
	ok(t, s.Delete(3))
	ok(t, s.Close())

	s, err = Open(path)
	ok(t, err)
	defer s.Close()
	if n := len(list(t, s, "all")); n != 0 {
		t.Fatalf("todos after reopen = %d, want 0 (no reseed)", n)
	}
}

func TestAddAssignsIncreasingIDs(t *testing.T) {
	s := empty()
	a, b := add(t, s, "a"), add(t, s, "b")
	if a.ID != 1 || b.ID != 2 {
		t.Fatalf("ids = %d,%d want 1,2", a.ID, b.ID)
	}
	if a.Done || a.Created.IsZero() {
		t.Fatalf("new todo should be not done with a created time: %+v", a)
	}
	if got := list(t, s, "all")[0]; got.Created.IsZero() {
		t.Fatalf("listed todo lost its created time: %+v", got)
	}
}

func TestIDsNotReusedAfterDelete(t *testing.T) {
	s := empty()
	add(t, s, "a")
	add(t, s, "b")
	ok(t, s.Delete(2))
	if got := add(t, s, "c").ID; got != 3 {
		t.Fatalf("id after delete = %d, want 3", got)
	}
}

func TestToggle(t *testing.T) {
	s := empty()
	add(t, s, "a")
	ok(t, s.Toggle(1))
	if !list(t, s, "all")[0].Done {
		t.Fatal("expected done after first toggle")
	}
	ok(t, s.Toggle(1))
	if list(t, s, "all")[0].Done {
		t.Fatal("expected not done after second toggle")
	}
}

func TestMissingIDIsNoop(t *testing.T) {
	s := empty()
	add(t, s, "a")
	ok(t, s.Toggle(99))
	ok(t, s.Rename(99, "x"))
	ok(t, s.Delete(99))
	if got := titles(list(t, s, "all")); !equal(got, []string{"a"}) || list(t, s, "all")[0].Done {
		t.Fatalf("store changed by missing-id ops: %v", got)
	}
}

func TestListFilters(t *testing.T) {
	s := empty()
	add(t, s, "open")
	add(t, s, "closed")
	ok(t, s.Toggle(2))

	tests := []struct {
		filter string
		want   []string
	}{
		{"all", []string{"open", "closed"}},
		{"active", []string{"open"}},
		{"done", []string{"closed"}},
		{"bogus", []string{"open", "closed"}},
	}
	for _, tc := range tests {
		if got := titles(list(t, s, tc.filter)); !equal(got, tc.want) {
			t.Errorf("List(%q) = %v, want %v", tc.filter, got, tc.want)
		}
	}
}

func TestRemaining(t *testing.T) {
	s := empty()
	add(t, s, "a")
	add(t, s, "b")
	ok(t, s.Toggle(1))
	if got := remaining(t, s); got != 1 {
		t.Fatalf("remaining = %d, want 1", got)
	}
}

func TestRename(t *testing.T) {
	s := empty()
	add(t, s, "old")
	ok(t, s.Rename(1, "new"))
	if got := titles(list(t, s, "all")); !equal(got, []string{"new"}) {
		t.Fatalf("titles = %v", got)
	}
}

func TestDelete(t *testing.T) {
	s := empty()
	add(t, s, "a")
	add(t, s, "b")
	add(t, s, "c")
	ok(t, s.Delete(2))
	if got := titles(list(t, s, "all")); !equal(got, []string{"a", "c"}) {
		t.Fatalf("titles = %v", got)
	}
}

func TestClearDone(t *testing.T) {
	s := empty()
	add(t, s, "a")
	add(t, s, "b")
	add(t, s, "c")
	ok(t, s.Toggle(1))
	ok(t, s.Toggle(3))
	ok(t, s.ClearDone())
	if got := titles(list(t, s, "all")); !equal(got, []string{"b"}) {
		t.Fatalf("titles = %v", got)
	}
}

func TestInTxCommits(t *testing.T) {
	s := empty()
	err := s.InTx(func(tx *Store) error {
		_, err := tx.Add("a")
		return err
	})
	ok(t, err)
	if got := titles(list(t, s, "all")); !equal(got, []string{"a"}) {
		t.Fatalf("titles = %v", got)
	}
}

func TestInTxRollsBackOnError(t *testing.T) {
	s := empty()
	add(t, s, "keep")
	want := errors.New("boom")
	err := s.InTx(func(tx *Store) error {
		if _, err := tx.Add("lost"); err != nil {
			return err
		}
		if err := tx.Delete(1); err != nil {
			return err
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if got := titles(list(t, s, "all")); !equal(got, []string{"keep"}) {
		t.Fatalf("titles after rollback = %v, want [keep]", got)
	}
}

func TestInTxCannotNest(t *testing.T) {
	s := empty()
	err := s.InTx(func(tx *Store) error { return tx.InTx(func(*Store) error { return nil }) })
	if err == nil {
		t.Fatal("nested InTx should error, not deadlock")
	}
}

func TestClosedStoreReturnsErrors(t *testing.T) {
	s := empty()
	ok(t, s.Close())
	if _, err := s.List("all"); err == nil {
		t.Fatal("List on closed store should error")
	}
	if _, err := s.Add("x"); err == nil {
		t.Fatal("Add on closed store should error")
	}
}

// Run with -race.
func TestConcurrentAddList(t *testing.T) {
	s := empty()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, err := s.Add("x"); err != nil {
				t.Error(err)
			}
			if _, err := s.List("all"); err != nil {
				t.Error(err)
			}
			if _, err := s.Remaining(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := len(list(t, s, "all")); n != 20 {
		t.Fatalf("%d todos, want 20", n)
	}
}
