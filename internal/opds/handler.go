package opds

import (
	"buckleberry/internal/settings"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Strubbl/wallabago/v9"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gorilla/feeds"
)

const ACQUISISTION_TYPE string = "application/atom+xml;profile=opds-catalog;kind=acquisition"
const NAVIGATION_TYPE string = "application/atom+xml;profile=opds-catalog;kind=navigation"

type settingsReader interface {
	Get() (*settings.Settings, error)
}

type wallabagClient interface {
	GetEntries() (*wallabago.Entries, error)
	ExportEntry(id int, format string) ([]byte, error)
}

type Handler struct {
	settingsRepo settingsReader
	client       wallabagClient
	baseUrl      string
}

func NewHandler(settingsRepository settingsReader, client wallabagClient, baseUrl string) *Handler {
	return &Handler{
		settingsRepo: settingsRepository,
		client:       client,
		baseUrl:      baseUrl,
	}
}

func (h *Handler) GetNavigationFeeds(c fiber.Ctx) error {
	feedUrl := fmt.Sprintf("%s/opds", h.baseUrl)
	unreadFeedUrl := fmt.Sprintf("%s/opds/unread", h.baseUrl)
	updated := time.Now()

	feed := &feeds.Feed{
		Id:    feedUrl,
		Title: "Wallabag Articles",
		Items: []*feeds.Item{
			{
				Title:   "Unread articles",
				Id:      unreadFeedUrl,
				Updated: updated,
				Link:    &feeds.Link{Href: unreadFeedUrl, Type: ACQUISISTION_TYPE},
				Content: "Unread articles from Wallabag, sorted oldest to newest",
			},
		},
		Updated: updated,
		Link:    &feeds.Link{Href: feedUrl, Rel: "self", Type: NAVIGATION_TYPE},
	}

	c.Type("atom", "utf-8")

	atomFeed, err := feed.ToAtom()

	if err != nil {
		log.Error("generate navigation feed: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to generate feed")
	}

	return c.SendString(atomFeed)
}

func (h *Handler) GetUnreadFeed(c fiber.Ctx) error {
	feedUrl := fmt.Sprintf("%s/opds/unread", h.baseUrl)

	feed := &feeds.Feed{
		Title:   "Unread Wallabag Articles",
		Id:      feedUrl,
		Updated: time.Now(),
		Link:    &feeds.Link{Href: feedUrl, Rel: "self", Type: ACQUISISTION_TYPE},
	}

	entries, err := h.client.GetEntries()

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

		entryUrl := fmt.Sprintf("%s/opds/download/%d", h.baseUrl, entry.ID)

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

func (h *Handler) GetDownload(c fiber.Ctx) error {
	idParam := c.Params("id")

	if idParam == "" {
		return c.Status(400).SendString("Missing article ID")
	}

	articleId, err := strconv.Atoi(idParam)

	if err != nil {
		return c.Status(400).SendString("Invalid article ID")
	}

	log.Debug("Attempting to fetch ePUB for article with ID ", articleId)

	epubBytes, err := h.client.ExportEntry(articleId, "epub")

	if err != nil {
		log.Errorf("export article %d as ePUB: %v", articleId, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to download article")
	}

	log.Debug("Fetched article, length: ", len(epubBytes))

	return c.Type("epub").Send(epubBytes)
}
