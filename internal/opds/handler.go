package opds

import (
	"buckleberry/internal/adapter"
	"buckleberry/internal/settings"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gorilla/feeds"
)

const ACQ_TYPE string = "application/atom+xml;profile=opds-catalog;kind=acquisition"
const NAV_TYPE string = "application/atom+xml;profile=opds-catalog;kind=navigation"
const EPUB_TYPE string = "application/epub+zip"
const OPDS_ACQ_REL string = "http://opds-spec.org/acquisition"

type settingsReader interface {
	Get() (*settings.Settings, error)
}

type Handler struct {
	settingsRepo settingsReader
	baseUrl      string
}

func NewHandler(settingsRepository settingsReader, baseUrl string) *Handler {
	return &Handler{
		settingsRepo: settingsRepository,
		baseUrl:      baseUrl,
	}
}

func (h *Handler) GetTopLevelNavigationFeeds(c fiber.Ctx) error {
	adapters := c.Locals("adapters").(map[string]adapter.Adapter)

	feedUrl := fmt.Sprintf("%s/opds", h.baseUrl)
	updated := time.Now()

	feed := &feeds.Feed{
		Id:      feedUrl,
		Title:   "Buckleberry Articles",
		Items:   []*feeds.Item{},
		Updated: updated,
		Link:    &feeds.Link{Href: feedUrl, Rel: "self", Type: NAV_TYPE},
	}

	for name := range adapters {
		unreadFeedURL := fmt.Sprintf("%s/opds/%s", h.baseUrl, name)
		feed.Items = append(feed.Items, &feeds.Item{
			Title:   name,
			Id:      unreadFeedURL,
			Updated: updated,
			Content: fmt.Sprintf("articles from %s", name),
			Link:    &feeds.Link{Href: unreadFeedURL, Type: NAV_TYPE},
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

func (h *Handler) GetNavigationFeedsForAdapter(c fiber.Ctx) error {
	adapterParam := c.Params("adapter")
	adapters := c.Locals("adapters").(map[string]adapter.Adapter)

	if _, ok := adapters[adapterParam]; !ok {
		return c.Status(fiber.StatusBadRequest).SendString("invalid adapter")
	}

	selectedAdapter := adapters[adapterParam]
	adapterName := selectedAdapter.Name()

	feedUrl := fmt.Sprintf("%s/opds/%s", h.baseUrl, adapterName)

	feed := &feeds.Feed{
		Title:   fmt.Sprintf("%s articles", adapterName),
		Id:      feedUrl,
		Updated: time.Now(),
		Link:    &feeds.Link{Href: feedUrl, Rel: "self", Type: NAV_TYPE},
	}

	adapterFeeds, err := selectedAdapter.Feeds()

	if err != nil {
		log.Error("fetching feed list from adapter %s: %v", adapterName, err)
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("failed to fetch feed names for %s", adapterName))
	}

	for _, adapterFeed := range adapterFeeds {
		adapterFeedUrl := fmt.Sprintf("%s/opds/%s/%s", h.baseUrl, adapterName, adapterFeed)

		feed.Items = append(feed.Items, &feeds.Item{
			Id:      adapterFeedUrl,
			Title:   adapterFeed,
			Content: fmt.Sprintf("%s articles from %s", adapterFeed, adapterName),
			Updated: time.Now(),
			Link:    &feeds.Link{Href: adapterFeedUrl, Type: ACQ_TYPE},
		})
	}

	c.Type("atom", "utf-8")

	atomFeed, err := feed.ToAtom()

	if err != nil {
		log.Error("generating navigation feeds for adapter %s: %v", adapterName, err)
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("failed to generate navigation feeds for %s", adapterName))
	}

	return c.SendString(atomFeed)
}

func (h *Handler) GetAcquisitionFeedForAdapter(c fiber.Ctx) error {
	adapterParam := c.Params("adapter")
	feedParam := c.Params("feed")
	adapters := c.Locals("adapters").(map[string]adapter.Adapter)

	if _, ok := adapters[adapterParam]; !ok {
		return c.Status(fiber.StatusBadRequest).SendString("invalid adapter")
	}

	selectedAdapter := adapters[adapterParam]
	adapterName := selectedAdapter.Name()
	adapterFeeds, err := selectedAdapter.Feeds()

	if err != nil {
		log.Error("fetching feed list from adapter %s: %v", adapterName, err)
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("failed to fetch feed names for %s", adapterName))
	}

	if !slices.Contains(adapterFeeds, feedParam) {
		return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("invalid feed %s for %s", feedParam, adapterName))
	}

	feedUrl := fmt.Sprintf("%s/opds/%s/%s", h.baseUrl, adapterName, feedParam)

	feed := &feeds.Feed{
		Title:   fmt.Sprintf("%s %s Articles", feedParam, adapterName),
		Id:      feedUrl,
		Updated: time.Now(),
		Link:    &feeds.Link{Href: feedUrl, Rel: "self", Type: ACQ_TYPE},
	}

	articles, err := selectedAdapter.Articles(feedParam)

	if err != nil {
		log.Error("failed to fetch articles for %s from adapter %s: %v", feedParam, adapterName, err)
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("failed to fetch %s articles from %s", feedParam, adapterName))
	}

	for _, article := range articles {
		articleUrl := fmt.Sprintf("%s/opds/%s/%s/%d", h.baseUrl, adapterName, feedParam, article.ID)

		feed.Items = append(feed.Items, &feeds.Item{
			Id:      articleUrl,
			Title:   article.Title,
			Author:  &feeds.Author{Name: article.Author},
			Updated: time.Now(),
			Link:    &feeds.Link{Href: articleUrl, Type: EPUB_TYPE, Rel: OPDS_ACQ_REL},
		})
	}

	c.Type("atom", "utf-8")

	atomFeed, err := feed.ToAtom()

	if err != nil {
		log.Error("generating acquisition feed %s for adapter %s: %v", feedParam, adapterName, err)
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("failed to generate acquisition feed %s for %s", feedParam, adapterName))
	}

	return c.SendString(atomFeed)
}

func (h *Handler) DownloadArticle(c fiber.Ctx) error {
	adapterParam := c.Params("adapter")
	adapters := c.Locals("adapters").(map[string]adapter.Adapter)

	if _, ok := adapters[adapterParam]; !ok {
		return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("invalid adapter: %s", adapterParam))
	}

	selectedAdapter := adapters[adapterParam]

	idParam := c.Params("id")

	if idParam == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing article ID")
	}

	articleId, err := strconv.Atoi(idParam)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid article ID")
	}

	epubBytes, err := selectedAdapter.Download(articleId)

	if err != nil {
		log.Errorf("export %s article %d as ePUB: %v", adapterParam, articleId, err)
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to download %s article %d", adapterParam, articleId))
	}

	return c.Type("epub").Send(epubBytes)
}
