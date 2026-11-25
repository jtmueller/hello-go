package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"hello-go/model"
	"hello-go/views"
)

type app struct{ store *model.Store }

func main() {
	path := os.Getenv("DB_PATH")
	if path == "" {
		path = "todos.db"
	}
	store, err := model.Open(path)
	if err != nil {
		log.Fatalf("open database %q: %v", path, err)
	}
	a := &app{store: store}

	mux := a.routes()

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("listening on http://localhost%s (db: %s)", addr, path)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (a *app) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("GET /{$}", a.page)
	mux.HandleFunc("GET /todos", a.section)
	mux.HandleFunc("POST /todos", a.add)
	mux.HandleFunc("DELETE /todos", a.clearDone)
	mux.HandleFunc("POST /todos/{id}/toggle", a.toggle)
	mux.HandleFunc("PUT /todos/{id}", a.rename)
	mux.HandleFunc("DELETE /todos/{id}", a.remove)
	return mux
}

func filterOf(r *http.Request) string {
	switch f := r.URL.Query().Get("filter"); f {
	case "active", "done":
		return f
	}
	return "all"
}

func (a *app) view(r *http.Request) (model.View, error) {
	f := filterOf(r)
	todos, err := a.store.List(f)
	if err != nil {
		return model.View{}, err
	}
	remaining, err := a.store.Remaining()
	if err != nil {
		return model.View{}, err
	}
	return model.View{Todos: todos, Remaining: remaining, Filter: f}, nil
}

// fail logs a store error and responds 500.
func fail(w http.ResponseWriter, err error) {
	log.Printf("store error: %v", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func (a *app) page(w http.ResponseWriter, r *http.Request) {
	v, err := a.view(r)
	if err != nil {
		fail(w, err)
		return
	}
	templ.Handler(views.Page(v)).ServeHTTP(w, r)
}

func (a *app) renderSection(w http.ResponseWriter, r *http.Request) {
	v, err := a.view(r)
	if err != nil {
		fail(w, err)
		return
	}
	templ.Handler(views.Section(v)).ServeHTTP(w, r)
}

func (a *app) section(w http.ResponseWriter, r *http.Request) { a.renderSection(w, r) }

// mutate runs a store change and, if it succeeds, renders the section.
func (a *app) mutate(w http.ResponseWriter, r *http.Request, change func() error) {
	if err := change(); err != nil {
		fail(w, err)
		return
	}
	a.renderSection(w, r)
}

func (a *app) add(w http.ResponseWriter, r *http.Request) {
	a.mutate(w, r, func() error {
		if title := strings.TrimSpace(r.FormValue("title")); title != "" {
			_, err := a.store.Add(title)
			return err
		}
		return nil
	})
}

func (a *app) toggle(w http.ResponseWriter, r *http.Request) {
	a.mutate(w, r, func() error {
		if id, err := strconv.Atoi(r.PathValue("id")); err == nil {
			return a.store.Toggle(id)
		}
		return nil
	})
}

func (a *app) rename(w http.ResponseWriter, r *http.Request) {
	a.mutate(w, r, func() error {
		id, err := strconv.Atoi(r.PathValue("id"))
		if title := strings.TrimSpace(r.FormValue("title")); err == nil && title != "" {
			return a.store.Rename(id, title)
		}
		return nil
	})
}

func (a *app) remove(w http.ResponseWriter, r *http.Request) {
	a.mutate(w, r, func() error {
		if id, err := strconv.Atoi(r.PathValue("id")); err == nil {
			return a.store.Delete(id)
		}
		return nil
	})
}

func (a *app) clearDone(w http.ResponseWriter, r *http.Request) {
	a.mutate(w, r, a.store.ClearDone)
}
