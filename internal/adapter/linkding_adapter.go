package adapter

import (
	"buckleberry/pkg/epub"
	"bytes"
	"fmt"
	"io"

	linkdinglib "github.com/piero-vic/go-linkding"
)

type linkdingClient interface {
	GetUnread() ([]linkdinglib.Bookmark, error)
	GetBookmark(id int) (*linkdinglib.Bookmark, error)
	Ping() error
}

type articleFetcher interface {
	FetchFromContent(title, author, content string) (*epub.ReadableArticle, error)
	FetchFromURL(articleURL string) (*epub.ReadableArticle, error)
}

type epubBuilder interface {
	Build(article *epub.ReadableArticle, writer io.Writer) error
}

type LinkdingAdapter struct {
	client  linkdingClient
	fetcher articleFetcher
	builder epubBuilder
}

func NewLinkdingAdapter(client linkdingClient, fetcher articleFetcher, builder epubBuilder) *LinkdingAdapter {
	return &LinkdingAdapter{
		client:  client,
		fetcher: fetcher,
		builder: builder,
	}
}

func (la *LinkdingAdapter) Name() string {
	return "linkding"
}

func (la *LinkdingAdapter) Feeds() ([]string, error) {
	return []string{"unread"}, nil
}

func (la *LinkdingAdapter) Articles(feed string) ([]Article, error) {
	if feed != "unread" {
		return nil, fmt.Errorf("invalid feed")
	}

	bookmarks, err := la.client.GetUnread()

	if err != nil {
		return nil, fmt.Errorf("unable to fetch Linkding bookmarks: %v", err)
	}

	articles := []Article{}

	for _, bookmark := range bookmarks {
		articles = append(articles, bookmarkToArticle(bookmark))
	}

	return articles, nil
}

func (la *LinkdingAdapter) Download(id int) ([]byte, error) {
	bookmark, err := la.client.GetBookmark(id)

	if err != nil {
		return nil, fmt.Errorf("unable to fetch Linkding bookmark %d: %v", id, err)
	}

	readableArticle, err := la.fetcher.FetchFromURL(bookmark.URL)

	if err != nil {
		return nil, fmt.Errorf("unable to fetch article content %d: %v", id, err)
	}

	buffer := bytes.NewBuffer([]byte{})

	err = la.builder.Build(readableArticle, buffer)

	if err != nil {
		return nil, fmt.Errorf("unable to build epub %d: %v", id, err)
	}

	return buffer.Bytes(), nil
}

func (la *LinkdingAdapter) Ping() error {
	return la.client.Ping()
}

func bookmarkToArticle(bookmark linkdinglib.Bookmark) Article {
	return Article{
		ID:     bookmark.ID,
		URL:    bookmark.URL,
		Title:  bookmark.Title,
		Author: bookmark.WebsiteTitle,
	}
}
