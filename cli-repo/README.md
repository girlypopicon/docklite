# Docklite TUI

A Terminal User Interface for Docklite built with Bubble Tea and Lipgloss.

## Features

- **Dashboard**: View system summary and health status
- **Containers**: List, search, and view container details
- **Logs**: Container logs viewer (coming soon)
- **Network**: Network management (coming soon)
- **Agent**: Agent management interface (coming soon)

## Navigation

- **↑/↓**: Move selection in focused panel
- **Enter**: Select item (switch screen in sidebar, open details in container list)
- **Tab**: Cycle focus between sidebar and main panel
- **Esc**: Go back from details view or exit search mode
- **/**: Start search (Containers screen only)
- **Ctrl+C**: Quit application

## Prerequisites

- Go 1.21 or higher
- Docklite API running (default: http://localhost:8080)

## Installation

1. Clone or download this repository
2. Install dependencies:

```powershell
go mod download
```

## Configuration

The TUI connects to the Docklite API using the `DOCKLITE_URL` environment variable.

Default: `http://localhost:8080`

To use a different URL:

```powershell
$env:DOCKLITE_URL = "http://localhost:9000"
```

## Running

```powershell
go run .
```

Or build and run:

```powershell
go build -o docklite-tui.exe
.\docklite-tui.exe
```

## Project Structure

```
.
├── main.go           # Application entry point
├── api/
│   └── client.go     # Docklite API client
├── tui/
│   ├── model.go      # Bubble Tea model and state management
│   └── views.go      # UI rendering and styles
└── go.mod            # Go module definition
```

## API Endpoints Used

- `GET /api/health` - Health check
- `GET /api/summary` - System summary (containers, images, volumes count)
- `GET /api/containers` - List all containers

Data is automatically refreshed every 10 seconds.

## Development

The TUI is built with:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Style and layout

To modify styles, edit `tui/views.go`.
To add new screens or functionality, update `tui/model.go` and `tui/views.go`.

## Troubleshooting

**Connection errors**: Ensure Docklite API is running and accessible at the configured URL.

**Display issues**: Make sure your terminal supports ANSI colors and has adequate size (minimum 80x24 recommended).
