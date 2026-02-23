package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/altinukshini/gha-tui/internal/api"
	"github.com/altinukshini/gha-tui/internal/cache"
	"github.com/altinukshini/gha-tui/internal/config"
	"github.com/altinukshini/gha-tui/internal/tui"
)

var version = "dev"

func init() {
	if version != "dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
}

func main() {
	repo := flag.String("R", "", "Repository in owner/repo format (required)")
	cacheSizeMB := flag.Int("cache-size", 500, "Max log cache size in MB")
	cacheTTL := flag.Duration("cache-ttl", 24*time.Hour, "Log cache TTL")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("gha-tui", version)
		os.Exit(0)
	}

	if *repo == "" {
		fmt.Fprintln(os.Stderr, "Error: -R owner/repo is required")
		flag.Usage()
		os.Exit(1)
	}

	parts := strings.SplitN(*repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		fmt.Fprintln(os.Stderr, "Error: repo must be in owner/repo format")
		os.Exit(1)
	}

	cfg := config.Config{Owner: parts[0], Repo: parts[1]}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	client, err := api.NewClient(cfg.Owner, cfg.Repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Auth error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Make sure you are authenticated with: gh auth login")
		os.Exit(1)
	}

	cacheDir := filepath.Join(os.TempDir(), "gha-tui", "logs")
	logCache, err := cache.NewLogCache(cacheDir, *cacheSizeMB, *cacheTTL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cache error: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewApp(cfg, client, logCache)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
