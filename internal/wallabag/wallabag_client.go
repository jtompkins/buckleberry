package wallabag

import (
	"fmt"

	"buckleberry/internal/settings"

	"github.com/Strubbl/wallabago/v9"
)

type Client struct{}

func NewClient() Client {
	return Client{}
}

// Configure applies the stored Wallabag credentials to the wallabago package's
// process-global config. Call it once at startup and again whenever the
// settings change.
func (Client) Configure(s *settings.Settings) {
	wallabago.SetConfig(wallabago.NewWallabagConfig(
		s.WallabagInstanceURL,
		s.WallabagClientID,
		s.WallabagClientSecret,
		s.WallabagUsername,
		s.WallabagPassword,
	))
}

func (Client) GetEntries() (*wallabago.Entries, error) {
	entries, err := wallabago.GetEntries(wallabago.APICall, 0, -1, "", "", -1, -1, "", -1, -1, "metadata", "")

	if err != nil {
		return nil, fmt.Errorf("get Wallabag entries: %w", err)
	}

	return &entries, nil
}

// Ping checks connectivity to the configured Wallabag instance by making a
// lightweight authenticated API call.
func (Client) Ping() error {
	if _, err := wallabago.GetTags(wallabago.APICall); err != nil {
		return fmt.Errorf("ping Wallabag: %w", err)
	}

	return nil
}

func (Client) GetEntry(id int) (*wallabago.Item, error) {
	entry, err := wallabago.GetEntry(wallabago.APICall, id)

	if err != nil {
		return nil, fmt.Errorf("get Wallabag entry %d: %w", id, err)
	}

	return &entry, nil
}

func (Client) ExportEntry(id int, format string) ([]byte, error) {
	epubBytes, err := wallabago.ExportEntry(wallabago.APICall, id, format)

	if err != nil {
		return nil, fmt.Errorf("export Wallabag entry %d as %s: %w", id, format, err)
	}

	return epubBytes, nil
}
