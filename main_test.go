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
		store.Add(title)
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
	for _, t := range s.List("all") {
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
	if !store.List("all")[0].Done {
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
	store.Toggle(1)
	mustContain(t, do(h, "DELETE", "/todos", nil), "1 left")
	if got := titlesOf(store); len(got) != 1 || got[0] != "b" {
		t.Fatalf("store = %v", got)
	}
}

func TestFilterQuery(t *testing.T) {
	h, store := newServer("open", "closed")
	store.Toggle(2)

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
