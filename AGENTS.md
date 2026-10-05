# DockLite - Agent Guidelines

## Project Overview

DockLite is a minimal Docker management system that provides a complete stack for managing Docker containers, databases, backups, DNS, and SSL certificates. It is designed to run in three modes:

1. **Full Stack** - Web GUI + Agent + TUI
2. **Headless** - Agent only (no GUI, TUI/CLI access)
3. **TUI Only** - Terminal UI client connecting to remote agents

### Architecture

```
┌──────────────────┐
│  docklite-tui    │  (Standalone TUI Client - Bubble Tea)
│  - Local/Remote  │
└────────┬─────────┘
         │ HTTP API (port 3000)
         ↓
┌────────────────────────────┐
│   docklite-agent           │  (Go API Server)
│   ├─ /api/* routes         │ → Docker operations
│   └─ /* (other routes)     │ → Proxy to Next.js (optional)
└────────┬───────────────────┘
         │
         ↓
    Docker Daemon
         ↓
   SQLite Database (metadata)
```


**Operating a DockLite server (as opposed to changing this code):** run `docklite docs` and `docklite doctor`
first. The CLI (`go-app/cmd/docklite`, installed as `docklite`) covers containers, sites, SSL, nginx, DNS, users,
backups and server health, always supports `--json`, and refuses destructive actions without `--yes` — only pass
`--yes` when the person you're working for asked for that exact action. The guide's source is
`go-app/cmd/docklite/docs/CLI.md`; update it whenever commands change.

## Technology Stack

### Backend (Agent)
- **Language**: Go 1.22+
- **Key Dependencies**:
  - `github.com/docker/docker` - Docker API client
  - `github.com/gorilla/websocket` - WebSocket support for terminals
  - `golang.org/x/crypto` - Password hashing
  - `modernc.org/sqlite` - SQLite driver
- **Location**: `go-app/`

### Web GUI
- **Framework**: Next.js 14+ (App Router)
- **Language**: TypeScript
- **Styling**: Tailwind CSS with custom "vapor/neon" theme
- **Key Dependencies**:
  - `@dnd-kit/core` & `@dnd-kit/sortable` - Drag and drop
  - `better-sqlite3` - Database access
  - `dockerode` - Docker integration
  - `iron-session` - Session management
  - `xterm` - Terminal emulation
- **Location**: `webapp/`

### TUI Client
- **Language**: Go 1.21+
- **Framework**: Bubble Tea (Charmbracelet)
- **Key Dependencies**:
  - `github.com/charmbracelet/bubbletea` - TUI framework
  - `github.com/charmbracelet/lipgloss` - Styling
- **Location**: `cli-repo/`

## Project Structure

```
/home/DOCKLITE_GEM_TEST/
├── go-app/                    # Go Agent API Server
│   ├── cmd/
│   │   ├── docklite-agent/    # Main agent entry point
│   │   └── docklite/          # Additional CLI
│   ├── internal/
│   │   ├── api/               # HTTP router & proxy
│   │   ├── backup/            # Backup scheduler & artifacts
│   │   ├── cli/               # CLI client utilities
│   │   ├── cloudflare/        # Cloudflare DNS integration
│   │   ├── config/            # Configuration management
│   │   ├── docker/            # Docker client wrappers
│   │   ├── handlers/          # HTTP handlers
│   │   ├── models/            # Data models
│   │   └── store/             # SQLite storage layer
│   ├── go.mod
│   └── go.sum
│
├── webapp/                    # Next.js Web Dashboard
│   ├── app/                   # App Router
│   │   ├── (auth)/            # Auth routes (login)
│   │   ├── (dashboard)/       # Main dashboard
│   │   │   ├── components/    # Shared components
│   │   │   ├── containers/    # Container management
│   │   │   ├── databases/     # Database management
│   │   │   ├── backups/       # Backup management
│   │   │   ├── dns/           # DNS management
│   │   │   ├── network/       # Network tools
│   │   │   ├── settings/      # System settings
│   │   │   └── users/         # User management
│   │   ├── api/               # API routes (proxied to agent)
│   │   ├── globals.css
│   │   └── layout.tsx
│   ├── lib/                   # Server utilities
│   │   ├── db.ts              # Database operations
│   │   ├── docker.ts          # Docker helpers
│   │   ├── migrations/        # Database migrations (001-015)
│   │   ├── templates/         # Deployment templates
│   │   └── hooks/             # React hooks
│   ├── types/                 # TypeScript types
│   ├── package.json
│   └── next.config.js
│
├── cli-repo/                  # TUI Client
│   ├── api/                   # API client
│   ├── tui/                   # TUI components
│   │   ├── model.go           # Main model/state
│   │   ├── views.go           # UI views
│   │   └── config.go          # Config management
│   ├── main.go
│   ├── go.mod
│   └── go.sum
│
├── bin/                       # Built binaries
│   ├── docklite-agent
│   └── docklite-tui
│
├── data/                      # SQLite database files
│   └── docklite.db
│
├── logs/                      # Log files
├── Makefile                   # Build commands
├── start-fullstack.sh         # Start GUI + Agent
├── start-agent.sh             # Start agent only
├── start-tui.sh               # Start TUI client
└── stop-all.sh                # Stop all services
```

## Build Commands

### Build All Components
```bash
make build-all          # Build agent, TUI, and GUI
```

### Individual Builds
```bash
make build-agent        # Build docklite-agent binary
make build-tui          # Build docklite-tui binary
make build-gui          # Build Next.js production bundle
make install-gui        # Install GUI dependencies only
```

### Development Mode
```bash
make run-agent          # Run agent in dev mode (port 9000)
make run-tui            # Run TUI in dev mode
make run-gui            # Run GUI in dev mode (port 3000)
```

### Scripts
```bash
./start-fullstack.sh    # Start GUI + Agent together
./start-agent.sh        # Start agent only (headless)
./start-tui.sh          # Start TUI client
./stop-all.sh           # Stop all running services
```

### Clean
```bash
make clean              # Remove binaries and build artifacts
```

## Code Style Guidelines

### Go Code
- Run `gofmt` on all Go files before committing
- Package layout follows standard Go conventions under `go-app/internal/`
- Use `snake_case.go` for file names
- Keep handlers modular - one file per major feature area

### TypeScript/React Code
- Follow existing TypeScript/TSX formatting
- Run `npm run lint` in `webapp/` before committing
- Use PascalCase for component files (e.g., `ContainerCard.tsx`)
- Colocate components with their routes when possible

### Database Migrations
- Migrations are in `webapp/lib/migrations/`
- Naming: `XXX_description.ts` (e.g., `001_initial_schema.ts`)
- Each migration must export `version`, `name`, `up()`, and `down()`
- Migrations are automatically applied on startup

## Environment Variables

### Agent (`docklite-agent`)
| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:3000` | Port to listen on |
| `DATABASE_PATH` | `data/docklite.db` | SQLite database location |
| `DOCKER_SOCKET_PATH` | `unix:///var/run/docker.sock` | Docker socket path |
| `NEXTJS_URL` | `http://localhost:3001` | Next.js proxy URL (or `disabled` for headless) |
| `DOCKLITE_TOKEN` | (empty) | Shared secret for API authentication |
| `BACKUP_BASE_DIR` | `/var/backups/docklite` | Backup storage directory |

### GUI (Next.js)
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3000` | Port to listen on |
| `DATABASE_PATH` | `data/docklite.db` | SQLite database path |
| `AGENT_URL` | `http://localhost:3000` | Agent API URL |
| `AGENT_TOKEN` | (empty) | Agent authentication token |
| `SESSION_SECRET` | (empty) | Session encryption key (32+ chars) |
| — | — | SSL is now managed by certbot + nginx (ACME_PATH removed) |

### TUI Client
| Variable | Default | Description |
|----------|---------|-------------|
| `DOCKLITE_URL` | `http://localhost:3000` | Agent API URL |
| `DOCKLITE_TOKEN` | (empty) | Authentication token |

## Web GUI Theming Notes

### Multi-Theme Support
The Web GUI supports multiple themes (cyberpunk, corpo, corpo-blue, unicorn). When adding new UI components or styling:

**⚠️ IMPORTANT: Avoid Hardcoded Colors**
- **Never use hardcoded hex colors** like `#000`, `#0a0a1e`, `#1a0a2e`, `#ffffff`
- **Never use Tailwind's gray scale** for backgrounds/text: `bg-gray-900`, `text-gray-300`, etc.
- **Never use pure black backdrops** like `bg-black/80` or `rgba(0, 0, 0, 0.8)`

**✅ Use Theme Variables Instead**
Always use CSS custom properties from `webapp/app/globals.css`:
- Backgrounds: `var(--bg-dark)`, `var(--bg-darker)`, `var(--card-bg-1)`, `var(--surface-muted)`
- Text: `var(--text-primary)`, `var(--text-secondary)`, `var(--text-muted)`
- Accent colors: `var(--neon-cyan)`, `var(--neon-pink)`, `var(--neon-purple)`
- Modal backdrops: `var(--modal-backdrop)`

**Common Dark Spot Offenders to Watch For:**
- Modal/drawer backdrops with `bg-black/80` or `rgba(0, 0, 0, 0.8)`
- Code blocks with `bg-black/50`
- Debug/emergency pages with inline `style={{ background: '#000' }}`
- Status badges using `bg-dark-bg/60` (especially for light themes like Unicorn)
- Dropdown menus with gradient backgrounds using `--neon-purple`/`--neon-cyan`

**When Adding New Themes:**
1. Add theme variables to `:root` in `globals.css`
2. Create `[data-theme='themename']` CSS section
3. Test ALL pages - modals, dropdowns, code blocks, debug pages
4. Add theme-specific overrides in the CSS if needed
5. Update `webapp/app/theme-init.tsx` for migration if renaming themes

## Testing Guidelines

- There is no dedicated automated test suite currently
- For Go changes: `go test ./...` from `go-app/`
- For web UI changes: manual testing via browser
- Always test both headless and full-stack modes for agent changes

### Manual Testing Checklist
- [ ] Agent starts successfully in both headless and proxy modes
- [ ] TUI can connect and authenticate
- [ ] Web GUI loads and can perform CRUD operations
- [ ] Container operations (start/stop/restart/logs) work
- [ ] Database management functions correctly
- [ ] File browser and editor work as expected
- [ ] Backup operations complete successfully

## Security Considerations

### Token Generation
Generate secure tokens using:
```bash
openssl rand -hex 32
```

### File Permissions
```bash
chmod 600 /path/to/docklite.db        # Database
chmod 600 ~/.config/docklite/config.json  # TUI config
chmod 755 ./bin/*                      # Binaries
```

### Production Deployment
- SSL is managed automatically via nginx + certbot (Certificates tab or `/api/ssl/issue`)
- Do not expose port 3000 publicly without TLS
- Set strong `SESSION_SECRET` and `DOCKLITE_TOKEN`
- Run agent as user in `docker` group, not root if possible

## Database Schema

The application uses SQLite with the following main tables:

- `users` - User accounts with role-based access
- `sites` - Deployed website containers
- `databases` - Database containers (PostgreSQL, MySQL, MongoDB, SQLite)
- `database_permissions` - User access to databases
- `folders` - Container organization folders
- `folder_containers` - Container-to-folder mappings
- `backup_destinations` - Backup target configurations
- `backup_jobs` - Scheduled backup jobs
- `backups` - Backup history
- `cloudflare_config` & `dns_zones` & `dns_records` - DNS management
- `tokens` - API authentication tokens
- `migrations` - Schema version tracking

## API Endpoints

All API endpoints require `Authorization: Bearer <token>` header except `/api/auth/login`.

Key endpoint groups:
- `/api/containers/*` - Container management
- `/api/databases/*` - Database operations
- `/api/files/*` - File system access
- `/api/backups/*` - Backup management
- `/api/dns/*` - DNS management
- `/api/server/*` - Server stats and monitoring
- `/api/network/*` - Network diagnostics
- `/api/auth/*` - Authentication

## Common Development Tasks

### Adding a New Database Migration
1. Create file in `webapp/lib/migrations/XXX_name.ts`
2. Export `version`, `name`, `up(db)`, and `down(db)`
3. Import and add to `allMigrations` array in `index.ts`
4. Migration runs automatically on next startup

### Adding a New API Endpoint
1. Add handler in `go-app/internal/handlers/` (existing or new file)
2. Register route in `go-app/internal/api/router.go`
3. Add corresponding type definitions in `webapp/types/` if needed

### Adding a New Page to Web GUI
1. Create route folder in `webapp/app/(dashboard)/`
2. Add `page.tsx` with your component
3. Update navigation in `webapp/app/(dashboard)/nav.tsx`

### Modifying TUI Screens
1. Update model state in `cli-repo/tui/model.go`
2. Add view rendering in `cli-repo/tui/views.go`
3. Handle input in model's `handleKeyPress` method

## Deployment Notes

See detailed deployment guide in `docs/INSTALL.md`.

Quick production checklist:
- [ ] Set `DOCKLITE_TOKEN` to secure random string
- [ ] Set `SESSION_SECRET` to 32+ character random string
- [ ] Configure reverse proxy for HTTPS
- [ ] Set proper file permissions on database
- [ ] Configure backup destinations
- [ ] Test TUI connectivity

## Troubleshooting

### Agent won't start
- Check database permissions: `ls -la ./data/docklite.db`
- Check Docker socket access: `ls -la /var/run/docker.sock`
- Verify Docker is running: `docker ps`

### TUI can't connect
- Test agent health: `curl -H "Authorization: Bearer TOKEN" http://localhost:3000/api/health`
- Check token is correct
- Verify network connectivity

### Database errors
- Check migrations ran: Look for `migrations` table
- Restore from backup if needed: `*.db.pre-migration-*` files
