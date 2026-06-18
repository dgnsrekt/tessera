package matrix

import (
	"os"
	"testing"
)

// TestLiveStatus performs a read-only status query against a real matrix.
// It is skipped unless TESSERA_LIVE=1; point it at a device with
// TESSERA_HOST / TESSERA_PORT (defaults 10.10.0.1:5000). It never sends a
// switch/preset/buzzer command, so it cannot change live HDMI routing.
func TestLiveStatus(t *testing.T) {
	if os.Getenv("TESSERA_LIVE") != "1" {
		t.Skip("set TESSERA_LIVE=1 to run the live read-only status check")
	}
	host := os.Getenv("TESSERA_HOST")
	if host == "" {
		host = "10.10.0.1"
	}
	c := New(host, 5000)
	defer c.Close()

	routes, err := c.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !c.Connected() {
		t.Fatal("expected connected after a successful Status")
	}
	if len(routes) == 0 {
		t.Fatalf("expected routes, got empty (reply parse failed?)")
	}
	t.Logf("live routes: %v", routes)
}
