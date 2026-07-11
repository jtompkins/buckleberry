package opds

import (
	"buckleberry/internal/settings"
	"fmt"
	"strconv"
	"strings"

	"github.com/Strubbl/wallabago/v9"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gorilla/feeds"
)

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
	feed := &feeds.Feed{
		Title: "Wallabag Articles",
		Items: []*feeds.Item{
			{
				Title:   "Unread articles",
				Link:    &feeds.Link{Href: fmt.Sprintf("%s/opds/unread", h.baseUrl), Type: "application/atom+xml;profile=opds-catalog"},
				Content: "Unread articles from Wallabag, sorted oldest to newest",
			},
		},
	}

	c.Type("atom", "utf-8")

	atomFeed, err := feed.ToAtom()

	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendString(atomFeed)
}

func (h *Handler) GetUnreadFeed(c fiber.Ctx) error {
	feed := &feeds.Feed{
		Title: "Unread Wallabag Articles",
	}

	entries, err := h.client.GetEntries()

	if err != nil {
		return c.Status(500).SendString(fmt.Sprintf("Failed to fetch Wallabag articles: %s", err.Error()))
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

		feedItems = append(feedItems, &feeds.Item{
			Id:     strconv.Itoa(entry.ID),
			Title:  entry.Title,
			Link:   &feeds.Link{Href: fmt.Sprintf("%s/opds/download/%d", h.baseUrl, entry.ID), Type: "application/epub+zip", Rel: "http://opds-spec.org/acquisition"},
			Author: &feeds.Author{Name: author},
		})
	}

	feed.Items = feedItems

	atomFeed, err := feed.ToAtom()

	if err != nil {
		return c.Status(500).SendString(err.Error())
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
		log.Debug("Error when fetching ePUB: ", err.Error())
		return c.Status(500).SendString(err.Error())
	}

	log.Debug("Fetched article, length: ", len(epubBytes))

	return c.Type("epub").Send(epubBytes)
}
