package adapter

import (
	"fmt"
	"strings"

	"github.com/Strubbl/wallabago/v9"
)

type wallabagClient interface {
	GetEntries() (*wallabago.Entries, error)
	ExportEntry(id int, format string) ([]byte, error)
	GetEntry(id int) (*wallabago.Item, error)
	Ping() error
}

type WallabagAdapter struct {
	client wallabagClient
}

func NewWallabagAdapter(client wallabagClient) *WallabagAdapter {
	return &WallabagAdapter{
		client: client,
	}
}

func (wa *WallabagAdapter) Name() string {
	return "wallabag"
}

func (wa *WallabagAdapter) Feeds() ([]string, error) {
	return []string{"unread"}, nil
}

func (wa *WallabagAdapter) Articles(feed string) ([]Article, error) {
	entries, err := wa.client.GetEntries()

	if err != nil {
		return nil, fmt.Errorf("unable to fetch Wallabag entries: %v", err)
	}

	articles := []Article{}

	for _, entry := range entries.Embedded.Items {
		articles = append(articles, entryToArticle(entry))
	}

	return articles, nil
}

func (wa *WallabagAdapter) Download(id int) ([]byte, error) {
	epubBytes, err := wa.client.ExportEntry(id, "epub")

	if err != nil {
		return nil, fmt.Errorf("unable to download Wallabag article %d: %v", id, err)
	}

	return epubBytes, nil
}

func (wa *WallabagAdapter) Ping() error {
	return wa.client.Ping()
}

func entryToArticle(entry wallabago.Item) Article {
	var author string
	authorCount := len(entry.PublishedBy)

	if authorCount == 0 {
		author = entry.DomainName
	} else {
		author = strings.Join(entry.PublishedBy, ", ")
	}

	return Article{
		ID:     entry.ID,
		URL:    entry.URL,
		Title:  entry.Title,
		Author: author,
	}
}
