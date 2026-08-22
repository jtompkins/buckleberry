package linkding

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *[]string) {
	t.Helper()

	var requested []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.RequestURI())
		handler(w, r)
	}))
	t.Cleanup(server.Close)

	client := NewClient()
	client.Configure(server.URL, "test-token")

	return client, &requested
}

// Every call must fail cleanly rather than panic when Configure was never
// called -- that is the state the client is in before onboarding completes.
func TestUnconfiguredClientReturnsErrors(t *testing.T) {
	client := NewClient()

	if err := client.Ping(); err == nil {
		t.Error("Ping() on an unconfigured client = nil error, want failure")
	}
	if _, err := client.GetUnread(); err == nil {
		t.Error("GetUnread() on an unconfigured client = nil error, want failure")
	}
	if _, err := client.GetBookmark(1); err == nil {
		t.Error("GetBookmark() on an unconfigured client = nil error, want failure")
	}
}

func TestPing(t *testing.T) {
	client, requested := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"count":0,"results":[]}`))
	})

	if err := client.Ping(); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if len(*requested) != 1 || !strings.HasPrefix((*requested)[0], "/api/tags") {
		t.Errorf("Ping() requested %v, want a call to /api/tags", *requested)
	}
}

func TestPingReportsServerFailure(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	err := client.Ping()
	if err == nil {
		t.Fatal("Ping() against an unauthorized server = nil error, want failure")
	}
	if !strings.Contains(err.Error(), "error pinging server") {
		t.Errorf("error = %v, want it wrapped with ping context", err)
	}
}

func TestPingSendsAPIKey(t *testing.T) {
	var gotAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"count":0,"results":[]}`))
	}))
	defer server.Close()

	client := NewClient()
	client.Configure(server.URL, "secret-key")

	if err := client.Ping(); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if gotAuth != "Token secret-key" {
		t.Errorf("Authorization header = %q, want the configured API key", gotAuth)
	}
}

func TestGetUnread(t *testing.T) {
	client, requested := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"count": 2,
			"results": [
				{"id": 1, "url": "https://example.com/a", "title": "First", "website_title": "Example", "unread": true},
				{"id": 2, "url": "https://example.com/b", "title": "Second", "website_title": "Example", "unread": true}
			]
		}`))
	})

	bookmarks, err := client.GetUnread()
	if err != nil {
		t.Fatalf("GetUnread() error = %v", err)
	}
	if len(bookmarks) != 2 {
		t.Fatalf("GetUnread() returned %d bookmarks, want 2", len(bookmarks))
	}
	if bookmarks[0].ID != 1 || bookmarks[0].Title != "First" || bookmarks[0].URL != "https://example.com/a" {
		t.Errorf("first bookmark = %#v, want the decoded first result", bookmarks[0])
	}

	// The whole point of this call is that it asks only for unread bookmarks.
	if len(*requested) != 1 || !strings.Contains((*requested)[0], "unread=yes") {
		t.Errorf("GetUnread() requested %v, want an unread filter", *requested)
	}
}

func TestGetUnreadReportsServerFailure(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.GetUnread()
	if err == nil {
		t.Fatal("GetUnread() against a failing server = nil error, want failure")
	}
	if !strings.Contains(err.Error(), "error fetching bookmarks") {
		t.Errorf("error = %v, want it wrapped with fetch context", err)
	}
}

func TestGetBookmark(t *testing.T) {
	client, requested := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id": 42, "url": "https://example.com/article", "title": "An article", "website_title": "Example"}`))
	})

	bookmark, err := client.GetBookmark(42)
	if err != nil {
		t.Fatalf("GetBookmark() error = %v", err)
	}
	if bookmark.ID != 42 || bookmark.URL != "https://example.com/article" || bookmark.Title != "An article" {
		t.Errorf("bookmark = %#v, want the decoded bookmark", bookmark)
	}
	if len(*requested) != 1 || !strings.Contains((*requested)[0], "/42") {
		t.Errorf("GetBookmark() requested %v, want the bookmark's own endpoint", *requested)
	}
}

func TestGetBookmarkReportsServerFailure(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetBookmark(42)
	if err == nil {
		t.Fatal("GetBookmark() for a missing bookmark = nil error, want failure")
	}
	if !strings.Contains(err.Error(), "error fetching bookmark 42") {
		t.Errorf("error = %v, want it wrapped with the bookmark ID", err)
	}
}
