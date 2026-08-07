package graph

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"

	"github.com/dschmidt/libre-graph-adapter-kit/auth"
)

// fakeGraph serves a minimal OpenCloud Graph API for one space:
//
//	root (id = drive id "sid$spid")
//	├── .space/            (skipped by walks)
//	├── root.txt
//	└── photos/            (id "sid$spid!f1")
//	    ├── a.jpg
//	    └── sub/           (id "sid$spid!f2")
//	        └── b.jpg
func fakeGraph(t *testing.T) *httptest.Server {
	t.Helper()

	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	folder := func(id, name string) map[string]any {
		return map[string]any{"id": id, "name": name, "folder": map[string]any{}}
	}
	file := func(id, name, mime string) map[string]any {
		return map[string]any{"id": id, "name": name, "file": map[string]any{"mimeType": mime}}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/graph/v1.0/me", func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := r.BasicAuth(); !ok {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		writeJSON(w, map[string]any{"id": "u1", "displayName": "Alice", "onPremisesSamAccountName": "alice"})
	})
	mux.HandleFunc("/graph/v1.0/me/drives", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"value": []any{
			map[string]any{"id": "sid$personal", "name": "Alice", "driveType": "personal"},
			map[string]any{"id": "sid$spid", "name": "Photos", "driveType": "project", "description": "our pictures"},
		}})
	})
	mux.HandleFunc("/graph/v1.0/drives/sid$spid", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"id": "sid$spid", "name": "Photos", "driveType": "project"})
	})
	children := map[string][]any{
		"sid$spid":    {folder("sid$spid!dot", ".space"), file("sid$spid!r1", "root.txt", "text/plain"), folder("sid$spid!f1", "photos")},
		"sid$spid!f1": {file("sid$spid!a", "a.jpg", "image/jpeg"), folder("sid$spid!f2", "sub")},
		"sid$spid!f2": {file("sid$spid!b", "b.jpg", "image/jpeg")},
	}
	mux.HandleFunc("/graph/v1.0/drives/sid$spid/items/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/graph/v1.0/drives/sid$spid/items/")
		if itemID, ok := strings.CutSuffix(rest, "/children"); ok {
			kids, found := children[itemID]
			if !found {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, map[string]any{"value": kids})
			return
		}
		if rest == "sid$spid!a" {
			writeJSON(w, file("sid$spid!a", "a.jpg", "image/jpeg"))
			return
		}
		http.NotFound(w, r)
	})
	return httptest.NewServer(mux)
}

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := New(srv.URL, auth.Static(auth.Basic("svc", "token")))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNewValidation(t *testing.T) {
	if _, err := New("", auth.Static(auth.Basic("u", "p"))); err == nil {
		t.Fatal("empty baseURL accepted")
	}
	if _, err := New("https://cloud.example", nil); err == nil {
		t.Fatal("nil source accepted")
	}
}

func TestGetMe(t *testing.T) {
	srv := fakeGraph(t)
	defer srv.Close()

	user, err := newTestClient(t, srv).GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if user.GetDisplayName() != "Alice" {
		t.Fatalf("displayName = %q", user.GetDisplayName())
	}
}

func TestNoCredentials(t *testing.T) {
	srv := fakeGraph(t)
	defer srv.Close()

	// A per-request source with nothing on the context must fail
	// before any request is sent.
	c, err := New(srv.URL, auth.FromRequestContext())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetMe(context.Background()); err == nil {
		t.Fatal("GetMe without credentials succeeded")
	}
	if _, err := c.GetDriveItem(context.Background(), "sid$spid!a"); err != ErrNoCredentials {
		t.Fatalf("GetDriveItem error = %v, want ErrNoCredentials", err)
	}

	// With credentials on the context the same client works.
	ctx := auth.WithCredentials(context.Background(), auth.Basic("alice", "token"))
	if _, err := c.GetMe(ctx); err != nil {
		t.Fatalf("GetMe with context credentials: %v", err)
	}
}

func TestDrives(t *testing.T) {
	srv := fakeGraph(t)
	defer srv.Close()
	c := newTestClient(t, srv)
	ctx := context.Background()

	drives, err := c.MyDrives(ctx)
	if err != nil {
		t.Fatalf("MyDrives: %v", err)
	}
	if len(drives) != 2 {
		t.Fatalf("MyDrives returned %d drives", len(drives))
	}

	d, err := c.DriveByName(ctx, "photos") // case-insensitive
	if err != nil {
		t.Fatalf("DriveByName: %v", err)
	}
	if d.GetId() != "sid$spid" {
		t.Fatalf("DriveByName id = %q", d.GetId())
	}
	if _, err := c.DriveByName(ctx, "nope"); err == nil {
		t.Fatal("DriveByName found a drive that does not exist")
	}

	d, err = c.Drive(ctx, "sid$spid")
	if err != nil {
		t.Fatalf("Drive: %v", err)
	}
	if d.GetName() != "Photos" {
		t.Fatalf("Drive name = %q", d.GetName())
	}
}

func TestGetDriveItem(t *testing.T) {
	srv := fakeGraph(t)
	defer srv.Close()
	c := newTestClient(t, srv)

	item, err := c.GetDriveItem(context.Background(), "sid$spid!a")
	if err != nil {
		t.Fatalf("GetDriveItem: %v", err)
	}
	if item.GetName() != "a.jpg" || item.File == nil {
		t.Fatalf("GetDriveItem = %+v", item)
	}

	if _, err := c.GetDriveItem(context.Background(), "no-separator"); err == nil {
		t.Fatal("malformed resource id accepted")
	}
	if _, err := c.GetDriveItem(context.Background(), "sid$spid!missing"); err == nil {
		t.Fatal("missing item did not error")
	}
}

func TestSplitResourceID(t *testing.T) {
	cases := []struct {
		id, drive, item string
		ok              bool
	}{
		{"sid$spid!opaque", "sid$spid", "opaque", true},
		{"sid$spid!", "", "", false},
		{"!opaque", "", "", false},
		{"plain", "", "", false},
	}
	for _, tc := range cases {
		drive, item, ok := SplitResourceID(tc.id)
		if drive != tc.drive || item != tc.item || ok != tc.ok {
			t.Fatalf("SplitResourceID(%q) = (%q, %q, %v)", tc.id, drive, item, ok)
		}
	}
}

func TestWalkFiles(t *testing.T) {
	srv := fakeGraph(t)
	defer srv.Close()
	c := newTestClient(t, srv)

	type seen struct{ path, parent string }
	var got []seen
	err := c.WalkFiles(context.Background(), "sid$spid", "sid$spid",
		func(path string, _ libregraph.DriveItem, parent *libregraph.DriveItem) error {
			p := ""
			if parent != nil {
				p = parent.GetName()
			}
			got = append(got, seen{path, p})
			return nil
		})
	if err != nil {
		t.Fatalf("WalkFiles: %v", err)
	}

	want := []seen{
		{"root.txt", ""},           // directly under the walk root
		{"photos/a.jpg", "photos"}, // .space was skipped before this
		{"photos/sub/b.jpg", "sub"},
	}
	if len(got) != len(want) {
		t.Fatalf("walk visited %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("walk visit %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestWalkAborts(t *testing.T) {
	srv := fakeGraph(t)
	defer srv.Close()
	c := newTestClient(t, srv)

	calls := 0
	err := c.WalkFiles(context.Background(), "sid$spid", "sid$spid",
		func(string, libregraph.DriveItem, *libregraph.DriveItem) error {
			calls++
			return context.Canceled
		})
	if err == nil || calls != 1 {
		t.Fatalf("walk did not abort on visit error (err=%v, calls=%d)", err, calls)
	}
}
