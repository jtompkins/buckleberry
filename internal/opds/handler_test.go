package opds

import (
	"buckleberry/internal/epub"
	"buckleberry/internal/settings"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Strubbl/wallabago/v9"
	"github.com/gofiber/fiber/v3"
	"github.com/piero-vic/go-linkding"
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
	entry          *wallabago.Item
	entryErr       error
	export         []byte
	exportErr      error
	exportedID     int
	exportedFormat string
	gotEntryID     int
}

type stubSettingsRepo struct {
	settings    *settings.Settings
	settingsErr error
}

func (r *stubSettingsRepo) Get() (*settings.Settings, error) {
	return r.settings, r.settingsErr
}

func (s *stubWallabagClient) GetEntry(id int) (*wallabago.Item, error) {
	s.gotEntryID = id
	return s.entry, s.entryErr
}

type stubArticleFetcher struct {
	article *epub.ReadableArticle
	err     error

	gotTitle    string
	gotAuthor   string
	gotContent  string
	gotTempPath string
	gotURL      string
}

func (s *stubArticleFetcher) FetchFromURL(articleURL, tempPath string) (*epub.ReadableArticle, error) {
	s.gotURL = articleURL
	s.gotTempPath = tempPath
	return s.article, s.err
}

type stubLinkdingClient struct {
	bookmarks    []linkding.Bookmark
	bookmarksErr error
	bookmark     *linkding.Bookmark
	bookmarkErr  error

	gotBookmarkID int
}

func (s *stubLinkdingClient) GetUnread() ([]linkding.Bookmark, error) {
	return s.bookmarks, s.bookmarksErr
}

func (s *stubLinkdingClient) GetBookmark(id int) (*linkding.Bookmark, error) {
	s.gotBookmarkID = id
	return s.bookmark, s.bookmarkErr
}

func (s *stubArticleFetcher) FetchFromContent(title, author, content, tempPath string) (*epub.ReadableArticle, error) {
	s.gotTitle = title
	s.gotAuthor = author
	s.gotContent = content
	s.gotTempPath = tempPath
	return s.article, s.err
}

type stubEPUBBuilder struct {
	output []byte
	err    error

	gotArticle *epub.ReadableArticle
}

func (s *stubEPUBBuilder) Build(article *epub.ReadableArticle, writer io.Writer) error {
	s.gotArticle = article
	if s.err != nil {
		return s.err
	}
	_, err := writer.Write(s.output)
	return err
}

func (s *stubWallabagClient) GetEntries() (*wallabago.Entries, error) {
	return s.entries, s.entriesErr
}

func (s *stubWallabagClient) ExportEntry(id int, format string) ([]byte, error) {
	s.exportedID = id
	s.exportedFormat = format
	return s.export, s.exportErr
}

// The navigation feed advertises only the sources the user has switched on.
func TestGetNavigationFeeds(t *testing.T) {
	tests := []struct {
		name        string
		useWallabag bool
		useLinkding bool
		wantEntries []string
	}{
		{
			name:        "both sources enabled",
			useWallabag: true,
			useLinkding: true,
			wantEntries: []string{"https://books.example.com/opds/wallabag", "https://books.example.com/opds/linkding"},
		},
		{
			name:        "only wallabag enabled",
			useWallabag: true,
			wantEntries: []string{"https://books.example.com/opds/wallabag"},
		},
		{
			name:        "only linkding enabled",
			useLinkding: true,
			wantEntries: []string{"https://books.example.com/opds/linkding"},
		},
		{
			name:        "no sources enabled",
			wantEntries: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubSettingsRepo{settings: &settings.Settings{UseWallabag: tc.useWallabag, UseLinkding: tc.useLinkding}}
			handler := NewHandler(repo, &stubWallabagClient{}, &stubLinkdingClient{}, nil, nil, "https://books.example.com")
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

			if len(feed.Entries) != len(tc.wantEntries) {
				t.Fatalf("entries = %d, want %d", len(feed.Entries), len(tc.wantEntries))
			}
			for i, wantID := range tc.wantEntries {
				entry := feed.Entries[i]
				if entry.ID != wantID {
					t.Errorf("entry[%d] ID = %q, want %q", i, entry.ID, wantID)
				}
				assertRFC3339(t, "entry updated", entry.Updated)
				assertLink(t, entry.Links, "alternate", wantID, ACQ_TYPE)
			}
		})
	}
}

func TestGetNavigationFeedsSettingsError(t *testing.T) {
	repo := &stubSettingsRepo{settingsErr: errors.New("db unavailable")}
	handler := NewHandler(repo, &stubWallabagClient{}, &stubLinkdingClient{}, nil, nil, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds", handler.GetNavigationFeeds)

	response := performRequest(t, app, "/opds")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusInternalServerError)
	}
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
	handler := NewHandler(nil, client, &stubLinkdingClient{}, nil, nil, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/wallabag", handler.GetUnreadWallabagFeed)

	response := performRequest(t, app, "/opds/wallabag")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if got := response.Header.Get("Content-Type"); got != "application/atom+xml; charset=utf-8" {
		t.Errorf("Content-Type = %q, want Atom", got)
	}
	feed := readAtom(t, response)
	if feed.ID != "https://books.example.com/opds/wallabag" {
		t.Errorf("feed ID = %q, want unread feed URL", feed.ID)
	}
	assertRFC3339(t, "feed updated", feed.Updated)
	assertLink(t, feed.Links, "self", "https://books.example.com/opds/wallabag", "")
	if len(feed.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(feed.Entries))
	}

	first := feed.Entries[0]
	if first.ID != "https://books.example.com/opds/wallabag/42" || first.Title != "An article" || first.Author.Name != "Ada, Grace" {
		t.Errorf("first entry metadata = %#v", first)
	}
	if first.Updated != updated.Format(time.RFC3339) {
		t.Errorf("first updated = %q, want %q", first.Updated, updated.Format(time.RFC3339))
	}
	assertLink(t, first.Links, "http://opds-spec.org/acquisition", "https://books.example.com/opds/wallabag/42", "application/epub+zip")

	second := feed.Entries[1]
	if second.Author.Name != "example.com" {
		t.Errorf("second author = %q, want domain fallback", second.Author.Name)
	}
	if second.Updated != created.Format(time.RFC3339) {
		t.Errorf("second updated = %q, want created-at fallback", second.Updated)
	}
}

func TestGetUnreadFeedClientError(t *testing.T) {
	handler := NewHandler(nil, &stubWallabagClient{entriesErr: errors.New("unavailable")}, &stubLinkdingClient{}, nil, nil, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/wallabag", handler.GetUnreadWallabagFeed)

	response := performRequest(t, app, "/opds/wallabag")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusInternalServerError)
	}
}

func TestGetDownload(t *testing.T) {
	client := &stubWallabagClient{export: []byte("epub contents")}
	repo := &stubSettingsRepo{settings: &settings.Settings{UseInternalEpubBuilder: false}}
	handler := NewHandler(repo, client, &stubLinkdingClient{}, nil, nil, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/wallabag/:id", handler.GetWallabagDownload)

	response := performRequest(t, app, "/opds/wallabag/42")
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

func TestGetDownloadUsesInternalBuilder(t *testing.T) {
	client := &stubWallabagClient{entry: &wallabago.Item{
		ID:          42,
		Title:       "An article",
		Content:     "<p>body</p>",
		PublishedBy: []string{"Ada", "Grace"},
	}}
	repo := &stubSettingsRepo{settings: &settings.Settings{UseInternalEpubBuilder: true}}
	wantArticle := &epub.ReadableArticle{Title: "An article", Author: "Ada, Grace", Content: "<p>body</p>"}
	fetcher := &stubArticleFetcher{article: wantArticle}
	builder := &stubEPUBBuilder{output: []byte("built epub bytes")}
	handler := NewHandler(repo, client, &stubLinkdingClient{}, fetcher, builder, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/wallabag/:id", handler.GetWallabagDownload)

	response := performRequest(t, app, "/opds/wallabag/42")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if got := response.Header.Get("Content-Type"); got != "application/epub+zip" {
		t.Errorf("Content-Type = %q, want application/epub+zip", got)
	}
	if body := readBody(t, response); body != "built epub bytes" {
		t.Errorf("body = %q, want %q", body, "built epub bytes")
	}

	if client.gotEntryID != 42 {
		t.Errorf("GetEntry() called with id = %d, want 42", client.gotEntryID)
	}
	if fetcher.gotTitle != "An article" || fetcher.gotAuthor != "Ada, Grace" || fetcher.gotContent != "<p>body</p>" {
		t.Errorf("FetchFromContent() called with title=%q author=%q content=%q, want title=%q author=%q content=%q",
			fetcher.gotTitle, fetcher.gotAuthor, fetcher.gotContent, "An article", "Ada, Grace", "<p>body</p>")
	}
	if fetcher.gotTempPath == "" {
		t.Error("FetchFromContent() called with empty tempPath")
	}
	if builder.gotArticle != wantArticle {
		t.Errorf("Build() called with article = %#v, want the article returned by the fetcher", builder.gotArticle)
	}
}

func TestGetDownloadInternalBuilderEntryFetchError(t *testing.T) {
	client := &stubWallabagClient{entryErr: errors.New("entry unavailable")}
	repo := &stubSettingsRepo{settings: &settings.Settings{UseInternalEpubBuilder: true}}
	handler := NewHandler(repo, client, &stubLinkdingClient{}, &stubArticleFetcher{}, &stubEPUBBuilder{}, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/wallabag/:id", handler.GetWallabagDownload)

	response := performRequest(t, app, "/opds/wallabag/42")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusInternalServerError)
	}
}

func TestGetDownloadInternalBuilderFetchError(t *testing.T) {
	client := &stubWallabagClient{entry: &wallabago.Item{ID: 42, Title: "An article"}}
	repo := &stubSettingsRepo{settings: &settings.Settings{UseInternalEpubBuilder: true}}
	fetcher := &stubArticleFetcher{err: errors.New("fetch failed")}
	handler := NewHandler(repo, client, &stubLinkdingClient{}, fetcher, &stubEPUBBuilder{}, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/wallabag/:id", handler.GetWallabagDownload)

	response := performRequest(t, app, "/opds/wallabag/42")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusInternalServerError)
	}
}

func TestGetDownloadInternalBuilderBuildError(t *testing.T) {
	client := &stubWallabagClient{entry: &wallabago.Item{ID: 42, Title: "An article"}}
	repo := &stubSettingsRepo{settings: &settings.Settings{UseInternalEpubBuilder: true}}
	fetcher := &stubArticleFetcher{article: &epub.ReadableArticle{}}
	builder := &stubEPUBBuilder{err: errors.New("build failed")}
	handler := NewHandler(repo, client, &stubLinkdingClient{}, fetcher, builder, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/wallabag/:id", handler.GetWallabagDownload)

	response := performRequest(t, app, "/opds/wallabag/42")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusInternalServerError)
	}
}

func TestGetDownloadSettingsFetchError(t *testing.T) {
	repo := &stubSettingsRepo{settingsErr: errors.New("db unavailable")}
	handler := NewHandler(repo, &stubWallabagClient{}, &stubLinkdingClient{}, &stubArticleFetcher{}, &stubEPUBBuilder{}, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/wallabag/:id", handler.GetWallabagDownload)

	response := performRequest(t, app, "/opds/wallabag/42")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusInternalServerError)
	}
}

func TestGetDownloadRejectsInvalidID(t *testing.T) {
	repo := &stubSettingsRepo{settings: &settings.Settings{UseInternalEpubBuilder: false}}
	handler := NewHandler(repo, &stubWallabagClient{}, &stubLinkdingClient{}, nil, nil, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/wallabag/:id", handler.GetWallabagDownload)

	response := performRequest(t, app, "/opds/wallabag/not-a-number")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusBadRequest)
	}
}

func TestGetDownloadExportError(t *testing.T) {
	repo := &stubSettingsRepo{settings: &settings.Settings{UseInternalEpubBuilder: false}}
	client := &stubWallabagClient{exportErr: errors.New("export failed")}
	handler := NewHandler(repo, client, &stubLinkdingClient{}, nil, nil, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/wallabag/:id", handler.GetWallabagDownload)

	response := performRequest(t, app, "/opds/wallabag/42")
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

func TestGetUnreadLinkdingFeed(t *testing.T) {
	added := time.Date(2026, time.July, 12, 15, 30, 0, 0, time.UTC)
	olderAdded := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	client := &stubLinkdingClient{bookmarks: []linkding.Bookmark{
		{ID: 7, Title: "A bookmark", URL: "https://example.com/a", WebsiteTitle: "Example", DateAdded: added},
		{ID: 8, Title: "Another bookmark", URL: "https://example.com/b", WebsiteTitle: "Other", DateAdded: olderAdded},
	}}
	handler := NewHandler(nil, &stubWallabagClient{}, client, nil, nil, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/linkding", handler.GetUnreadLinkdingFeed)

	response := performRequest(t, app, "/opds/linkding")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if got := response.Header.Get("Content-Type"); got != "application/atom+xml; charset=utf-8" {
		t.Errorf("Content-Type = %q, want Atom", got)
	}

	feed := readAtom(t, response)
	if feed.ID != "https://books.example.com/opds/linkding" {
		t.Errorf("feed ID = %q, want the linkding feed URL", feed.ID)
	}
	assertLink(t, feed.Links, "self", "https://books.example.com/opds/linkding", "")
	if len(feed.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(feed.Entries))
	}

	first := feed.Entries[0]
	if first.ID != "https://books.example.com/opds/linkding/7" || first.Title != "A bookmark" {
		t.Errorf("first entry metadata = %#v", first)
	}
	// Linkding has no author field, so the site name stands in for one.
	if first.Author.Name != "Example" {
		t.Errorf("first author = %q, want the website title", first.Author.Name)
	}
	if first.Updated != added.Format(time.RFC3339) {
		t.Errorf("first updated = %q, want the date the bookmark was added", first.Updated)
	}
	assertLink(t, first.Links, "http://opds-spec.org/acquisition", "https://books.example.com/opds/linkding/7", "application/epub+zip")

	if second := feed.Entries[1]; second.ID != "https://books.example.com/opds/linkding/8" {
		t.Errorf("second entry ID = %q, want the second bookmark's URL", second.ID)
	}
}

func TestGetUnreadLinkdingFeedClientError(t *testing.T) {
	client := &stubLinkdingClient{bookmarksErr: errors.New("unavailable")}
	handler := NewHandler(nil, &stubWallabagClient{}, client, nil, nil, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/linkding", handler.GetUnreadLinkdingFeed)

	response := performRequest(t, app, "/opds/linkding")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusInternalServerError)
	}
}

// Linkding stores only a URL, so downloads always go through the internal
// builder: fetch the page, then build an EPUB from it.
func TestGetLinkdingDownload(t *testing.T) {
	client := &stubLinkdingClient{bookmark: &linkding.Bookmark{ID: 42, URL: "https://example.com/article", Title: "An article"}}
	wantArticle := &epub.ReadableArticle{Title: "An article", Author: "Ada", Content: "<p>body</p>"}
	fetcher := &stubArticleFetcher{article: wantArticle}
	builder := &stubEPUBBuilder{output: []byte("built epub bytes")}
	handler := NewHandler(nil, &stubWallabagClient{}, client, fetcher, builder, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/linkding/:id", handler.GetLinkdingDownload)

	response := performRequest(t, app, "/opds/linkding/42")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if got := response.Header.Get("Content-Type"); got != "application/epub+zip" {
		t.Errorf("Content-Type = %q, want application/epub+zip", got)
	}
	if body := readBody(t, response); body != "built epub bytes" {
		t.Errorf("body = %q, want %q", body, "built epub bytes")
	}

	if client.gotBookmarkID != 42 {
		t.Errorf("GetBookmark() called with id = %d, want 42", client.gotBookmarkID)
	}
	if fetcher.gotURL != "https://example.com/article" {
		t.Errorf("FetchFromURL() called with %q, want the bookmark's URL", fetcher.gotURL)
	}
	if fetcher.gotTempPath == "" {
		t.Error("FetchFromURL() called with empty tempPath")
	}
	if builder.gotArticle != wantArticle {
		t.Errorf("Build() called with article = %#v, want the article returned by the fetcher", builder.gotArticle)
	}
}

func TestGetLinkdingDownloadRejectsInvalidID(t *testing.T) {
	handler := NewHandler(nil, &stubWallabagClient{}, &stubLinkdingClient{}, &stubArticleFetcher{}, &stubEPUBBuilder{}, "https://books.example.com")
	app := fiber.New()
	app.Get("/opds/linkding/:id", handler.GetLinkdingDownload)

	response := performRequest(t, app, "/opds/linkding/not-a-number")
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusBadRequest)
	}
}

func TestGetLinkdingDownloadErrors(t *testing.T) {
	tests := []struct {
		name    string
		client  *stubLinkdingClient
		fetcher *stubArticleFetcher
		builder *stubEPUBBuilder
	}{
		{
			name:    "bookmark lookup fails",
			client:  &stubLinkdingClient{bookmarkErr: errors.New("bookmark unavailable")},
			fetcher: &stubArticleFetcher{},
			builder: &stubEPUBBuilder{},
		},
		{
			name:    "page fetch fails",
			client:  &stubLinkdingClient{bookmark: &linkding.Bookmark{ID: 42, URL: "https://example.com/article"}},
			fetcher: &stubArticleFetcher{err: errors.New("fetch failed")},
			builder: &stubEPUBBuilder{},
		},
		{
			name:    "epub build fails",
			client:  &stubLinkdingClient{bookmark: &linkding.Bookmark{ID: 42, URL: "https://example.com/article"}},
			fetcher: &stubArticleFetcher{article: &epub.ReadableArticle{}},
			builder: &stubEPUBBuilder{err: errors.New("build failed")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewHandler(nil, &stubWallabagClient{}, tc.client, tc.fetcher, tc.builder, "https://books.example.com")
			app := fiber.New()
			app.Get("/opds/linkding/:id", handler.GetLinkdingDownload)

			response := performRequest(t, app, "/opds/linkding/42")
			defer response.Body.Close()

			if response.StatusCode != fiber.StatusInternalServerError {
				t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusInternalServerError)
			}
		})
	}
}
