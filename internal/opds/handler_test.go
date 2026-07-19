package opds

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Strubbl/wallabago/v9"
	"github.com/gofiber/fiber/v3"
)

type atomDocument struct {
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	ID      string     `xml:"id"`
	Title   string     `xml:"title"`
	Updated string     `xml:"updated"`
	Author  atomAuthor `xml:"author"`
	Links   []atomLink `xml:"link"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type stubWallabagClient struct {
	entries        *wallabago.Entries
	entriesErr     error
	export         []byte
	exportErr      error
	exportedID     int
	exportedFormat string
}

func (s *stubWallabagClient) GetEntries() (*wallabago.Entries, error) {
	return s.entries, s.entriesErr
}

func (s *stubWallabagClient) ExportEntry(id int, format string) ([]byte, error) {
	s.exportedID = id
	s.exportedFormat = format
	return s.export, s.exportErr
}

func TestGetNavigationFeeds(t *testing.T) {
	handler := NewHandler(nil, &stubWallabagClient{}, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds", handler.GetNavigationFeeds)

	response := performRequest(t, app, "/opds")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if got := response.Header.Get("Content-Type"); got != "application/atom+xml; charset=utf-8" {
		t.Errorf("Content-Type = %q, want Atom", got)
	}
	feed := readAtom(t, response)
	if feed.ID != "https://books.example.com/opds" {
		t.Errorf("feed ID = %q, want navigation URL", feed.ID)
	}
	assertRFC3339(t, "feed updated", feed.Updated)
	assertLink(t, feed.Links, "self", "https://books.example.com/opds", "")
	if len(feed.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(feed.Entries))
	}
	entry := feed.Entries[0]
	if entry.ID != "https://books.example.com/opds/unread" {
		t.Errorf("entry ID = %q, want unread feed URL", entry.ID)
	}
	assertRFC3339(t, "entry updated", entry.Updated)
	assertLink(t, entry.Links, "alternate", "https://books.example.com/opds/unread", ACQ_TYPE)
}

func TestGetUnreadFeed(t *testing.T) {
	updated := time.Date(2026, time.July, 12, 15, 30, 0, 0, time.UTC)
	created := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	client := &stubWallabagClient{entries: &wallabago.Entries{
		Embedded: wallabago.Embedded{Items: []wallabago.Item{
			{ID: 42, Title: "An article", PublishedBy: []string{"Ada", "Grace"}, UpdatedAt: &wallabago.WallabagTime{Time: updated}},
			{ID: 43, Title: "Another article", DomainName: "example.com", CreatedAt: &wallabago.WallabagTime{Time: created}},
		}},
	}}
	handler := NewHandler(nil, client, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/unread", handler.GetUnreadFeed)

	response := performRequest(t, app, "/opds/unread")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if got := response.Header.Get("Content-Type"); got != "application/atom+xml; charset=utf-8" {
		t.Errorf("Content-Type = %q, want Atom", got)
	}
	feed := readAtom(t, response)
	if feed.ID != "https://books.example.com/opds/unread" {
		t.Errorf("feed ID = %q, want unread feed URL", feed.ID)
	}
	assertRFC3339(t, "feed updated", feed.Updated)
	assertLink(t, feed.Links, "self", "https://books.example.com/opds/unread", "")
	if len(feed.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(feed.Entries))
	}

	first := feed.Entries[0]
	if first.ID != "https://books.example.com/opds/download/42" || first.Title != "An article" || first.Author.Name != "Ada, Grace" {
		t.Errorf("first entry metadata = %#v", first)
	}
	if first.Updated != updated.Format(time.RFC3339) {
		t.Errorf("first updated = %q, want %q", first.Updated, updated.Format(time.RFC3339))
	}
	assertLink(t, first.Links, "http://opds-spec.org/acquisition", "https://books.example.com/opds/download/42", "application/epub+zip")

	second := feed.Entries[1]
	if second.Author.Name != "example.com" {
		t.Errorf("second author = %q, want domain fallback", second.Author.Name)
	}
	if second.Updated != created.Format(time.RFC3339) {
		t.Errorf("second updated = %q, want created-at fallback", second.Updated)
	}
}

func TestGetUnreadFeedClientError(t *testing.T) {
	handler := NewHandler(nil, &stubWallabagClient{entriesErr: errors.New("unavailable")}, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/unread", handler.GetUnreadFeed)

	response := performRequest(t, app, "/opds/unread")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusInternalServerError)
	}
}

func TestGetDownload(t *testing.T) {
	client := &stubWallabagClient{export: []byte("epub contents")}
	handler := NewHandler(nil, client, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/download/:id", handler.GetDownload)

	response := performRequest(t, app, "/opds/download/42")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if client.exportedID != 42 || client.exportedFormat != "epub" {
		t.Errorf("ExportEntry() called with (%d, %q), want (42, %q)", client.exportedID, client.exportedFormat, "epub")
	}
	if got := response.Header.Get("Content-Type"); got != "application/epub+zip" {
		t.Errorf("Content-Type = %q, want application/epub+zip", got)
	}
	if body := readBody(t, response); body != "epub contents" {
		t.Errorf("body = %q, want %q", body, "epub contents")
	}
}

func TestGetDownloadRejectsInvalidID(t *testing.T) {
	handler := NewHandler(nil, &stubWallabagClient{}, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/download/:id", handler.GetDownload)

	response := performRequest(t, app, "/opds/download/not-a-number")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusBadRequest)
	}
}

func TestGetDownloadExportError(t *testing.T) {
	client := &stubWallabagClient{exportErr: errors.New("export failed")}
	handler := NewHandler(nil, client, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/download/:id", handler.GetDownload)

	response := performRequest(t, app, "/opds/download/42")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusInternalServerError)
	}
	if client.exportedID != 42 || client.exportedFormat != "epub" {
		t.Errorf("ExportEntry() called with (%d, %q), want (42, %q)", client.exportedID, client.exportedFormat, "epub")
	}
}

func performRequest(t *testing.T, app *fiber.App, path string) *http.Response {
	t.Helper()

	response, err := app.Test(httptest.NewRequest("GET", path, nil))
	if err != nil {
		t.Fatalf("request %s: %v", path, err)
	}
	return response
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return string(body)
}

func readAtom(t *testing.T, response *http.Response) atomDocument {
	t.Helper()

	body := readBody(t, response)
	var feed atomDocument
	if err := xml.Unmarshal([]byte(body), &feed); err != nil {
		t.Fatalf("parse Atom response: %v\n%s", err, body)
	}
	return feed
}

func assertRFC3339(t *testing.T, name, value string) {
	t.Helper()
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		t.Errorf("%s = %q, want RFC3339 timestamp: %v", name, value, err)
	}
}

func assertLink(t *testing.T, links []atomLink, rel, href, mediaType string) {
	t.Helper()
	for _, link := range links {
		if link.Rel == rel && link.Href == href && link.Type == mediaType {
			return
		}
	}
	t.Errorf("missing link rel=%q href=%q type=%q; got %#v", rel, href, mediaType, links)
}
