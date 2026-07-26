package wallabag

import (
	"fmt"

	"github.com/Strubbl/wallabago/v9"
)

type Client struct {
	config       *WallabagSettings
	isConfigured bool
}

func NewClient() *Client {
	return &Client{
		isConfigured: false,
	}
}

func (c *Client) Configure(wallabagConfig *WallabagSettings) {
	c.config = wallabagConfig
}

func (c *Client) configureWallabago() {
	wallabago.SetConfig(wallabago.NewWallabagConfig(
		*c.config.WallabagInstanceURL,
		*c.config.WallabagClientID,
		*c.config.WallabagClientSecret,
		*c.config.WallabagUsername,
		*c.config.WallabagPassword,
	))
}

func (c *Client) GetEntries() (*wallabago.Entries, error) {
	if !c.isConfigured {
		c.configureWallabago()
	}

	entries, err := wallabago.GetEntries(wallabago.APICall, 0, -1, "", "", -1, -1, "", -1, -1, "metadata", "")

	if err != nil {
		return nil, fmt.Errorf("get Wallabag entries: %w", err)
	}

	return &entries, nil
}

// Ping checks connectivity to the configured Wallabag instance by making a
// lightweight authenticated API call.
func (c *Client) Ping() error {
	if !c.isConfigured {
		c.configureWallabago()
	}

	if _, err := wallabago.GetTags(wallabago.APICall); err != nil {
		return fmt.Errorf("ping Wallabag: %w", err)
	}

	return nil
}

func (c *Client) GetEntry(id int) (*wallabago.Item, error) {
	if !c.isConfigured {
		c.configureWallabago()
	}

	entry, err := wallabago.GetEntry(wallabago.APICall, id)

	if err != nil {
		return nil, fmt.Errorf("get Wallabag entry %d: %w", id, err)
	}

	return &entry, nil
}

func (c *Client) ExportEntry(id int, format string) ([]byte, error) {
	if !c.isConfigured {
		c.configureWallabago()
	}

	epubBytes, err := wallabago.ExportEntry(wallabago.APICall, id, format)

	if err != nil {
		return nil, fmt.Errorf("export Wallabag entry %d as %s: %w", id, format, err)
	}

	return epubBytes, nil
}
