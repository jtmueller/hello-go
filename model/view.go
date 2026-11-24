package model

import "time"

type Todo struct {
	ID      int
	Title   string
	Done    bool
	Created time.Time
}

// View is everything the todo templates need to render.
type View struct {
	Todos     []Todo
	Remaining int
	Filter    string
}
