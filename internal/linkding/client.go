package linkding

import (
	"fmt"

	linkdinglib "github.com/piero-vic/go-linkding"
)

type Client struct {
	client *linkdinglib.Client
}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Configure(settings *LinkdingSettings) {
	c.client = linkdinglib.NewClient(*settings.LinkdingInstanceURL, *settings.LinkdingAPIKey)
}

func (c *Client) Ping() error {
	if c.client == nil {
		return fmt.Errorf("client not configured, please call Configure() first")
	}

	_, err := c.client.ListTags(linkdinglib.ListTagsParams{Limit: 1})

	if err != nil {
		return fmt.Errorf("error pinging server: %w", err)
	}

	return nil
}

func (c *Client) GetUnread() ([]linkdinglib.Bookmark, error) {
	if c.client == nil {
		return nil, fmt.Errorf("client not configured, please call Configure() first")
	}

	resp, err := c.client.ListBookmarks(linkdinglib.ListBookmarksParams{Unread: true})

	if err != nil {
		return nil, fmt.Errorf("error fetching bookmarks: %w", err)
	}

	return resp.Results, nil
}

func (c *Client) GetBookmark(id int) (*linkdinglib.Bookmark, error) {
	if c.client == nil {
		return nil, fmt.Errorf("client not configured, please call Configure() first")
	}

	resp, err := c.client.GetBookmark(id)

	if err != nil {
		return nil, fmt.Errorf("error fetching bookmark %d: %w", id, err)
	}

	return resp, nil
}
