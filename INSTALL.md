# DockLite Installation Guide

## Quick Install (Ubuntu/Debian)

### Prerequisites

Install only what you need:

```bash
# Docker (required)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
newgrp docker

# Node.js 18+ (required for web GUI)
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs
```

### Install DockLite

```bash
# Clone repository
git clone https://github.com/sgauth0/docklite-new.git
cd docklite-new

# Start everything (one command!)
./start-fullstack.sh
```

That's it! Open http://localhost:3000 in your browser.

**Default credentials:**
- Username: `superadmin`
- Password: generated on first start and saved to `data/initial-admin-password` (`sudo cat /opt/docklite/data/initial-admin-password`)

## What Gets Installed

- **Pre-built binaries** (no Go compiler needed):
  - `bin/docklite-agent` (18MB) - Go backend
  - `bin/docklite-tui` (8.5MB) - Terminal UI client

- **Node.js dependencies** (auto-installed on first run):
  - Next.js and React for web GUI
  - better-sqlite3 for database

## Installation Modes

### Full Stack (Recommended)
Web GUI + Agent + TUI - everything in one command:
```bash
./start-fullstack.sh
```

### Headless Mode (No GUI)
Agent only with TUI access:
```bash
./start-agent.sh
./start-tui.sh
```

### Development Mode
Run from source with hot reload:
```bash
make run-agent   # Terminal 1
make run-gui     # Terminal 2
make run-tui     # Terminal 3
```

## Troubleshooting

### "Docker is not running"
```bash
sudo systemctl start docker
sudo usermod -aG docker $USER
newgrp docker
```

### "Port already in use"
```bash
./stop-all.sh
# Or manually kill processes
ps aux | grep docklite
```

### Check logs
```bash
tail -f logs/agent.log
tail -f logs/nextjs.log
```

### Clean install
```bash
./stop-all.sh
rm -rf data/ logs/ webapp/.next/ webapp/node_modules/
./start-fullstack.sh
```

## System Requirements

- **OS**: Ubuntu 20.04+, Debian 11+, or any modern Linux
- **CPU**: 1 core minimum, 2+ recommended
- **RAM**: 512MB minimum, 1GB+ recommended
- **Disk**: 2GB for DockLite + Docker images
- **Docker**: Version 20.10 or newer
- **Node.js**: Version 18 or newer

## Network Ports

- **3000** - Main access (agent + web GUI proxy)
- **3001** - Internal Next.js GUI (not exposed)

## Security

Generate secure tokens for production:
```bash
openssl rand -hex 32
```

Set in environment or `.env` file:
```bash
DOCKLITE_TOKEN=your-secure-token-here
SESSION_SECRET=your-session-secret-here
```

## Next Steps

After installation:
1. Log in with default credentials
2. **Change the superadmin password immediately**
3. Create additional users (Settings → Users)
4. Configure backup destinations (Backups)
5. Set up DNS integration if needed (Network → DNS)

## Getting Help

- **Documentation**: See `README.md` and `CLAUDE.md`
- **Issues**: https://github.com/sgauth0/docklite-new/issues
- **Logs**: Check `logs/agent.log` and `logs/nextjs.log`
