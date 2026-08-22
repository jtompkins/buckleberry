package epub

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"codeberg.org/readeck/go-readability/v2"
)

type ArticleFetcher struct{}

// FetchFromContent wraps article HTML that a source has already extracted for
// us. Images are left as they appear in the markup: the builder downloads and
// embeds them when it writes the EPUB.
func (ArticleFetcher) FetchFromContent(title, author, content string) (*ReadableArticle, error) {
	return &ReadableArticle{
		Title:   title,
		Author:  author,
		Content: content,
	}, nil
}

func (f ArticleFetcher) FetchFromURL(articleURL string) (*ReadableArticle, error) {
	article, err := fetchReadableHtml(articleURL)

	if err != nil {
		return nil, fmt.Errorf("fetchReadableHtml: %w", err)
	}

	contentBuilder := new(strings.Builder)

	err = article.RenderHTML(contentBuilder)

	if err != nil {
		return nil, fmt.Errorf("rendering HTML: %w", err)
	}

	content := contentBuilder.String()

	return f.FetchFromContent(article.Title(), article.Byline(), content)
}

func fetchReadableHtml(address string) (*readability.Article, error) {
	resp, err := http.Get(address)

	if err != nil {
		return nil, fmt.Errorf("http GET of article at URL %s: %w", address, err)
	}

	defer resp.Body.Close()

	parsedUrl, err := url.Parse(address)

	if err != nil {
		return nil, fmt.Errorf("parsing url %s: %w", address, err)
	}

	article, err := readability.FromReader(resp.Body, parsedUrl)

	if err != nil {
		return nil, fmt.Errorf("creating Readability reader: %w", err)
	}

	return &article, nil

}
