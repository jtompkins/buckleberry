package wallabag

import (
	"strings"
	"testing"

	"github.com/Strubbl/wallabago/v9"
)

// Before onboarding completes, nothing has called Configure. Every call must
// report that cleanly instead of dereferencing a nil config.
func TestUnconfiguredClientReturnsErrors(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client) error
	}{
		{"Ping", func(c *Client) error { return c.Ping() }},
		{"GetEntries", func(c *Client) error { _, err := c.GetEntries(); return err }},
		{"GetEntry", func(c *Client) error { _, err := c.GetEntry(1); return err }},
		{"ExportEntry", func(c *Client) error { _, err := c.ExportEntry(1, "epub"); return err }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(NewClient())
			if err == nil {
				t.Fatalf("%s() on an unconfigured client = nil error, want failure", tc.name)
			}
			if !strings.Contains(err.Error(), "client not configured") {
				t.Errorf("error = %v, want it to say the client is not configured", err)
			}
		})
	}
}

func TestConfigureStoresSettings(t *testing.T) {
	client := NewClient()
	url := "https://wallabag.example.com"

	client.Configure(&WallabagSettings{WallabagInstanceURL: &url})

	if client.config == nil {
		t.Fatal("Configure() did not store the settings")
	}
	if client.config.WallabagInstanceURL == nil || *client.config.WallabagInstanceURL != url {
		t.Errorf("stored instance URL = %v, want %q", client.config.WallabagInstanceURL, url)
	}
}

// wallabago keeps its credentials in package-level state, which this client
// only writes on the first call after Configure. Saving new settings must
// therefore reset that latch, or the change wouldn't take effect until the
// process restarted.
func TestConfigureRepointsWallabago(t *testing.T) {
	settings := func(url string) *WallabagSettings {
		str := func(s string) *string { return &s }
		return &WallabagSettings{
			WallabagInstanceURL:  str(url),
			WallabagUsername:     str("user"),
			WallabagPassword:     str("pass"),
			WallabagClientID:     str("id"),
			WallabagClientSecret: str("secret"),
		}
	}

	client := NewClient()

	client.Configure(settings("https://first.example.com"))
	// Ping fails (nothing is listening), but pushes the config through first.
	_ = client.Ping()
	if got := wallabago.LibConfig.WallabagURL; got != "https://first.example.com" {
		t.Fatalf("LibConfig.WallabagURL = %q, want the first instance URL", got)
	}

	client.Configure(settings("https://second.example.com"))
	_ = client.Ping()
	if got := wallabago.LibConfig.WallabagURL; got != "https://second.example.com" {
		t.Errorf("LibConfig.WallabagURL = %q, want the reconfigured instance URL", got)
	}
}
