package wallabag

import (
	"fmt"

	"github.com/Strubbl/wallabago/v9"
)

type Client struct{}

func NewClient() Client {
	return Client{}
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

func (Client) ExportEntry(id int, format string) ([]byte, error) {
	epubBytes, err := wallabago.ExportEntry(wallabago.APICall, id, format)

	if err != nil {
		return nil, fmt.Errorf("export Wallabag entry %d as %s: %w", id, format, err)
	}

	return epubBytes, nil
}
