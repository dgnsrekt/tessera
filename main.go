// Command tessera is a terminal UI controller for the TESmart HDMI matrix switcher.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dgnsrekt/tessera/internal/config"
	"github.com/dgnsrekt/tessera/internal/matrix"
	"github.com/dgnsrekt/tessera/tui"
)

// version is the build version (overridable via -ldflags "-X main.version=...").
var version = "0.1.0"

func main() {
	host := flag.String("host", "", "override matrix host (default from config)")
	port := flag.Int("port", 0, "override matrix port (default from config)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("tessera", version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
	}
	if *host != "" {
		cfg.Host = *host
	}
	if *port != 0 {
		cfg.Port = *port
	}

	client := matrix.New(cfg.Host, cfg.Port)
	defer client.Close()

	p := tea.NewProgram(tui.New(cfg, client, version), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
