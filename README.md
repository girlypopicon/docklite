# DockLite Complete Distribution

The complete DockLite stack - a minimal Docker management system with three modes of operation:
1. **Headless** - Agent only (no GUI)
2. **TUI** - Terminal UI client
3. **Full Stack** - Web GUI + Agent + TUI

## What's Included

- **`docklite-agent`** (17MB) - Go-based API server for Docker operations
- **`docklite-tui`** (8.5MB) - Terminal UI client with remote access
- **`webapp/`** - Next.js web interface with full dashboard
- **Headless mode** - Run the agent without any GUI
- **Full-stack mode** - Complete web interface + API + TUI

## Quick Start (3 Commands)

```bash
# 1. Clone the repository
git clone https://github.com/sgauth0/docklite-new.git
cd docklite-new

# 2. Run the installer (checks deps, builds binaries, installs npm packages)
./install.sh

# 3. Start everything
./start-fullstack.sh
```

Then open http://localhost:3000 in your browser!

**For remote servers:** Access via your server's IP address:
```
http://YOUR_SERVER_IP:3000
```

**Default credentials:**
- Username: `superadmin`
- Password: generated on first start and saved to `data/initial-admin-password` (`sudo cat /opt/docklite/data/initial-admin-password`)

## System Requirements

- **Docker** 20.10+ (required)
- **Node.js** 18+ (for web GUI)
- **Go** 1.22+ (only needed if building from source)

## Installation Methods

### Method 1: Quick Install (Recommended)

Run the included installer script which checks dependencies and builds binaries:

```bash
./install.sh              # Standard install
./install.sh --full        # Full install with systemd services
./install.sh --skip-build  # Use pre-built binaries
./install.sh --help        # Show all options
```

### Method 2: Manual Install

```bash
# 1. Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# 2. Install Node.js 18+
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs

# 3. Build binaries (requires Go)
./build.sh    # Or use: make build-all

# 4. Start
./start-fullstack.sh
```

## Quick Start Options

### Option 1: Full Stack (Web GUI + Agent + TUI)

**One command to start everything:**
```bash
./start-fullstack.sh
```

Then open http://localhost:3000 in your browser (or http://YOUR_SERVER_IP:3000 for remote access)

### Option 2: Headless Mode (Agent Only)

**Start just the agent:**
```bash
./start-agent.sh
```

**Connect with TUI:**
```bash
./start-tui.sh
```

### Option 3: Development Mode

**Terminal 1 - Agent:**
```bash
make run-agent
```

**Terminal 2 - GUI:**
```bash
make run-gui
```

**Terminal 3 - TUI:**
```bash
make run-tui
```

## System Requirements

- **Docker** 20.10+ (required)
- **Node.js** 18+ (for web GUI)
- **Go** 1.22+ (only needed if building from source)

## Installation Methods

### Method 1: Quick Install (Recommended)

Run the included installer script which checks dependencies and builds binaries:

```bash
./install.sh              # Standard install
./install.sh --full        # Full install with systemd services
./install.sh --skip-build  # Use pre-built binaries
./install.sh --help        # Show all options
```

### Method 2: Manual Install

```bash
# 1. Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# 2. Install Node.js 18+
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs

# 3. Build binaries (requires Go)
./build.sh    # Or use: make build-all

# 4. Start
./start-fullstack.sh
```

## Using DockLite from the terminal (and with Claude)

Everything the dashboard does is also a command. On the server:

```bash
docklite doctor              # is everything healthy? says how to fix what isn't
docklite containers list     # what's running (sites, databases, other)
docklite containers restart example.com
docklite sites create example.com
docklite ssl status
docklite docs                # the full guide
```

- **Other people on the server:** log in with your own DockLite account (`docklite login`, nothing else needed),
  or an admin can give a server user shell access with `docklite access grant <user>` (they can then run
  `docklite` with full admin rights, so treat it like handing out root for DockLite).
- **From your own computer:** `docklite --host https://your-panel.example.com login`, then the same commands.
- **Scripts:** add `--json`; exit codes are stable (see the guide). Destructive commands refuse to run from a
  script without `--yes`.
- **AI assistants (Claude and others):** tell it to run `docklite docs` first — the guide is built into the
  program, so it works even where this repository isn't checked out. `docklite commands --json` lists every
  command, flagged destructive or not, for tools that want a machine-readable list. The guide's source is
  [`go-app/cmd/docklite/docs/CLI.md`](go-app/cmd/docklite/docs/CLI.md).

## Features

**Web GUI (Next.js):**
- Full container dashboard with drag-and-drop organization
- Database management (PostgreSQL, MySQL, MongoDB)
- File browser and code editor
- Real-time container stats and logs
- Backup system with multiple destinations (S3, SFTP, local)
- DNS management (Cloudflare integration)
- SSL certificate status and management
- User management with role-based access control

**TUI Client:**
- Real-time container stats
- Log viewer
- File browser and editor
- Container management (start/stop/restart/remove)
- Remote access (connect to any agent)
- Persistent config storage

**Agent:**
- Direct Docker API integration
- SQLite metadata storage
- Token-based authentication
- Headless operation (no GUI required)
- Optional Next.js GUI proxy
- Single binary deployment

## Available Scripts

| Script | Description |
|--------|-------------|
| `install.sh` | Quick installer - checks deps, builds binaries, installs npm packages |
| `build.sh` | Build all binaries and prepare for distribution |
| `start-fullstack.sh` | Start GUI + Agent together (recommended) |
| `start-agent.sh` | Start agent in headless mode |
| `start-tui.sh` | Start TUI client |
| `stop-all.sh` | Stop all running services |

## Documentation

- **[INSTALL.md](INSTALL.md)** - Detailed Ubuntu/Debian installation guide
- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Complete deployment guide with all modes
- **[CLAUDE.md](CLAUDE.md)** - Developer documentation

## Architecture

**Full Stack Mode:**
```
Browser → Agent (port 3000) ──┬→ Next.js GUI (port 3001)
                              │
TUI Client → Agent ───────────┴→ Docker Daemon
                              │
                              ↓
                          SQLite DB
```

**Headless Mode:**
```
TUI Client → Agent (port 3000) → Docker Daemon
                                    ↓
                                SQLite DB
```

## Deployment Modes

### Full Stack (Recommended)
```bash
# One command starts everything
./start-fullstack.sh

# Access:
# - Browser: http://localhost:3000
# - TUI: Use token from startup output
```

### Headless (CLI/TUI Only)
```bash
# Server
./start-agent.sh

# Client (local or remote)
./start-tui.sh
```

### Manual Control
```bash
# Start GUI on port 3001
cd webapp && PORT=3001 npm start &

# Start agent on port 3000 (proxies to GUI)
NEXTJS_URL=http://localhost:3001 ./bin/docklite-agent &

# Or headless (no GUI)
NEXTJS_URL=disabled ./bin/docklite-agent
```

## Building from Source

Requirements:
- Go 1.22+ (Agent and TUI)
- Node.js 18+ (GUI)

```bash
make build-all     # Build everything (agent + TUI + GUI)
make build-agent   # Build agent only
make build-tui     # Build TUI only
make build-gui     # Build Next.js GUI
make install-gui   # Install GUI dependencies only
make clean         # Remove all build artifacts
```

Development mode:
```bash
make run-agent     # Run agent in dev mode
make run-tui       # Run TUI in dev mode
make run-gui       # Run GUI in dev mode (port 3000)
```

## Environment Variables

**Agent:**
- `LISTEN_ADDR` - Port to listen on (default: `:3000`)
- `DATABASE_PATH` - SQLite database path (default: `data/docklite.db`)
- `DOCKER_SOCKET_PATH` - Docker socket (default: `unix:///var/run/docker.sock`)
- `NEXTJS_URL` - Next.js proxy URL or `disabled` (default: `http://localhost:3001`)
- `DOCKLITE_TOKEN` - Authentication token (recommended: 32+ chars)

**GUI (Next.js):**
- `PORT` - Port to listen on (default: `3000`)
- `DATABASE_PATH` - SQLite database path (default: `data/docklite.db`)
- `AGENT_URL` - Agent API URL (default: `http://localhost:3000`)
- `AGENT_TOKEN` - Agent authentication token
- `SESSION_SECRET` - Session encryption key (32+ chars in production)

**TUI:**
- `DOCKLITE_URL` - Agent URL (default: `http://localhost:3000`)
- `DOCKLITE_TOKEN` - Authentication token

## Security

Generate secure tokens:
```bash
openssl rand -hex 32
```

Use HTTPS in production (reverse proxy recommended):
```bash
# Example with Caddy
caddy reverse-proxy --from docklite.example.com --to localhost:3000
```

## License

See main DockLite repository for license information.
