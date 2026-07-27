package opds

import (
	"buckleberry/internal/epub"
	"buckleberry/internal/settings"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Strubbl/wallabago/v9"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gorilla/feeds"
	"github.com/piero-vic/go-linkding"
)

const ACQ_TYPE string = "application/atom+xml;profile=opds-catalog;kind=acquisition"
const NAV_TYPE string = "application/atom+xml;profile=opds-catalog;kind=navigation"

type settingsReader interface {
	Get() (*settings.Settings, error)
}

type wallabagClient interface {
	GetEntries() (*wallabago.Entries, error)
	ExportEntry(id int, format string) ([]byte, error)
	GetEntry(id int) (*wallabago.Item, error)
}

type linkdingClient interface {
	GetUnread() ([]linkding.Bookmark, error)
	GetBookmark(id int) (*linkding.Bookmark, error)
}

type epubBuilder interface {
	Build(article *epub.ReadableArticle, writer io.Writer) error
}

type articleFetcher interface {
	FetchFromContent(title, author, content string, tempPath string) (*epub.ReadableArticle, error)
	FetchFromURL(articleURL string, tempPath string) (*epub.ReadableArticle, error)
}

type Handler struct {
	settingsRepo   settingsReader
	wallabagClient wallabagClient
	linkdingClient linkdingClient
	fetcher        articleFetcher
	builder        epubBuilder
	baseUrl        string
}

func NewHandler(settingsRepository settingsReader, wallabagClient wallabagClient, linkdingClient linkdingClient, fetcher articleFetcher, builder epubBuilder, baseUrl string) *Handler {
	return &Handler{
		settingsRepo:   settingsRepository,
		wallabagClient: wallabagClient,
		linkdingClient: linkdingClient,
		fetcher:        fetcher,
		builder:        builder,
		baseUrl:        baseUrl,
	}
}

func (h *Handler) GetNavigationFeeds(c fiber.Ctx) error {
	settings, err := h.settingsRepo.Get()

	if err != nil {
		log.Error("navigation feeds: failed to fetch settings: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to generate feed")
	}

	feedUrl := fmt.Sprintf("%s/opds", h.baseUrl)
	updated := time.Now()

	feed := &feeds.Feed{
		Id:      feedUrl,
		Title:   "Buckleberry Articles",
		Items:   []*feeds.Item{},
		Updated: updated,
		Link:    &feeds.Link{Href: feedUrl, Rel: "self", Type: NAV_TYPE},
	}

	if settings.UseWallabag {
		unreadFeedUrl := fmt.Sprintf("%s/opds/wallabag", h.baseUrl)

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   "Wallabag",
			Id:      unreadFeedUrl,
			Updated: updated,
			Link:    &feeds.Link{Href: unreadFeedUrl, Type: ACQ_TYPE},
			Content: "Unread articles from Wallabag, sorted oldest to newest",
		})
	}

	if settings.UseLinkding {
		unreadFeedUrl := fmt.Sprintf("%s/opds/linkding", h.baseUrl)

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   "Linkding",
			Id:      unreadFeedUrl,
			Updated: updated,
			Link:    &feeds.Link{Href: unreadFeedUrl, Type: ACQ_TYPE},
			Content: "Unread articles from Linkding, sorted oldest to newest",
		})
	}

	c.Type("atom", "utf-8")

	atomFeed, err := feed.ToAtom()

	if err != nil {
		log.Error("generate navigation feed: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to generate feed")
	}

	return c.SendString(atomFeed)
}

func (h *Handler) GetUnreadWallabagFeed(c fiber.Ctx) error {
	feedUrl := fmt.Sprintf("%s/opds/wallabag", h.baseUrl)

	feed := &feeds.Feed{
		Title:   "Unread Wallabag Articles",
		Id:      feedUrl,
		Updated: time.Now(),
		Link:    &feeds.Link{Href: feedUrl, Rel: "self", Type: ACQ_TYPE},
	}

	entries, err := h.wallabagClient.GetEntries()

	if err != nil {
		log.Error("fetch Wallabag entries: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to fetch Wallabag articles")
	}

	var feedItems []*feeds.Item

	for _, entry := range entries.Embedded.Items {
		var author string
		authorCount := len(entry.PublishedBy)

		if authorCount == 0 {
			author = entry.DomainName
		} else {
			var authorBuilder strings.Builder

			for i, author := range entry.PublishedBy {
				if i == authorCount-1 {
					authorBuilder.WriteString(author)
				} else {
					fmt.Fprintf(&authorBuilder, "%s, ", author)
				}
			}

			author = authorBuilder.String()
		}

		entryUrl := fmt.Sprintf("%s/opds/wallabag/%d", h.baseUrl, entry.ID)

		var entryUpdated time.Time

		if entry.UpdatedAt != nil {
			entryUpdated = entry.UpdatedAt.Time
		} else if entry.CreatedAt != nil {
			entryUpdated = entry.CreatedAt.Time
		} else {
			entryUpdated = time.Now()
		}

		feedItems = append(feedItems, &feeds.Item{
			Id:      entryUrl,
			Title:   entry.Title,
			Link:    &feeds.Link{Href: entryUrl, Type: "application/epub+zip", Rel: "http://opds-spec.org/acquisition"},
			Author:  &feeds.Author{Name: author},
			Updated: entryUpdated,
		})
	}

	feed.Items = feedItems

	atomFeed, err := feed.ToAtom()

	if err != nil {
		log.Error("generate unread feed: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to generate feed")
	}

	c.Type("atom", "utf-8")

	return c.SendString(atomFeed)
}

func (h *Handler) GetWallabagDownload(c fiber.Ctx) error {
	settings, err := h.settingsRepo.Get()

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Couldn't fetch settings")
	}

	idParam := c.Params("id")

	if idParam == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing article ID")
	}

	articleId, err := strconv.Atoi(idParam)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid article ID")
	}

	log.Debug("Attempting to fetch ePUB for article with ID ", articleId)

	if settings.UseInternalEpubBuilder {
		entry, err := h.wallabagClient.GetEntry(articleId)

		if err != nil {
			log.Errorf("Failed to fetch article: %w", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Couldn't fetch wallabag article")
		}

		tempDir, err := os.MkdirTemp("", "epubbuilder")

		if err != nil {
			log.Errorf("Failed to create temp dir: %w", err)
			return fmt.Errorf("creating temp dir: %w", err)
		}

		defer os.RemoveAll(tempDir)

		author := strings.Join(entry.PublishedBy, ", ")

		article, err := h.fetcher.FetchFromContent(entry.Title, author, entry.Content, tempDir)

		if err != nil {
			log.Errorf("Failed to create readable article: %w", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Couldn't fetch wallabag article")
		}

		buffer := bytes.NewBuffer([]byte{})

		err = h.builder.Build(article, buffer)

		if err != nil {
			log.Errorf("Failed to create EPUB: %w", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Couldn't write wallabag epub bytes")
		}

		return c.Type("epub").Send(buffer.Bytes())
	} else {
		epubBytes, err := h.wallabagClient.ExportEntry(articleId, "epub")

		if err != nil {
			log.Errorf("export article %d as ePUB: %v", articleId, err)
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to download wallabag article")
		}

		return c.Type("epub").Send(epubBytes)
	}
}

func (h *Handler) GetUnreadLinkdingFeed(c fiber.Ctx) error {
	feedUrl := fmt.Sprintf("%s/opds/linkding", h.baseUrl)

	feed := &feeds.Feed{
		Title:   "Unread Linkding Articles",
		Id:      feedUrl,
		Updated: time.Now(),
		Link:    &feeds.Link{Href: feedUrl, Rel: "self", Type: ACQ_TYPE},
	}

	bookmarks, err := h.linkdingClient.GetUnread()

	if err != nil {
		log.Error("fetch Linkding entries: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to fetch Linkding articles")
	}

	var feedItems []*feeds.Item

	for _, bookmark := range bookmarks {
		entryUrl := fmt.Sprintf("%s/opds/linkding/%d", h.baseUrl, bookmark.ID)

		feedItems = append(feedItems, &feeds.Item{
			Id:      entryUrl,
			Title:   bookmark.Title,
			Link:    &feeds.Link{Href: entryUrl, Type: "application/epub+zip", Rel: "http://opds-spec.org/acquisition"},
			Author:  &feeds.Author{Name: bookmark.WebsiteTitle},
			Updated: bookmark.DateAdded,
		})
	}

	feed.Items = feedItems

	atomFeed, err := feed.ToAtom()

	if err != nil {
		log.Error("generate linkding unread feed: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to generate linkding unread feed")
	}

	c.Type("atom", "utf-8")

	return c.SendString(atomFeed)
}

func (h *Handler) GetLinkdingDownload(c fiber.Ctx) error {
	idParam := c.Params("id")

	if idParam == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing article ID")
	}

	articleId, err := strconv.Atoi(idParam)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid article ID")
	}

	log.Debug("Attempting to fetch ePUB for article with ID ", articleId)

	bookmark, err := h.linkdingClient.GetBookmark(articleId)

	if err != nil {
		log.Errorf("Failed to fetch linkding article: %w", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Couldn't fetch linkding article")
	}

	tempDir, err := os.MkdirTemp("", "epubbuilder")

	if err != nil {
		log.Errorf("Failed to create temp dir: %w", err)
		return fmt.Errorf("creating temp dir: %w", err)
	}

	defer os.RemoveAll(tempDir)

	article, err := h.fetcher.FetchFromURL(bookmark.URL, tempDir)

	if err != nil {
		log.Errorf("Failed to create readable article: %w", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Couldn't fetch linkding article")
	}

	buffer := bytes.NewBuffer([]byte{})

	err = h.builder.Build(article, buffer)

	if err != nil {
		log.Errorf("Failed to create EPUB: %w", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Couldn't write linkding epub bytes")
	}

	return c.Type("epub").Send(buffer.Bytes())
}
