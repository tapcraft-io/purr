package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tapcraft-io/purr/internal/config"
	"github.com/tapcraft-io/purr/internal/history"
	"github.com/tapcraft-io/purr/internal/k8s"
	"github.com/tapcraft-io/purr/internal/kubecomplete"
	"github.com/tapcraft-io/purr/internal/tui"
)

func main() {
	// Parse command-line flags
	demoMode := flag.Bool("demo", false, "Run in demo mode with mock Kubernetes data (no cluster required)")
	flag.Parse()

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

	var cache k8s.Cache
	var currentContext string

	if *demoMode {
		// Demo mode: use mock cache
		fmt.Println("Starting Purr in demo mode with mock data...")
		cache = k8s.NewMockResourceCache()
		currentContext = "demo-cluster"

		// Start mock cache (no-op for mock)
		go func() {
			if err := cache.Start(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Mock cache initialization failed: %v\n", err)
			}
		}()
	} else {
		// Production mode: connect to real cluster
		client, err := k8s.NewClient(cfg.KubeconfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to Kubernetes: %v\n", err)
			fmt.Fprintf(os.Stderr, "Make sure kubectl is configured and you have access to a cluster.\n")
			fmt.Fprintf(os.Stderr, "Or run with --demo flag to try demo mode without a cluster.\n")
			os.Exit(1)
		}

		// Get current context
		currentContext, err = k8s.GetCurrentContext(cfg.KubeconfigPath)
		if err != nil {
			currentContext = "unknown"
		}

		// Initialize resource cache
		cache = k8s.NewResourceCache(client.Clientset)

		// Start cache refresh in background
		go func() {
			if err := cache.Start(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Cache initialization failed: %v\n", err)
			}
		}()
	}

	// Initialize history
	hist, err := history.NewHistory(cfg.HistorySize, cfg.HistoryFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not load history: %v\n", err)
		// Continue without history
	}

	// Load kubectl command specifications
	root, err := kubecomplete.LoadRootSpecFromFile("kubectl_commands.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading kubectl commands spec: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure kubectl_commands.json exists in the current directory.\n")
		os.Exit(1)
	}

	// Create registry and completer
	registry := kubecomplete.NewRegistry(root)
	completer := kubecomplete.NewCompleter(registry, cache)

	// Create and run the TUI
	model := tui.NewModel(cache, hist, currentContext, cfg.KubeconfigPath, completer)

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
