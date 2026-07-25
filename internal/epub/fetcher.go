package epub

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
)

type ArticleFetcher struct{}

func (ArticleFetcher) FetchFromContent(title, author, content string, tempPath string) (*ReadableArticle, error) {
	readableArticle := &ReadableArticle{
		Title:      title,
		Author:     author,
		Content:    content,
		ImagePaths: map[string]string{},
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))

	if err != nil {
		return nil, fmt.Errorf("creating goquery document: %w", err)
	}

	articleTempPath, err := os.MkdirTemp(tempPath, uuid.NewString())

	if err != nil {
		return nil, fmt.Errorf("creating temp file path: %w", err)
	}

	doc.Find("img").EachWithBreak(func(i int, s *goquery.Selection) bool {
		href, exists := s.Attr("src")

		if !exists {
			return true
		}

		outputFilename, err := fetchImage(href, articleTempPath)

		if err != nil {
			return false
		}

		readableArticle.ImagePaths[href] = outputFilename

		return true
	})

	return readableArticle, nil
}

func (f ArticleFetcher) FetchFromURL(title, author, articleURL string, tempPath string) (*ReadableArticle, error) {
	content, err := fetchReadableHtml(articleURL)

	if err != nil {
		return nil, fmt.Errorf("fetchReadableHtml: %w", err)
	}

	return f.FetchFromContent(title, author, content, tempPath)
}

func fetchReadableHtml(address string) (string, error) {
	resp, err := http.Get(address)

	if err != nil {
		return "", fmt.Errorf("http GET of article at URL %s: %w", address, err)
	}

	defer resp.Body.Close()

	parsedUrl, err := url.Parse(address)

	if err != nil {
		return "", fmt.Errorf("parsing url %s: %w", address, err)
	}

	article, err := readability.FromReader(resp.Body, parsedUrl)

	if err != nil {
		return "", fmt.Errorf("creating Readability reader: %w", err)
	}

	contentBuilder := new(strings.Builder)

	err = article.RenderHTML(contentBuilder)

	if err != nil {
		return "", fmt.Errorf("rendering HTML: %w", err)
	}

	return contentBuilder.String(), nil
}

func fetchImage(imageAddress string, tempDir string) (string, error) {
	parts := strings.Split(imageAddress, "/")
	filename := parts[len(parts)-1]

	imgResp, err := http.Get(imageAddress)

	if err != nil {
		return "", fmt.Errorf("HTTP GET of image URL %s: %w", imageAddress, err)
	}

	defer imgResp.Body.Close()

	outputFile, err := writeFileToTempLocation(imgResp.Body, tempDir, filename)

	if err != nil {
		return "", fmt.Errorf("writing image to temp location %s: %w", filename, err)
	}

	return outputFile, nil
}

func writeFileToTempLocation(reader io.Reader, writePath string, fileName string) (string, error) {
	outputFileName := filepath.Join(writePath, fileName)

	file, err := os.Create(outputFileName)

	if err != nil {
		return "", fmt.Errorf("creating temp file at %s: %w", outputFileName, err)
	}

	defer file.Close()

	_, err = io.Copy(file, reader)

	if err != nil {
		return "", fmt.Errorf("writing file bytes to %s: %w", outputFileName, err)
	}

	return outputFileName, nil
}
