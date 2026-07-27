package epub

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func readZipEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open result as zip: %v", err)
	}

	entries := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read entry %s: %v", f.Name, err)
		}
		entries[f.Name] = content
	}
	return entries
}

// newImageServer serves the same image bytes at any path and records what was
// requested, so tests can assert which images the builder went and fetched.
func newImageServer(t *testing.T, imageBytes []byte) (*httptest.Server, *[]string) {
	t.Helper()

	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Path)
		w.Write(imageBytes)
	}))
	t.Cleanup(server.Close)

	return server, &requested
}

func TestBuildProducesValidEpub(t *testing.T) {
	imageBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	server, _ := newImageServer(t, imageBytes)

	article := &ReadableArticle{
		Title:   "My Article",
		Author:  "Jane Doe",
		Content: fmt.Sprintf(`<p>Hello <img src="%s/cover.jpg"> world</p>`, server.URL),
	}

	var buf bytes.Buffer
	if err := (EPUBBuilder{}).Build(article, &buf); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	entries := readZipEntries(t, buf.Bytes())

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open result as zip: %v", err)
	}
	first := zr.File[0]
	if first.Name != "mimetype" {
		t.Fatalf("first entry = %q, want mimetype", first.Name)
	}
	if first.Method != zip.Store {
		t.Errorf("mimetype method = %d, want Store (%d)", first.Method, zip.Store)
	}
	if string(entries["mimetype"]) != "application/epub+zip" {
		t.Errorf("mimetype contents = %q, want application/epub+zip", entries["mimetype"])
	}

	var sectionBody, imageEntry []byte
	for name, content := range entries {
		if strings.Contains(string(content), "Hello") {
			sectionBody = content
		}
		if strings.Contains(strings.ToLower(name), "cover.jpg") {
			imageEntry = content
		}
	}
	if sectionBody == nil {
		t.Fatal("no section entry containing article body text found")
	}
	if !strings.Contains(string(sectionBody), "world") {
		t.Errorf("section body = %s, want it to contain %q", sectionBody, "world")
	}
	if strings.Contains(string(sectionBody), server.URL) {
		t.Error("section body still references the remote image src instead of the rewritten epub path")
	}
	if imageEntry == nil {
		t.Fatal("embedded image entry not found in output epub")
	}
	if !bytes.Equal(imageEntry, imageBytes) {
		t.Errorf("embedded image bytes = %v, want %v", imageEntry, imageBytes)
	}
}

func TestBuildWithoutImages(t *testing.T) {
	article := &ReadableArticle{
		Title:   "Text Only",
		Author:  "Jane Doe",
		Content: `<p>Just some text, no pictures.</p>`,
	}

	var buf bytes.Buffer
	if err := (EPUBBuilder{}).Build(article, &buf); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	entries := readZipEntries(t, buf.Bytes())

	var found bool
	for _, content := range entries {
		if strings.Contains(string(content), "Just some text, no pictures.") {
			found = true
			break
		}
	}
	if !found {
		t.Error("output epub missing article text")
	}
}

// One unreachable image must not cost us the rest of the article's images.
func TestBuildEmbedsRemainingImagesAfterOneFails(t *testing.T) {
	imageBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 'a', 'f', 't', 'e', 'r'}
	server, _ := newImageServer(t, imageBytes)

	article := &ReadableArticle{
		Title:  "Broken Image Reference",
		Author: "Jane Doe",
		Content: fmt.Sprintf(
			`<p><img src="http://127.0.0.1:1/missing.jpg"><img src="%s/after.jpg"></p>`,
			server.URL),
	}

	var buf bytes.Buffer
	if err := (EPUBBuilder{}).Build(article, &buf); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	var found bool
	for _, content := range readZipEntries(t, buf.Bytes()) {
		if bytes.Equal(content, imageBytes) {
			found = true
		}
	}
	if !found {
		t.Error("after.jpg was not embedded, want an earlier failed image not to stop the ones behind it")
	}
}

// srcset wins over src wherever a reader supports it, so it has to go: leaving
// it in sends the reader back to the network for an image we just embedded.
func TestBuildStripsSrcsetSoReadersUseTheEmbeddedImage(t *testing.T) {
	imageBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 'p', 'i', 'c'}
	server, requested := newImageServer(t, imageBytes)

	article := &ReadableArticle{
		Title:  "Responsive Images",
		Author: "Jane Doe",
		Content: fmt.Sprintf(
			`<p><img src="%s/src.jpg" srcset="%s/small.jpg 480w, %s/big.jpg 1200w" sizes="(max-width: 600px) 480px, 1200px"></p>`,
			server.URL, server.URL, server.URL),
	}

	var buf bytes.Buffer
	if err := (EPUBBuilder{}).Build(article, &buf); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// go-epub checks each source with a HEAD before the GET, so the src image
	// legitimately shows up more than once; no srcset candidate may appear.
	for _, path := range *requested {
		if path != "/src.jpg" {
			t.Errorf("fetched %v, want only the src image", *requested)
			break
		}
	}

	var sectionBody []byte
	for _, content := range readZipEntries(t, buf.Bytes()) {
		if strings.Contains(string(content), "img") && strings.Contains(string(content), "Responsive Images") {
			sectionBody = content
		}
	}
	if sectionBody == nil {
		t.Fatal("no section entry containing the article body found")
	}
	if strings.Contains(string(sectionBody), "srcset") || strings.Contains(string(sectionBody), "sizes=") {
		t.Errorf("section body = %s, want srcset and sizes stripped", sectionBody)
	}
	if strings.Contains(string(sectionBody), server.URL) {
		t.Errorf("section body = %s, want no remote image URLs left", sectionBody)
	}
}

// An image whose only source is a srcset still has to make it into the EPUB.
func TestBuildEmbedsSrcsetOnlyImages(t *testing.T) {
	imageBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 'p', 'i', 'c'}
	server, requested := newImageServer(t, imageBytes)

	article := &ReadableArticle{
		Title:  "Srcset Only",
		Author: "Jane Doe",
		Content: fmt.Sprintf(
			`<p>before<img srcset="%s/small.jpg 480w, %s/big.jpg 1200w">after</p>`,
			server.URL, server.URL),
	}

	var buf bytes.Buffer
	if err := (EPUBBuilder{}).Build(article, &buf); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// The highest-resolution candidate is the one worth carrying around.
	for _, path := range *requested {
		if path != "/big.jpg" {
			t.Errorf("fetched %v, want only the widest srcset candidate", *requested)
			break
		}
	}

	var found bool
	for _, content := range readZipEntries(t, buf.Bytes()) {
		if bytes.Equal(content, imageBytes) {
			found = true
		}
	}
	if !found {
		t.Error("srcset-only image was not embedded in the epub")
	}
}

// With no source at all there is nothing to embed, and an empty <img> just
// renders as a broken image.
func TestBuildDropsImagesWithNoSourceAtAll(t *testing.T) {
	article := &ReadableArticle{
		Title:   "No Source",
		Author:  "Jane Doe",
		Content: `<p>before<img alt="decorative">after</p>`,
	}

	var buf bytes.Buffer
	if err := (EPUBBuilder{}).Build(article, &buf); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	for _, content := range readZipEntries(t, buf.Bytes()) {
		if !strings.Contains(string(content), "before") {
			continue
		}
		if strings.Contains(string(content), "<img") {
			t.Errorf("section body = %s, want the sourceless img dropped", content)
		}
	}
}

// AddSection writes its argument between the section's own <body> tags, so a
// full document here would nest <html>/<body> inside the section's body.
func TestBuildDoesNotNestADocumentInsideTheSection(t *testing.T) {
	article := &ReadableArticle{
		Title:   "Well Formed",
		Author:  "Jane Doe",
		Content: `<p>Just some text.</p>`,
	}

	var buf bytes.Buffer
	if err := (EPUBBuilder{}).Build(article, &buf); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	for name, content := range readZipEntries(t, buf.Bytes()) {
		if !strings.Contains(string(content), "Just some text.") {
			continue
		}
		body := string(content)
		if strings.Count(body, "<body") != 1 || strings.Count(body, "<html") != 1 {
			t.Errorf("section %s = %s, want exactly one <html> and one <body>", name, body)
		}
	}
}

func TestSrcsetCandidate(t *testing.T) {
	tests := []struct {
		name   string
		srcset string
		want   string
	}{
		{"empty", "", ""},
		{"single url, no descriptor", "a.jpg", "a.jpg"},
		{"widths", "small.jpg 480w, big.jpg 1200w", "big.jpg"},
		{"widths out of order", "big.jpg 1200w, small.jpg 480w", "big.jpg"},
		{"densities", "one.jpg 1x, three.jpg 3x, two.jpg 2x", "three.jpg"},
		{"no descriptors takes the first", "a.jpg, b.jpg", "a.jpg"},
		{"no space after comma", "a.jpg 480w,b.jpg 1200w", "b.jpg"},
		{"commas inside the url", "https://cdn/w_100,c_scale/a.jpg 480w, https://cdn/w_900,c_scale/b.jpg 900w", "https://cdn/w_900,c_scale/b.jpg"},
		{"newlines and padding", "\n  small.jpg   480w,\n  big.jpg   1200w\n", "big.jpg"},
		{"unparseable descriptor", "a.jpg junk, b.jpg 800w", "b.jpg"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := srcsetCandidate(tc.srcset); got != tc.want {
				t.Errorf("srcsetCandidate(%q) = %q, want %q", tc.srcset, got, tc.want)
			}
		})
	}
}
