package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tapcraft-io/purr/internal/config"
	"github.com/tapcraft-io/purr/internal/history"
	"github.com/tapcraft-io/purr/internal/k8s"
	"github.com/tapcraft-io/purr/internal/tui"
)

func main() {
	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize Kubernetes client
	client, err := k8s.NewClient(cfg.KubeconfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to Kubernetes: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure kubectl is configured and you have access to a cluster.\n")
		os.Exit(1)
	}

	// Get current context
	currentContext, err := k8s.GetCurrentContext(cfg.KubeconfigPath)
	if err != nil {
		currentContext = "unknown"
	}

	// Initialize resource cache
	cache := k8s.NewResourceCache(client.Clientset)

	// Start cache refresh in background
	go func() {
		if err := cache.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Cache initialization failed: %v\n", err)
		}
	}()

	// Initialize history
	hist, err := history.NewHistory(cfg.HistorySize, cfg.HistoryFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not load history: %v\n", err)
		// Continue without history
	}

	// Create and run the TUI
	model := tui.NewModel(cache, hist, currentContext, cfg.KubeconfigPath)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Run the program
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}

	// Save history before exiting
	if m, ok := finalModel.(tui.Model); ok {
		if hist != nil {
			_ = hist.Save()
		}

		// Stop cache
		if cache != nil {
			cache.Stop()
		}

		// Check if there was an error
		if !m.IsReady() {
			fmt.Println("Warning: Cache was not fully initialized")
		}
	}
}
