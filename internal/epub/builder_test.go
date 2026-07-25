package epub

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
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

func TestBuildProducesValidEpub(t *testing.T) {
	imageBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	imagePath := filepath.Join(t.TempDir(), "cover.jpg")
	if err := os.WriteFile(imagePath, imageBytes, 0o600); err != nil {
		t.Fatalf("write fixture image: %v", err)
	}

	article := &ReadableArticle{
		Title:      "My Article",
		Author:     "Jane Doe",
		Content:    `<p>Hello <img src="cover.jpg"> world</p>`,
		ImagePaths: map[string]string{"cover.jpg": imagePath},
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
	if strings.Contains(string(sectionBody), `src="cover.jpg"`) {
		t.Error("section body still references the original local image src instead of the rewritten epub path")
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
		Title:      "Text Only",
		Author:     "Jane Doe",
		Content:    `<p>Just some text, no pictures.</p>`,
		ImagePaths: map[string]string{},
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

func TestBuildSkipsRemainingImagesWhenOneImagePathIsMissing(t *testing.T) {
	// article.ImagePaths has no entry for "missing.jpg", so the builder asks
	// go-epub to embed an empty source, which fails and stops the
	// EachWithBreak loop -- "after.jpg" never gets processed either, even
	// though it does have a valid path. This test documents that behavior.
	afterPath := filepath.Join(t.TempDir(), "after.jpg")
	if err := os.WriteFile(afterPath, []byte{0x01, 0x02}, 0o600); err != nil {
		t.Fatalf("write fixture image: %v", err)
	}

	article := &ReadableArticle{
		Title:   "Broken Image Reference",
		Author:  "Jane Doe",
		Content: `<p><img src="missing.jpg"><img src="after.jpg"></p>`,
		ImagePaths: map[string]string{
			"after.jpg": afterPath,
		},
	}

	var buf bytes.Buffer
	if err := (EPUBBuilder{}).Build(article, &buf); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	entries := readZipEntries(t, buf.Bytes())
	for name, content := range entries {
		if strings.Contains(strings.ToLower(name), "after.jpg") || bytes.Equal(content, []byte{0x01, 0x02}) {
			t.Errorf("expected after.jpg to be skipped once an earlier image failed, but found entry %q", name)
		}
	}
}
