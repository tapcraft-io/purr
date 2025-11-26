# Purr 🐱

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](https://github.com/tapcraft-io/purr)
[![Coverage](https://img.shields.io/badge/coverage-90%25-green)](https://github.com/tapcraft-io/purr)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

A beautiful TUI (Text User Interface) wrapper for kubectl that maintains 100% feature parity while adding quality-of-life improvements through intelligent completion, resource caching, and elegant design.

```
┌─ Purr ─────────────────────── [context: production] ─┐
│                                                       │
│  > kubectl get pods█                                  │
│                                                       │
│  ┌─ Select Namespace ──────────────────────┐         │
│  │ Search: pro█                             │         │
│  ├──────────────────────────────────────────┤         │
│  │ ❯ production (last used)                 │         │
│  │   staging                                │         │
│  │   development                            │         │
│  │   default                                │         │
│  └──────────────────────────────────────────┘         │
│                                                       │
│  [Tab] autocomplete  [Ctrl+R] history  [Ctrl+C] quit │
└───────────────────────────────────────────────────────┘
```

## Features

✨ **100% kubectl Compatible** - Every kubectl command works identically
🚀 **Smart Completions** - Interactive pickers for namespaces, resources, and files
💾 **Resource Caching** - Background refresh every 30s for instant lookups
📜 **Command History** - Persistent history with fuzzy search (Ctrl+R)
🎨 **Beautiful UI** - Elegant design with Charm's Bubble Tea & Lipgloss
⚡ **Zero Friction** - Enhances kubectl without changing your workflow
🔒 **Safety First** - Confirmation dialogs for destructive operations
⚡ **Fast** - 90%+ test coverage, concurrent-safe caching

## Installation

### From Source

```bash
git clone https://github.com/tapcraft-io/purr.git
cd purr
go build -o purr cmd/purr/main.go
sudo mv purr /usr/local/bin/
```

### Using Go Install

```bash
go install github.com/tapcraft-io/purr/cmd/purr@latest
```

## Quick Start

Simply run `purr` in your terminal:

```bash
purr
```

Purr will use your existing kubectl configuration and context.

## Demo

### Main Interface
Type any kubectl command and enjoy enhanced autocomplete:

```
┌─ Purr ────────────────── [context: prod] ─────┐
│                                               │
│  > kubectl get pods -n production             │
│                                               │
│  ┌─ Output ──────────────────────────────┐   │
│  │ $ kubectl get pods -n production      │   │
│  │                                        │   │
│  │ NAME              READY   STATUS   AGE │   │
│  │ api-server-abc    1/1     Running  2d  │   │
│  │ worker-xyz        1/1     Running  1d  │   │
│  │ cache-123         1/1     Running  3h  │   │
│  └────────────────────────────────────────┘   │
│                                               │
│  ✓ Command succeeded                          │
│                                               │
│  [n] new  [r] re-run  [e] edit  [Ctrl+C] quit │
└───────────────────────────────────────────────┘
```

### Resource Picker
Press `Tab` after typing a resource type to browse available resources:

```
┌─ Purr ────────────────── [context: prod] ─────┐
│                                               │
│  > kubectl describe pod                       │
│                                               │
│  ┌─ Select pods ────────────────────────┐    │
│  │ Search: api█                         │    │
│  ├──────────────────────────────────────┤    │
│  │ ● api-server-abc                     │    │
│  │   Status: Running | Age: 2d | ...    │    │
│  │                                      │    │
│  │   api-server-def                     │    │
│  │   Status: Running | Age: 1d | ...    │    │
│  │                                      │    │
│  │   worker-api-001                     │    │
│  │   Status: Running | Age: 12h | ...   │    │
│  └──────────────────────────────────────┘    │
│                                               │
│  [↑↓] navigate  [Enter] select  [/] filter   │
└───────────────────────────────────────────────┘
```

### Command History (Ctrl+R)
Search through your command history with fuzzy matching:

```
┌─ Purr ────────────────── [context: prod] ─────┐
│                                               │
│  ┌─ Command History ────────────────────┐    │
│  │ Search: deploy█                      │    │
│  ├──────────────────────────────────────┤    │
│  │ ❯ kubectl get deployments -n prod   │    │
│  │   2024-01-15 14:30 | prod/default   │    │
│  │                                      │    │
│  │   kubectl rollout restart deploy... │    │
│  │   2024-01-15 12:15 | prod/default   │    │
│  │                                      │    │
│  │   kubectl apply -f deployment.yaml  │    │
│  │   2024-01-14 16:45 | prod/default   │    │
│  └──────────────────────────────────────┘    │
│                                               │
│  [Enter] run  [e] edit  [Esc] cancel         │
└───────────────────────────────────────────────┘
```

### Safety Confirmation
Destructive operations require confirmation:

```
┌─ Purr ────────────────── [context: prod] ─────┐
│                                               │
│  ⚠ Destructive Operation                      │
│                                               │
│  Command: kubectl delete deployment api-srv   │
│                                               │
│  This command may delete or modify resources. │
│  Are you sure you want to continue?          │
│                                               │
│  [y] yes  [n] no                              │
└───────────────────────────────────────────────┘
```

## Usage

### Basic Commands

Just type kubectl commands as you normally would:

```
> kubectl get pods
> kubectl get pods -n production
> kubectl describe pod my-pod
> kubectl logs my-pod -f
> kubectl apply -f deployment.yaml
```

### Smart Features

#### Namespace Completion

Type `-n ` or `--namespace ` and press `Tab` to see a list of all namespaces:

```
> kubectl get pods -n [Tab]
```

A picker will appear with all available namespaces. Use arrow keys to navigate and Enter to select.

#### Resource Completion

After specifying a resource type, press `Tab` to see available resources:

```
> kubectl get pods [Tab]
```

You'll see a list of all pods in the current/specified namespace with their status and age.

#### File Picker

Type `@` or use `-f ` flag to open a file picker:

```
> kubectl apply -f @
```

Navigate through your filesystem to select YAML/JSON files.

#### Command History

Press `Ctrl+R` to search through your command history:

- Navigate with arrow keys
- Press `Enter` to execute
- Press `e` to edit before executing
- Fuzzy search by typing

### Keybindings

#### Global
- `Ctrl+C` or `q` - Quit (from output view)
- `Ctrl+L` - Clear screen
- `Ctrl+R` - Open command history
- `Esc` - Cancel/Go back

#### Typing Mode
- `Tab` - Autocomplete / Show suggestions
- `Enter` - Execute command
- `Ctrl+U` - Clear line
- `Ctrl+W` - Delete word

#### Selection Mode (Pickers)
- `↑/↓` or `j/k` - Navigate
- `/` - Search/Filter
- `Enter` - Select
- `Esc` - Cancel

#### Output Mode
- `↑/↓` or `j/k` - Scroll
- `n` - New command
- `r` - Re-run last command
- `e` - Edit and re-run
- `q` or `Ctrl+C` - Return to typing

## Configuration

Purr stores its configuration in `~/.purr/`:

```yaml
# ~/.purr/config.yaml
preferences:
  default_namespace: default
  history_size: 1000
  cache_ttl: 30s
  confirm_destructive: true

ui:
  theme: dark
  show_help: true
  compact_mode: false
```

### History

Command history is stored in `~/.purr/history.json` and persists across sessions.

## Supported kubectl Commands

Purr supports **all** kubectl commands. Here are some with enhanced features:

| Command | Enhanced Features |
|---------|-------------------|
| `get` | Resource picker, namespace picker, output format picker |
| `describe` | Resource picker, namespace picker |
| `logs` | Pod picker, container picker (multi-container pods) |
| `exec` | Pod picker, container picker |
| `apply` | File picker |
| `delete` | Resource picker, confirmation dialog |
| `edit` | Resource picker |
| `port-forward` | Resource picker, port suggestions |
| `scale` | Resource picker, replica count display |
| `rollout` | Deployment picker |

## Architecture

```
purr/
├── cmd/purr/          # Main entry point
├── internal/
│   ├── tui/          # Bubble Tea UI components
│   ├── k8s/          # Kubernetes client and cache
│   ├── exec/         # kubectl execution
│   ├── history/      # Command history
│   └── config/       # Configuration management
└── pkg/types/        # Shared types
```

## Requirements

- Go 1.22 or higher
- kubectl installed and configured
- Access to a Kubernetes cluster

## Development

### Building

```bash
make build
```

### Running Tests

```bash
make test
```

### Running Locally

```bash
make run
```

## Why Purr?

kubectl is powerful but typing resource names, namespaces, and paths repeatedly can be tedious. Purr enhances kubectl with smart completions while maintaining 100% compatibility. You get:

- **No learning curve** - Use kubectl commands you already know
- **Speed boost** - Quick resource selection instead of typing names
- **Safety** - Confirmation dialogs for destructive operations
- **History** - Never lose that complex command you ran last week
- **Beauty** - A pleasant terminal experience

## Comparison with kubectl

| Feature | kubectl | Purr |
|---------|---------|------|
| All commands | ✅ | ✅ |
| Direct execution | ✅ | ✅ |
| Autocomplete | Shell-dependent | ✅ Built-in |
| Resource browsing | ❌ | ✅ Interactive |
| History search | Shell-dependent | ✅ Built-in |
| Visual feedback | Basic | ✅ Rich |
| Destructive confirmations | ❌ | ✅ Optional |

## Development & Testing

### Running Tests

Purr has comprehensive test coverage for core components:

```bash
# Run all tests
make test

# Run tests with coverage
go test -cover ./...

# Run tests verbosely
go test -v ./pkg/types/... ./internal/exec/... ./internal/history/...
```

### Test Coverage

```
✓ pkg/types        100.0% coverage
✓ internal/exec     74.1% coverage
✓ internal/history  97.5% coverage
─────────────────────────────────
  Overall:          ~90% coverage
```

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Install locally
make install

# Run without installing
make run
```

### Project Structure

```
purr/
├── cmd/purr/              # Main entry point
│   └── main.go
├── internal/
│   ├── tui/              # Bubble Tea UI components
│   │   ├── model.go      # Application state
│   │   ├── update.go     # Event handling & updates
│   │   ├── view.go       # Rendering logic
│   │   └── styles.go     # Lipgloss styling
│   ├── k8s/              # Kubernetes client & cache
│   │   ├── client.go     # K8s client initialization
│   │   └── cache.go      # Resource caching with watch
│   ├── exec/             # kubectl execution
│   │   ├── kubectl.go    # Command executor
│   │   └── parser.go     # Command parser
│   ├── history/          # Command history
│   │   └── history.go    # Persistent history with search
│   └── config/           # Configuration
│       └── config.go     # App configuration
└── pkg/types/            # Shared types
    └── types.go
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see LICENSE file for details

## Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) by Charm
- Inspired by the kubectl experience
- Thanks to the Kubernetes community

## Support

- Report issues: [GitHub Issues](https://github.com/tapcraft-io/purr/issues)
- Discussions: [GitHub Discussions](https://github.com/tapcraft-io/purr/discussions)

---

Made with ❤️ by the Tapcraft team
