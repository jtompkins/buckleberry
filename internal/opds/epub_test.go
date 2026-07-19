package opds

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"testing"
)

func TestEscapeBareAmpersands(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bare ampersand", "Dungeons & Dragons", "Dungeons &amp; Dragons"},
		{"already escaped", "Tom &amp; Jerry", "Tom &amp; Jerry"},
		{"numeric entity", "quote &#39; here", "quote &#39; here"},
		{"hex entity", "dash &#x2013; here", "dash &#x2013; here"},
		{"mixed", "A & B &amp; C", "A &amp; B &amp; C"},
		{"trailing", "trailing &", "trailing &amp;"},
		{"entity-looking but no semicolon", "R&D", "R&amp;D"},
		{"no ampersand", "nothing to do", "nothing to do"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapeBareAmpersands(tc.in); got != tc.want {
				t.Errorf("escapeBareAmpersands(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

const brokenOPF = `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0">
  <metadata><dc:title>Dungeons & Dragons</dc:title></metadata>
</package>`

// buildEPUB assembles a minimal ePUB with a spec-compliant (first, stored)
// mimetype, a binary asset, and a deliberately broken OPF.
func buildEPUB(t *testing.T, image []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	mt, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mt.Write([]byte("application/epub+zip")); err != nil {
		t.Fatal(err)
	}

	entries := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container/>`,
		"OEBPS/book.opf":         brokenOPF,
	}
	for name, body := range entries {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}

	img, err := zw.CreateHeader(&zip.FileHeader{Name: "OEBPS/cover.jpg", Method: zip.Deflate})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := img.Write(image); err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func readEntry(t *testing.T, data []byte, name string) ([]byte, *zip.File) {
	t.Helper()

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open result zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()
			body, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			return body, f
		}
	}
	t.Fatalf("entry %q not found in result", name)
	return nil, nil
}

func TestSanitizeEPUBFixesOPF(t *testing.T) {
	image := []byte{0xFF, 0xD8, 0xFF, 0x00, 0x26, 0x01, 0x02} // includes a raw 0x26 ('&') byte
	in := buildEPUB(t, image)

	out, err := sanitizeEPUB(in)
	if err != nil {
		t.Fatalf("sanitizeEPUB() error = %v", err)
	}

	// The OPF must now be well-formed XML (scan every token).
	opf, _ := readEntry(t, out, "OEBPS/book.opf")
	dec := xml.NewDecoder(bytes.NewReader(opf))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("repaired OPF is not well-formed: %v\n%s", err, opf)
		}
	}
	if bytes.Contains(opf, []byte("Dungeons & Dragons")) {
		t.Error("repaired OPF still contains a bare ampersand")
	}
	if !bytes.Contains(opf, []byte("Dungeons &amp; Dragons")) {
		t.Errorf("repaired OPF missing escaped title:\n%s", opf)
	}
}

func TestSanitizeEPUBPreservesMimetypeAndBinaries(t *testing.T) {
	image := []byte{0xFF, 0xD8, 0xFF, 0x00, 0x26, 0x01, 0x02}
	out, err := sanitizeEPUB(buildEPUB(t, image))
	if err != nil {
		t.Fatalf("sanitizeEPUB() error = %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}

	// mimetype must remain the first entry, stored (uncompressed), exact bytes.
	first := zr.File[0]
	if first.Name != "mimetype" {
		t.Fatalf("first entry = %q, want mimetype", first.Name)
	}
	if first.Method != zip.Store {
		t.Errorf("mimetype method = %d, want Store (%d)", first.Method, zip.Store)
	}
	mt, _ := readEntry(t, out, "mimetype")
	if string(mt) != "application/epub+zip" {
		t.Errorf("mimetype = %q, want application/epub+zip", mt)
	}

	// Binary assets must survive byte-for-byte (the 0x26 byte must NOT be touched).
	img, _ := readEntry(t, out, "OEBPS/cover.jpg")
	if !bytes.Equal(img, image) {
		t.Errorf("cover.jpg was altered: got %v, want %v", img, image)
	}
}

func TestSanitizeEPUBRejectsNonZip(t *testing.T) {
	if _, err := sanitizeEPUB([]byte("this is not a zip")); err == nil {
		t.Fatal("sanitizeEPUB(non-zip) = nil error, want failure")
	}
}
