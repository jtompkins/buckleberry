package epub

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchFromContentPassesContentThrough(t *testing.T) {
	content := `<p>Hello <img src="https://example.com/cover.jpg"> world</p>`

	article, err := (ArticleFetcher{}).FetchFromContent("A Title", "An Author", content)
	if err != nil {
		t.Fatalf("FetchFromContent() error = %v", err)
	}

	if article.Title != "A Title" || article.Author != "An Author" || article.Content != content {
		t.Errorf("article = %#v, want Title/Author/Content passed through unchanged", article)
	}
}

// Images are the builder's job now: fetching content must not touch the network.
func TestFetchFromContentDoesNotDownloadImages(t *testing.T) {
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Path)
	}))
	defer server.Close()

	content := fmt.Sprintf(`<p><img src="%s/cover.jpg"></p>`, server.URL)

	if _, err := (ArticleFetcher{}).FetchFromContent("Title", "Author", content); err != nil {
		t.Fatalf("FetchFromContent() error = %v", err)
	}

	if len(requested) != 0 {
		t.Errorf("fetched %v, want no image requests", requested)
	}
}

func TestFetchFromURLFailsOnInvalidAddress(t *testing.T) {
	_, err := (ArticleFetcher{}).FetchFromURL("")
	if err == nil {
		t.Fatal("FetchFromURL() with an empty address = nil error, want failure")
	}
	if !strings.Contains(err.Error(), "fetchReadableHtml") {
		t.Errorf("error = %v, want it to be wrapped with fetchReadableHtml context", err)
	}
}

func TestFetchFromURLExtractsArticle(t *testing.T) {
	const bodyText = "This paragraph exists purely to give the readability parser enough real prose to recognize this page as the main article content, rather than boilerplate navigation or chrome around the edges of the page."

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/article", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Server-Side Title</title></head>
<body>
<article>
<h1>Server-Side Title</h1>
<p>%s</p>
<p>%s</p>
<img src="%s/image.jpg">
<p>%s</p>
</article>
</body>
</html>`, bodyText, bodyText, server.URL, bodyText)
	})

	article, err := (ArticleFetcher{}).FetchFromURL(server.URL + "/article")
	if err != nil {
		t.Fatalf("FetchFromURL() error = %v", err)
	}

	// FetchFromURL takes no title/author: both are extracted from the page by
	// the readability parser.
	if article.Title != "Server-Side Title" {
		t.Errorf("article.Title = %q, want the title extracted from the page", article.Title)
	}
	if !strings.Contains(article.Content, "readability parser") {
		t.Errorf("article.Content = %q, want it to contain the extracted body text", article.Content)
	}

	// The image reference survives extraction, absolute, for the builder to embed.
	if !strings.Contains(article.Content, server.URL+"/image.jpg") {
		t.Errorf("article.Content = %q, want it to keep the absolute image URL", article.Content)
	}
}
