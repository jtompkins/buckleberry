package epub

import (
	"fmt"
	"io"

	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-epub"
)

type EPUBBuilder struct{}

func (EPUBBuilder) Build(article *ReadableArticle, writer io.Writer) error {
	outputEpub, err := epub.NewEpub("Output EPUB")

	if err != nil {
		return fmt.Errorf("creating epub: %w", err)
	}

	outputEpub.SetAuthor(article.Author)
	outputEpub.SetTitle(article.Title)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(article.Content))

	if err != nil {
		return fmt.Errorf("creating goquery document: %w", err)
	}

	doc.Find("img").EachWithBreak(func(i int, s *goquery.Selection) bool {
		href, exists := s.Attr("src")

		if !exists {
			return false
		}

		epubImagePath, err := outputEpub.AddImage(article.ImagePaths[href], "")

		if err != nil {
			return false
		}

		s.SetAttr("src", epubImagePath)

		return true
	})

	readableHtml, err := doc.Html()

	if err != nil {
		return fmt.Errorf("writing HTML to epub: %w", err)
	}

	_, err = outputEpub.AddSection(readableHtml, article.Title, "", "")

	if err != nil {
		return fmt.Errorf("adding section to epub: %w", err)
	}

	outputEpub.EmbedImages()

	_, err = outputEpub.WriteTo(writer)

	if err != nil {
		return fmt.Errorf("writing output bytes to writer: %w", err)
	}

	return nil
}
