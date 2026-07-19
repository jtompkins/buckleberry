package opds

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

// ampPattern matches an ampersand optionally followed by a valid XML entity
// body (named, decimal, or hex). A lone "&" (no valid entity after it) is the
// malformed case we need to escape. RE2 has no lookahead, so we match the whole
// thing and decide in the replacement func.
var ampPattern = regexp.MustCompile(`&(?:#[0-9]+;|#x[0-9a-fA-F]+;|[A-Za-z][A-Za-z0-9]*;)?`)

// sanitizeEPUB works around a Wallabag/PHPePub export bug where the article
// title is written into the OPF (and sometimes NCX) metadata without escaping
// "&". That single bare ampersand makes those documents invalid XML, so strict
// readers such as Apple Books reject the whole book. We open the ePUB, escape
// bare ampersands in the metadata documents, and re-zip — leaving every other
// entry (content, images, the stored mimetype) byte-for-byte untouched.
func sanitizeEPUB(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open epub: %w", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for _, f := range zr.File {
		if !shouldRepair(f.Name) {
			// Copy preserves the raw entry verbatim, including the
			// first-and-stored mimetype that the ePUB spec requires.
			if err := zw.Copy(f); err != nil {
				return nil, fmt.Errorf("copy %s: %w", f.Name, err)
			}
			continue
		}

		content, err := readZipEntry(f)
		if err != nil {
			return nil, err
		}

		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:     f.Name,
			Method:   f.Method,
			Modified: f.Modified,
		})
		if err != nil {
			return nil, fmt.Errorf("write %s: %w", f.Name, err)
		}
		if _, err := w.Write([]byte(escapeBareAmpersands(string(content)))); err != nil {
			return nil, fmt.Errorf("write %s: %w", f.Name, err)
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize epub: %w", err)
	}

	return buf.Bytes(), nil
}

// shouldRepair reports whether an ePUB entry is a metadata document that
// Wallabag/PHPePub is known to emit with unescaped ampersands. We deliberately
// leave content documents alone: they're generated from sanitized HTML and
// could legitimately contain a bare "&" inside a CDATA section.
func shouldRepair(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".opf", ".ncx":
		return true
	default:
		return false
	}
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", f.Name, err)
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", f.Name, err)
	}
	return content, nil
}

// escapeBareAmpersands replaces "&" that does not begin a valid XML entity
// reference with "&amp;", leaving already-valid entities untouched.
func escapeBareAmpersands(s string) string {
	return ampPattern.ReplaceAllStringFunc(s, func(m string) string {
		if m == "&" {
			return "&amp;"
		}
		return m
	})
}
