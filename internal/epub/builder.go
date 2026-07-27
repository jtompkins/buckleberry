package epub

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-epub"
)

// How long to wait on any single image the article references.
const imageFetchTimeout = 30 * time.Second

type EPUBBuilder struct{}

func (EPUBBuilder) Build(article *ReadableArticle, writer io.Writer) error {
	outputEpub, err := epub.NewEpub("Output EPUB")

	if err != nil {
		return fmt.Errorf("creating epub: %w", err)
	}

	outputEpub.Client = &http.Client{Timeout: imageFetchTimeout}

	outputEpub.SetAuthor(article.Author)
	outputEpub.SetTitle(article.Title)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(article.Content))

	if err != nil {
		return fmt.Errorf("creating goquery document: %w", err)
	}

	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		// EmbedImages only follows src, and a surviving srcset would take
		// priority over src in most readers -- sending them back to the
		// network for an image we already embedded. So srcset always goes,
		// but not before standing in for a missing src.
		if _, hasSrc := s.Attr("src"); !hasSrc {
			if candidate := srcsetCandidate(s.AttrOr("srcset", "")); candidate != "" {
				s.SetAttr("src", candidate)
			}
		}

		s.RemoveAttr("srcset")
		s.RemoveAttr("sizes")
	})

	// Anything still without a src has no source we can embed, and an empty
	// <img> just renders as a broken image.
	doc.Find("img:not([src])").Remove()

	// AddSection puts this between the section's <body> tags, so hand it the
	// article markup rather than the whole document goquery parsed it into.
	readableHtml, err := doc.Find("body").Html()

	if err != nil {
		return fmt.Errorf("writing HTML to epub: %w", err)
	}

	_, err = outputEpub.AddSection(readableHtml, article.Title, "", "")

	if err != nil {
		return fmt.Errorf("adding section to epub: %w", err)
	}

	// Downloads every image the article references and rewrites its src to
	// point at the copy stored in the EPUB.
	outputEpub.EmbedImages()

	_, err = outputEpub.WriteTo(writer)

	if err != nil {
		return fmt.Errorf("writing output bytes to writer: %w", err)
	}

	return nil
}

// srcsetCandidate picks one URL out of a srcset attribute: the highest
// resolution on offer, or the first candidate when none of them say.
//
// A srcset is a comma-separated list of "<url> [<descriptor>]" candidates,
// where a descriptor is either a width ("1200w") or a pixel density ("2x").
// Following the HTML parsing rules, a candidate's URL runs to the next space,
// and a comma there ends the candidate -- commas inside a URL, as image CDNs
// like to emit, are left alone.
func srcsetCandidate(srcset string) string {
	const whitespace = " \t\r\n\f"

	var bestURL string
	bestResolution := -1.0

	for rest := srcset; ; {
		rest = strings.TrimLeft(rest, whitespace+",")

		if rest == "" {
			break
		}

		var candidateURL string
		candidateURL, rest = cutAny(rest, whitespace)
		resolution := 0.0

		// A comma on the URL ends the candidate outright; otherwise what
		// follows is its descriptor, which ends at the next comma.
		if trimmed := strings.TrimRight(candidateURL, ","); trimmed != candidateURL {
			candidateURL = trimmed
		} else {
			var descriptor string
			descriptor, rest = cutAny(strings.TrimLeft(rest, whitespace), whitespace+",")

			if value, err := strconv.ParseFloat(strings.TrimRight(descriptor, "wx"), 64); err == nil {
				resolution = value
			}
		}

		if resolution > bestResolution {
			bestResolution, bestURL = resolution, candidateURL
		}
	}

	return bestURL
}

// cutAny splits s around the first character in chars, returning everything
// before it and everything from it onwards.
func cutAny(s string, chars string) (before, after string) {
	if i := strings.IndexAny(s, chars); i >= 0 {
		return s[:i], s[i:]
	}

	return s, ""
}
