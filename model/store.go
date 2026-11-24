package model

import (
	"sync"
	"time"
)

// Store is a tiny concurrency-safe in-memory todo store.
type Store struct {
	mu     sync.RWMutex
	nextID int
	todos  []Todo
}

// NewEmptyStore returns a store with no todos.
func NewEmptyStore() *Store { return &Store{nextID: 1} }

// NewStore returns a store seeded with a few example todos.
func NewStore() *Store {
	s := NewEmptyStore()
	s.Add("Read the Templ docs")
	s.Add("Wire up HTMX")
	s.Add("Sprinkle in some Alpine.js")
	return s
}

func (s *Store) Add(title string) Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := Todo{ID: s.nextID, Title: title, Created: time.Now()}
	s.nextID++
	s.todos = append(s.todos, t)
	return t
}

// List returns todos filtered by "all", "active" or "done".
func (s *Store) List(filter string) []Todo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Todo, 0, len(s.todos))
	for _, t := range s.todos {
		if filter == "active" && t.Done || filter == "done" && !t.Done {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (s *Store) Remaining() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, t := range s.todos {
		if !t.Done {
			n++
		}
	}
	return n
}

func (s *Store) Toggle(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.todos {
		if s.todos[i].ID == id {
			s.todos[i].Done = !s.todos[i].Done
		}
	}
}

func (s *Store) Rename(id int, title string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.todos {
		if s.todos[i].ID == id {
			s.todos[i].Title = title
		}
	}
}

func (s *Store) Delete(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.todos {
		if s.todos[i].ID == id {
			s.todos = append(s.todos[:i], s.todos[i+1:]...)
			return
		}
	}
}

func (s *Store) ClearDone() {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.todos[:0]
	for _, t := range s.todos {
		if !t.Done {
			kept = append(kept, t)
		}
	}
	s.todos = kept
}
