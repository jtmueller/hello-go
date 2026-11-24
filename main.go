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
	a := &app{store: model.NewStore()}

	mux := a.routes()

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("listening on http://localhost%s", addr)
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

func (a *app) view(r *http.Request) model.View {
	f := filterOf(r)
	return model.View{Todos: a.store.List(f), Remaining: a.store.Remaining(), Filter: f}
}

func (a *app) page(w http.ResponseWriter, r *http.Request) {
	templ.Handler(views.Page(a.view(r))).ServeHTTP(w, r)
}

func (a *app) renderSection(w http.ResponseWriter, r *http.Request) {
	templ.Handler(views.Section(a.view(r))).ServeHTTP(w, r)
}

func (a *app) section(w http.ResponseWriter, r *http.Request) { a.renderSection(w, r) }

func (a *app) add(w http.ResponseWriter, r *http.Request) {
	if title := strings.TrimSpace(r.FormValue("title")); title != "" {
		a.store.Add(title)
	}
	a.renderSection(w, r)
}

func (a *app) toggle(w http.ResponseWriter, r *http.Request) {
	if id, err := strconv.Atoi(r.PathValue("id")); err == nil {
		a.store.Toggle(id)
	}
	a.renderSection(w, r)
}

func (a *app) rename(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if title := strings.TrimSpace(r.FormValue("title")); err == nil && title != "" {
		a.store.Rename(id, title)
	}
	a.renderSection(w, r)
}

func (a *app) remove(w http.ResponseWriter, r *http.Request) {
	if id, err := strconv.Atoi(r.PathValue("id")); err == nil {
		a.store.Delete(id)
	}
	a.renderSection(w, r)
}

func (a *app) clearDone(w http.ResponseWriter, r *http.Request) {
	a.store.ClearDone()
	a.renderSection(w, r)
}
