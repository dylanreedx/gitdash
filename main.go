package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dylan/gitdash/config"
	"github.com/dylan/gitdash/tui"
)

func main() {
	configPath := flag.String("config", "", "path to config file (default: ~/.config/gitdash/config.toml)")
	flag.Parse()

	path := *configPath
	explicit := path != ""
	if !explicit {
		path = config.DefaultConfigPath()
	}

	cfg, err := config.Load(path)
	if err != nil {
		// If using default path and file doesn't exist, use empty config
		if !explicit && errors.Is(err, os.ErrNotExist) {
			cfg = config.Config{}
		} else {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
	}

	app := tui.NewApp(cfg)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
