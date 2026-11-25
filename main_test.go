package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"hello-go/model"
)

// newServer returns a handler backed by an empty store seeded with the
// given titles, plus the store for direct assertions.
func newServer(titles ...string) (http.Handler, *model.Store) {
	store := model.NewEmptyStore()
	for _, title := range titles {
		if _, err := store.Add(title); err != nil {
			panic(err)
		}
	}
	return (&app{store: store}).routes(), store
}

func do(h http.Handler, method, target string, form url.Values) *httptest.ResponseRecorder {
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func titlesOf(s *model.Store) []string {
	var out []string
	todos, err := s.List("all")
	if err != nil {
		panic(err)
	}
	for _, t := range todos {
		out = append(out, t.Title)
	}
	return out
}

func mustContain(t *testing.T, rec *httptest.ResponseRecorder, subs ...string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, s := range subs {
		if !strings.Contains(body, s) {
			t.Errorf("body missing %q\n%s", s, body)
		}
	}
}

func mustNotContain(t *testing.T, rec *httptest.ResponseRecorder, subs ...string) {
	t.Helper()
	body := rec.Body.String()
	for _, s := range subs {
		if strings.Contains(body, s) {
			t.Errorf("body unexpectedly contains %q", s)
		}
	}
}

func TestPageRendersFullDocument(t *testing.T) {
	h, _ := newServer("Buy milk")
	rec := do(h, "GET", "/", nil)
	mustContain(t, rec, "<!doctype html>", `id="todo-section"`, "Buy milk", "1 left", "/static/app.css")
}

// Clicking a todo's text toggles it (like the checkbox), but must not fire on
// a double-click, which starts inline editing.
func TestItemTextClickTogglesButNotOnDoubleClick(t *testing.T) {
	h, _ := newServer("Buy milk")
	rec := do(h, "GET", "/", nil)
	mustContain(t, rec,
		`hx-trigger="change, toggle"`, // checkbox also listens for the programmatic toggle
		`x-ref="toggle"`,
		"setTimeout",   // single click is deferred...
		"clearTimeout", // ...and cancelled by a double-click
	)
}

func TestFavicon(t *testing.T) {
	h, _ := newServer()
	mustContain(t, do(h, "GET", "/", nil), `rel="icon"`, "/static/favicon.svg")
	for _, path := range []string{"/static/favicon.svg", "/favicon.ico"} {
		rec := do(h, "GET", path, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/svg+xml" {
			t.Errorf("GET %s Content-Type = %q, want image/svg+xml", path, ct)
		}
		if !strings.Contains(rec.Body.String(), "<svg") {
			t.Errorf("GET %s body is not an SVG", path)
		}
	}
}

func TestPageUnknownPathIs404(t *testing.T) {
	h, _ := newServer()
	if rec := do(h, "GET", "/nope", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestSectionIsFragmentOnly(t *testing.T) {
	h, _ := newServer("a")
	rec := do(h, "GET", "/todos", nil)
	mustContain(t, rec, `id="todo-section"`)
	mustNotContain(t, rec, "<html")
}

func TestEmptyState(t *testing.T) {
	h, _ := newServer()
	mustContain(t, do(h, "GET", "/todos", nil), "Nothing here", "0 left")
}

func TestAdd(t *testing.T) {
	h, store := newServer()
	rec := do(h, "POST", "/todos", url.Values{"title": {"  Write tests  "}})
	mustContain(t, rec, "Write tests")
	if got := titlesOf(store); len(got) != 1 || got[0] != "Write tests" {
		t.Fatalf("store = %v, want trimmed title", got)
	}
}

func TestAddIgnoresBlankTitle(t *testing.T) {
	h, store := newServer()
	rec := do(h, "POST", "/todos", url.Values{"title": {"   "}})
	mustContain(t, rec, "Nothing here")
	if len(titlesOf(store)) != 0 {
		t.Fatal("blank title should not be added")
	}
}

func TestAddEscapesHTML(t *testing.T) {
	h, _ := newServer()
	rec := do(h, "POST", "/todos", url.Values{"title": {"<script>alert(1)</script>"}})
	mustContain(t, rec, "&lt;script&gt;")
	mustNotContain(t, rec, "<script>alert(1)")
}

func TestToggle(t *testing.T) {
	h, store := newServer("a", "b")
	rec := do(h, "POST", "/todos/1/toggle", nil)
	mustContain(t, rec, "1 left")
	if todos, _ := store.List("all"); !todos[0].Done {
		t.Fatal("todo 1 should be done")
	}
}

func TestToggleBadIDIsNoop(t *testing.T) {
	h, _ := newServer("a")
	mustContain(t, do(h, "POST", "/todos/abc/toggle", nil), "1 left")
}

func TestRename(t *testing.T) {
	h, store := newServer("old")
	mustContain(t, do(h, "PUT", "/todos/1", url.Values{"title": {" new "}}), "new")
	if got := titlesOf(store); got[0] != "new" {
		t.Fatalf("store = %v", got)
	}
}

func TestRenameBlankKeepsOldTitle(t *testing.T) {
	h, store := newServer("keep")
	mustContain(t, do(h, "PUT", "/todos/1", url.Values{"title": {""}}), "keep")
	if got := titlesOf(store); got[0] != "keep" {
		t.Fatalf("store = %v", got)
	}
}

func TestDelete(t *testing.T) {
	h, store := newServer("a", "b")
	rec := do(h, "DELETE", "/todos/1", nil)
	mustContain(t, rec, "b")
	mustNotContain(t, rec, ">a<")
	if got := titlesOf(store); len(got) != 1 || got[0] != "b" {
		t.Fatalf("store = %v", got)
	}
}

func TestClearDone(t *testing.T) {
	h, store := newServer("a", "b")
	if err := store.Toggle(1); err != nil {
		t.Fatal(err)
	}
	mustContain(t, do(h, "DELETE", "/todos", nil), "1 left")
	if got := titlesOf(store); len(got) != 1 || got[0] != "b" {
		t.Fatalf("store = %v", got)
	}
}

func TestFilterQuery(t *testing.T) {
	h, store := newServer("open", "closed")
	if err := store.Toggle(2); err != nil {
		t.Fatal(err)
	}

	active := do(h, "GET", "/todos?filter=active", nil)
	mustContain(t, active, "open")
	mustNotContain(t, active, ">closed<")

	done := do(h, "GET", "/todos?filter=done", nil)
	mustContain(t, done, "closed")
	mustNotContain(t, done, ">open<")

	// Unknown filters fall back to "all".
	mustContain(t, do(h, "GET", "/todos?filter=bogus", nil), "open", "closed")
}

func TestMutationsPreserveFilter(t *testing.T) {
	h, _ := newServer("a")
	rec := do(h, "POST", "/todos/1/toggle?filter=active", nil)
	// Rendered URLs must carry the current filter so the next action keeps it.
	mustContain(t, rec, "filter=active")
}

func TestFilterIsNotInjectable(t *testing.T) {
	h, _ := newServer("a")
	rec := do(h, "GET", `/todos?filter="><script>x</script>`, nil)
	mustNotContain(t, rec, "<script>x")
}

func TestMethodNotAllowed(t *testing.T) {
	h, _ := newServer()
	if rec := do(h, "PATCH", "/todos", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestStoreFailureReturns500(t *testing.T) {
	h, store := newServer("a")
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, target string
		form           url.Values
	}{
		{"GET", "/", nil},
		{"GET", "/todos", nil},
		{"POST", "/todos", url.Values{"title": {"x"}}},
		{"POST", "/todos/1/toggle", nil},
		{"PUT", "/todos/1", url.Values{"title": {"y"}}},
		{"DELETE", "/todos/1", nil},
		{"DELETE", "/todos", nil},
	} {
		rec := do(h, tc.method, tc.target, tc.form)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s %s = %d, want 500", tc.method, tc.target, rec.Code)
		}
		mustNotContain(t, rec, "todo-section")
	}
}
