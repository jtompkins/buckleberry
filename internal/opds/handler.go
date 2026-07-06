package opds

import (
	"buckleberry/internal/settings"
	"fmt"
	"strconv"
	"strings"

	"github.com/Strubbl/wallabago/v9"
	"github.com/gofiber/fiber/v3"
	"github.com/gorilla/feeds"
)

type Handler struct {
	settingsRepo *settings.Repository
}

func NewHandler(settingsRepository *settings.Repository) *Handler {
	return &Handler{
		settingsRepo: settingsRepository,
	}
}

func (h *Handler) GetNavigationFeeds(c fiber.Ctx) error {
	feed := &feeds.Feed{
		Title: "Wallabag Articles",
	}

	feed.Items = []*feeds.Item{
		{
			Title:   "Unread articles",
			Link:    &feeds.Link{Href: "/opds/unread", Type: "application/atom+xml;profile=opds-catalog"},
			Content: "Unread articles from Wallabag, sorted oldest to newest",
		},
	}

	c.Type("application/atom+xml", "utf-8")

	atomFeed, err := feed.ToAtom()

	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendString(atomFeed)
}

func (h *Handler) GetUnreadFeed(c fiber.Ctx) error {
	settings, err := h.settingsRepo.Get()

	if err != nil {
		return c.Status(500).SendString(fmt.Sprintf("Failed to fetch settings: %s", err.Error()))
	}

	feed := &feeds.Feed{
		Title: "Unread Wallabag Articles",
	}

	// TODO: move this logic into the main.go file so that it doesn't have to be repeated every time?
	// alternative: move it into the constructor function
	wallabago.SetConfig(wallabago.NewWallabagConfig(
		settings.WallabagInstanceURL,
		settings.WallabagClientID,
		settings.WallabagClientSecret,
		settings.WallabagUsername,
		settings.WallabagPassword),
	)

	entries, err := wallabago.GetEntries(wallabago.APICall, 0, -1, "", "", -1, -1, "", -1, -1, "metadata", "")

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
			Link:   &feeds.Link{Href: entry.URL, Type: "application/epub+zip", Rel: "http://opds-spec.org/acquisition"},
			Author: &feeds.Author{Name: author},
		})
	}

	feed.Items = feedItems

	atomFeed, err := feed.ToAtom()

	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendString(atomFeed)
}

func (h *Handler) GetDownload(c fiber.Ctx) error {
	// TODO: Implement this method
	// The Wallabako library includes a method call to get the bytes for the epub;
	// we'll need to pull that down and then flow the bytes into the Fiber return
	// somehow. I'm sure there's a way to do that in Fiber, just need to find it.
	return nil
}
