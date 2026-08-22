package wallabag

import (
	"fmt"

	"github.com/Strubbl/wallabago/v9"
)

type WallabagConfig struct {
	WallabagInstanceURL  string
	WallabagClientID     string
	WallabagClientSecret string
	WallabagUsername     string
	WallabagPassword     string
}

type Client struct {
	isConfigured bool
}

func NewClient() *Client {
	return &Client{
		isConfigured: false,
	}
}

func (c *Client) Configure(wallabagConfig WallabagConfig) {
	wallabago.SetConfig(wallabago.NewWallabagConfig(
		wallabagConfig.WallabagInstanceURL,
		wallabagConfig.WallabagClientID,
		wallabagConfig.WallabagClientSecret,
		wallabagConfig.WallabagUsername,
		wallabagConfig.WallabagPassword,
	))

	c.isConfigured = true
}

func (c *Client) GetEntries() (*wallabago.Entries, error) {
	if !c.isConfigured {
		return nil, fmt.Errorf("client not configured, call Configure() first")
	}

	entries, err := wallabago.GetEntries(wallabago.APICall, 0, -1, "", "", -1, -1, "", -1, -1, "metadata", "")

	if err != nil {
		return nil, fmt.Errorf("get Wallabag entries: %w", err)
	}

	return &entries, nil
}

func (c *Client) Ping() error {
	if !c.isConfigured {
		return fmt.Errorf("client not configured, please call Configure() first")
	}

	if _, err := wallabago.GetTags(wallabago.APICall); err != nil {
		return fmt.Errorf("ping Wallabag: %w", err)
	}

	return nil
}

func (c *Client) GetEntry(id int) (*wallabago.Item, error) {
	if !c.isConfigured {
		return nil, fmt.Errorf("client not configured, please call Configure() first")
	}

	entry, err := wallabago.GetEntry(wallabago.APICall, id)

	if err != nil {
		return nil, fmt.Errorf("get Wallabag entry %d: %w", id, err)
	}

	return &entry, nil
}

func (c *Client) ExportEntry(id int, format string) ([]byte, error) {
	if !c.isConfigured {
		return nil, fmt.Errorf("client not configured, please call Configure() first")
	}

	epubBytes, err := wallabago.ExportEntry(wallabago.APICall, id, format)

	if err != nil {
		return nil, fmt.Errorf("export Wallabag entry %d as %s: %w", id, format, err)
	}

	return epubBytes, nil
}
