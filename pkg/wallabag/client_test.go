package wallabag

import (
	"strings"
	"testing"
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
