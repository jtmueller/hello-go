package model

import "testing"

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

func TestNewStoreSeeds(t *testing.T) {
	s := NewStore()
	if got := len(s.List("all")); got != 3 {
		t.Fatalf("seeded todos = %d, want 3", got)
	}
	if s.Remaining() != 3 {
		t.Fatalf("remaining = %d, want 3", s.Remaining())
	}
}

func TestAddAssignsIncreasingIDs(t *testing.T) {
	s := empty()
	a, b := s.Add("a"), s.Add("b")
	if a.ID != 1 || b.ID != 2 {
		t.Fatalf("ids = %d,%d want 1,2", a.ID, b.ID)
	}
	if a.Done || a.Created.IsZero() {
		t.Fatalf("new todo should be not done with a created time: %+v", a)
	}
}

func TestIDsNotReusedAfterDelete(t *testing.T) {
	s := empty()
	s.Add("a")
	s.Delete(1)
	if got := s.Add("b").ID; got != 2 {
		t.Fatalf("id after delete = %d, want 2", got)
	}
}

func TestToggle(t *testing.T) {
	s := empty()
	s.Add("a")
	s.Toggle(1)
	if !s.List("all")[0].Done {
		t.Fatal("expected done after first toggle")
	}
	s.Toggle(1)
	if s.List("all")[0].Done {
		t.Fatal("expected not done after second toggle")
	}
	s.Toggle(99) // unknown id is a no-op
}

func TestListFilters(t *testing.T) {
	s := empty()
	s.Add("open")
	s.Add("closed")
	s.Toggle(2)

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
		if got := titles(s.List(tc.filter)); !equal(got, tc.want) {
			t.Errorf("List(%q) = %v, want %v", tc.filter, got, tc.want)
		}
	}
}

func TestListReturnsCopy(t *testing.T) {
	s := empty()
	s.Add("a")
	s.List("all")[0].Title = "mutated"
	if got := s.List("all")[0].Title; got != "a" {
		t.Fatalf("store modified via returned slice: %q", got)
	}
}

func TestRemaining(t *testing.T) {
	s := empty()
	s.Add("a")
	s.Add("b")
	s.Toggle(1)
	if got := s.Remaining(); got != 1 {
		t.Fatalf("remaining = %d, want 1", got)
	}
}

func TestRename(t *testing.T) {
	s := empty()
	s.Add("old")
	s.Rename(1, "new")
	s.Rename(99, "ignored")
	if got := titles(s.List("all")); !equal(got, []string{"new"}) {
		t.Fatalf("titles = %v", got)
	}
}

func TestDelete(t *testing.T) {
	s := empty()
	s.Add("a")
	s.Add("b")
	s.Add("c")
	s.Delete(2)
	s.Delete(99)
	if got := titles(s.List("all")); !equal(got, []string{"a", "c"}) {
		t.Fatalf("titles = %v", got)
	}
}

func TestClearDone(t *testing.T) {
	s := empty()
	s.Add("a")
	s.Add("b")
	s.Add("c")
	s.Toggle(1)
	s.Toggle(3)
	s.ClearDone()
	if got := titles(s.List("all")); !equal(got, []string{"b"}) {
		t.Fatalf("titles = %v", got)
	}
}

// Run with -race to verify the locking.
func TestConcurrentAccess(t *testing.T) {
	s := empty()
	done := make(chan struct{})
	for range 8 {
		go func() {
			defer func() { done <- struct{}{} }()
			for range 100 {
				todo := s.Add("x")
				s.Toggle(todo.ID)
				s.List("all")
				s.Remaining()
				s.Rename(todo.ID, "y")
				s.Delete(todo.ID)
			}
		}()
	}
	for range 8 {
		<-done
	}
	if n := len(s.List("all")); n != 0 {
		t.Fatalf("%d todos left, want 0", n)
	}
}
