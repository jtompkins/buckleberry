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

func (Client) ExportEntry(id int, format string) ([]byte, error) {
	epubBytes, err := wallabago.ExportEntry(wallabago.APICall, id, format)

	if err != nil {
		return nil, fmt.Errorf("export Wallabag entry %d as %s: %w", id, format, err)
	}

	return epubBytes, nil
}
