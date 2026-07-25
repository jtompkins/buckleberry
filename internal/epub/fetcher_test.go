package epub

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchFromContentDownloadsImages(t *testing.T) {
	imageBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 'c', 'o', 'v', 'e', 'r'}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(imageBytes)
	}))
	defer server.Close()

	imageURL := server.URL + "/cover.jpg"
	content := fmt.Sprintf(`<p>Hello <img src="%s"> world</p>`, imageURL)

	article, err := (ArticleFetcher{}).FetchFromContent("A Title", "An Author", content, t.TempDir())
	if err != nil {
		t.Fatalf("FetchFromContent() error = %v", err)
	}

	if article.Title != "A Title" || article.Author != "An Author" || article.Content != content {
		t.Errorf("article metadata = %#v, want Title/Author/Content passed through unchanged", article)
	}

	localPath, ok := article.ImagePaths[imageURL]
	if !ok {
		t.Fatalf("ImagePaths missing entry for %q: %#v", imageURL, article.ImagePaths)
	}

	got, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("read downloaded image at %s: %v", localPath, err)
	}
	if string(got) != string(imageBytes) {
		t.Errorf("downloaded image bytes = %v, want %v", got, imageBytes)
	}
}

func TestFetchFromContentSkipsImgTagsWithoutSrc(t *testing.T) {
	article, err := (ArticleFetcher{}).FetchFromContent("Title", "Author", `<p><img>no src here</p>`, t.TempDir())
	if err != nil {
		t.Fatalf("FetchFromContent() error = %v", err)
	}
	if len(article.ImagePaths) != 0 {
		t.Errorf("ImagePaths = %#v, want empty", article.ImagePaths)
	}
}

func TestFetchFromContentStopsAfterFirstImageDownloadError(t *testing.T) {
	imageBytes := []byte{0x01, 0x02, 0x03}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(imageBytes)
	}))
	defer server.Close()

	goodURL := server.URL + "/good.jpg"
	content := fmt.Sprintf(`<p><img src="://not-a-valid-url"><img src="%s"></p>`, goodURL)

	article, err := (ArticleFetcher{}).FetchFromContent("Title", "Author", content, t.TempDir())
	if err != nil {
		t.Fatalf("FetchFromContent() error = %v", err)
	}

	// The first (malformed) image URL fails to download, which currently
	// breaks the whole EachWithBreak loop -- the second, otherwise-valid,
	// image never gets fetched either.
	if _, ok := article.ImagePaths[goodURL]; ok {
		t.Error("ImagePaths contains an entry for the image after the one that failed to download, want it skipped")
	}
}

func TestFetchFromContentFailsWhenTempPathIsNotADirectory(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	_, err := (ArticleFetcher{}).FetchFromContent("Title", "Author", "<p>text</p>", notADir)
	if err == nil {
		t.Fatal("FetchFromContent() with a non-directory tempPath = nil error, want failure")
	}
}

func TestFetchFromURLFailsOnInvalidAddress(t *testing.T) {
	_, err := (ArticleFetcher{}).FetchFromURL("Title", "Author", "", t.TempDir())
	if err == nil {
		t.Fatal("FetchFromURL() with an empty address = nil error, want failure")
	}
	if !strings.Contains(err.Error(), "fetchReadableHtml") {
		t.Errorf("error = %v, want it to be wrapped with fetchReadableHtml context", err)
	}
}

func TestFetchFromURLFullPipeline(t *testing.T) {
	imageBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 'p', 'i', 'c'}
	const bodyText = "This paragraph exists purely to give the readability parser enough real prose to recognize this page as the main article content, rather than boilerplate navigation or chrome around the edges of the page."

	mux := http.NewServeMux()
	mux.HandleFunc("/image.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Write(imageBytes)
	})
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

	article, err := (ArticleFetcher{}).FetchFromURL("Passed-In Title", "Passed-In Author", server.URL+"/article", t.TempDir())
	if err != nil {
		t.Fatalf("FetchFromURL() error = %v", err)
	}

	if article.Title != "Passed-In Title" || article.Author != "Passed-In Author" {
		t.Errorf("article Title/Author = %q/%q, want the values passed to FetchFromURL, not anything extracted from the page", article.Title, article.Author)
	}
	if !strings.Contains(article.Content, "readability parser") {
		t.Errorf("article.Content = %q, want it to contain the extracted body text", article.Content)
	}

	if len(article.ImagePaths) != 1 {
		t.Fatalf("ImagePaths = %#v, want exactly one downloaded image", article.ImagePaths)
	}
	for _, localPath := range article.ImagePaths {
		got, err := os.ReadFile(localPath)
		if err != nil {
			t.Fatalf("read downloaded image at %s: %v", localPath, err)
		}
		if string(got) != string(imageBytes) {
			t.Errorf("downloaded image bytes = %v, want %v", got, imageBytes)
		}
	}
}
