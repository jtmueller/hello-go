package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

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

	// Serve until SIGINT/SIGTERM, then drain requests and close the store so
	// SQLite checkpoints the WAL.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{Addr: addr, Handler: mux}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	select {
	case err := <-errc:
		store.Close()
		log.Fatal(err)
	case <-ctx.Done():
	}
	stop()
	log.Print("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	if err := store.Close(); err != nil {
		log.Printf("close database: %v", err)
	}
}

func (a *app) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("GET /favicon.ico", favicon) // browsers ask for this by default
	mux.HandleFunc("GET /{$}", a.page)
	mux.HandleFunc("GET /todos", a.section)
	mux.HandleFunc("POST /todos", a.add)
	mux.HandleFunc("DELETE /todos", a.clearDone)
	mux.HandleFunc("POST /todos/{id}/toggle", a.toggle)
	mux.HandleFunc("PUT /todos/{id}", a.rename)
	mux.HandleFunc("DELETE /todos/{id}", a.remove)
	return mux
}

// favicon serves the SVG icon at the conventional /favicon.ico path.
func favicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	http.ServeFile(w, r, "static/favicon.svg")
}

func filterOf(r *http.Request) string {
	switch f := r.URL.Query().Get("filter"); f {
	case "active", "done":
		return f
	}
	return "all"
}

func (a *app) view(r *http.Request) (model.View, error) { return viewOf(r, a.store) }

func viewOf(r *http.Request, s *model.Store) (model.View, error) {
	f := filterOf(r)
	todos, err := s.List(f)
	if err != nil {
		return model.View{}, err
	}
	remaining, err := s.Remaining()
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

// mutate applies a store change and re-reads the view in one transaction.
// If either step fails nothing is saved, so the 500 is truthful and the
// client can safely retry without creating duplicates.
func (a *app) mutate(w http.ResponseWriter, r *http.Request, change func(s *model.Store) error) {
	var v model.View
	err := a.store.InTx(func(tx *model.Store) error {
		if err := change(tx); err != nil {
			return err
		}
		var err error
		v, err = viewOf(r, tx)
		return err
	})
	if err != nil {
		fail(w, err)
		return
	}
	templ.Handler(views.Section(v)).ServeHTTP(w, r)
}

func (a *app) add(w http.ResponseWriter, r *http.Request) {
	a.mutate(w, r, func(s *model.Store) error {
		if title := strings.TrimSpace(r.FormValue("title")); title != "" {
			_, err := s.Add(title)
			return err
		}
		return nil
	})
}

func (a *app) toggle(w http.ResponseWriter, r *http.Request) {
	a.mutate(w, r, func(s *model.Store) error {
		if id, err := strconv.Atoi(r.PathValue("id")); err == nil {
			return s.Toggle(id)
		}
		return nil
	})
}

func (a *app) rename(w http.ResponseWriter, r *http.Request) {
	a.mutate(w, r, func(s *model.Store) error {
		id, err := strconv.Atoi(r.PathValue("id"))
		if title := strings.TrimSpace(r.FormValue("title")); err == nil && title != "" {
			return s.Rename(id, title)
		}
		return nil
	})
}

func (a *app) remove(w http.ResponseWriter, r *http.Request) {
	a.mutate(w, r, func(s *model.Store) error {
		if id, err := strconv.Atoi(r.PathValue("id")); err == nil {
			return s.Delete(id)
		}
		return nil
	})
}

func (a *app) clearDone(w http.ResponseWriter, r *http.Request) {
	a.mutate(w, r, (*model.Store).ClearDone)
}
