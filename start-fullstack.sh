#!/bin/bash
# DockLite Full Stack Startup Script
# Starts both the Next.js GUI (port 3001) and Agent (port 3000)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${GREEN}DockLite Full Stack Startup${NC}"
echo "================================"
echo ""

# Configuration
DATABASE_PATH="${DATABASE_PATH:-./data/docklite.db}"
DOCKLITE_TOKEN="${DOCKLITE_TOKEN:-}"
AGENT_PORT="${AGENT_PORT:-3000}"
GUI_PORT="${GUI_PORT:-3002}"
ENABLE_DB_DEBUG="${ENABLE_DB_DEBUG:-false}"
SESSION_SECRET="${SESSION_SECRET:-}"

# Kill any existing processes on the target ports
for port in $AGENT_PORT $GUI_PORT; do
    existing_pid=$(sudo lsof -ti :$port 2>/dev/null || true)
    if [ -n "$existing_pid" ]; then
        echo -e "${YELLOW}Killing existing process on port $port (PID: $existing_pid)...${NC}"
        sudo kill $existing_pid 2>/dev/null || true
        sleep 1
    fi
done

# Also kill tracked PIDs from previous runs
for pidfile in .docklite-gui.pid .docklite-agent.pid; do
    if [ -f "$pidfile" ]; then
        old_pid=$(cat "$pidfile" 2>/dev/null)
        if [ -n "$old_pid" ] && kill -0 "$old_pid" 2>/dev/null; then
            echo -e "${YELLOW}Killing previous process (PID: $old_pid)...${NC}"
            kill "$old_pid" 2>/dev/null || true
        fi
        rm -f "$pidfile"
    fi
done
sleep 1

# Check if binaries exist, build if missing
if [ ! -f "./bin/docklite-agent" ]; then
    echo -e "${YELLOW}Agent binary not found, building...${NC}"
    if command -v go >/dev/null 2>&1; then
        make build-agent
    else
        echo -e "${RED}Error: Go not installed${NC}"
        echo "Please install Go 1.22+ or pre-build the binaries:"
        echo "  https://go.dev/dl/"
        exit 1
    fi
fi

if [ ! -f "./bin/docklite-tui" ]; then
    echo -e "${YELLOW}TUI binary not found, building...${NC}"
    if command -v go >/dev/null 2>&1; then
        make build-tui
    else
        echo -e "${YELLOW}Warning: TUI binary not built (Go not installed)${NC}"
        echo "The web GUI will still work without the TUI client."
    fi
fi

# Check if webapp exists
if [ ! -d "./webapp" ]; then
    echo -e "${RED}Error: webapp directory not found${NC}"
    exit 1
fi

# Check if node_modules exists
if [ ! -d "./webapp/node_modules" ]; then
    echo -e "${YELLOW}Installing webapp dependencies...${NC}"
    cd webapp && npm install
    cd ..
fi

# Check if Next.js is built
if [ ! -d "./webapp/.next" ]; then
    echo -e "${YELLOW}Building Next.js app...${NC}"
    cd webapp && npm run build
    cd ..
fi

# Check if Docker is running
if ! docker info >/dev/null 2>&1; then
    echo -e "${RED}Error: Docker is not running or not accessible${NC}"
    exit 1
fi

# Create data directory if it doesn't exist
mkdir -p ./data

# Check if database exists
if [ ! -f "$DATABASE_PATH" ]; then
    echo -e "${YELLOW}Database not found at $DATABASE_PATH${NC}"
    echo -e "${YELLOW}Creating empty database file...${NC}"
    touch "$DATABASE_PATH"
    chmod 600 "$DATABASE_PATH"
    echo -e "${GREEN}✓ Empty database file created${NC}"
    echo "The GUI will initialize tables on first startup."
    echo ""
fi

# Ensure secure database permissions even for existing files
chmod 600 "$DATABASE_PATH" 2>/dev/null || true

# Generate token if not set
if [ -z "$DOCKLITE_TOKEN" ]; then
    echo -e "${YELLOW}Generating authentication token...${NC}"
    DOCKLITE_TOKEN=$(openssl rand -hex 32)
    umask 077
    printf "%s\n" "$DOCKLITE_TOKEN" > ./data/docklite.token
    echo -e "${GREEN}Generated token and saved to ./data/docklite.token${NC}"
    echo "Use this token for TUI/API access."
    echo ""
fi

# Display configuration
echo -e "${BLUE}Configuration:${NC}"
echo "  Database: $DATABASE_PATH"
echo "  Agent Port: $AGENT_PORT"
echo "  GUI Port: $GUI_PORT"
echo "  Token: (set)"
echo "  DB Debug: $ENABLE_DB_DEBUG"
echo ""

# Create log directory
mkdir -p ./logs

# Start Next.js GUI
echo -e "${GREEN}Starting Next.js GUI on port $GUI_PORT...${NC}"
cd webapp
# Avoid weak predictable secrets; generate an ephemeral secret if one is not provided.
if [ -z "$SESSION_SECRET" ]; then
    SESSION_SECRET=$(openssl rand -hex 32)
    echo -e "${YELLOW}Warning: SESSION_SECRET not set, using ephemeral secret for this run only.${NC}"
fi

PORT=$GUI_PORT \
HOSTNAME="0.0.0.0" \
DATABASE_PATH="../$DATABASE_PATH" \
AGENT_URL="http://localhost:$AGENT_PORT" \
AGENT_TOKEN="$DOCKLITE_TOKEN" \
SESSION_SECRET="$SESSION_SECRET" \
npm start > ../logs/nextjs.log 2>&1 &
GUI_PID=$!
cd ..

echo -e "${GREEN}✓ GUI started (PID: $GUI_PID)${NC}"
echo -e "${BLUE}Waiting for GUI to initialize database...${NC}"

# Start Agent
echo -e "${GREEN}Starting Agent on port $AGENT_PORT...${NC}"
LISTEN_ADDR=":$AGENT_PORT" \
DATABASE_PATH="$DATABASE_PATH" \
DOCKER_SOCKET_PATH="unix:///var/run/docker.sock" \
NEXTJS_URL="http://127.0.0.1:$GUI_PORT" \
DOCKLITE_TOKEN="$DOCKLITE_TOKEN" \
ENABLE_DB_DEBUG="$ENABLE_DB_DEBUG" \
./bin/docklite-agent > ./logs/agent.log 2>&1 &
AGENT_PID=$!

echo -e "${GREEN}✓ Agent started (PID: $AGENT_PID)${NC}"
sleep 2

# Check if Agent is running
if ! kill -0 $AGENT_PID 2>/dev/null; then
    echo -e "${RED}✗ Agent failed to start${NC}"
    echo "Check logs/agent.log for errors"
    kill $GUI_PID 2>/dev/null
    exit 1
fi

# Test agent health
echo ""
echo -e "${BLUE}Testing connections...${NC}"
sleep 1

if curl -s -f "http://localhost:$AGENT_PORT/api/health" -H "Authorization: Bearer $DOCKLITE_TOKEN" >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Agent is healthy${NC}"
else
    echo -e "${RED}✗ Agent health check failed${NC}"
fi

if curl -s -f "http://localhost:$AGENT_PORT/" >/dev/null 2>&1; then
    echo -e "${GREEN}✓ GUI is accessible${NC}"
else
    echo -e "${YELLOW}⚠ GUI may still be starting...${NC}"
fi

echo ""
echo "================================"
echo -e "${GREEN}DockLite Full Stack Running!${NC}"
echo ""
echo -e "${BLUE}Access:${NC}"
echo "  🌐 Web GUI:   http://localhost:$AGENT_PORT"
echo "  🔧 Agent API: http://localhost:$AGENT_PORT/api/*"
echo "  💻 TUI:       DOCKLITE_URL=http://localhost:$AGENT_PORT DOCKLITE_TOKEN=<token> ./bin/docklite-tui"
echo ""
echo -e "${YELLOW}For remote access, use your server's IP:${NC}"
echo "  🌐 Web GUI:   http://YOUR_SERVER_IP:$AGENT_PORT"
echo ""
echo -e "${BLUE}Processes:${NC}"
echo "  GUI PID:   $GUI_PID (port $GUI_PORT)"
echo "  Agent PID: $AGENT_PID (port $AGENT_PORT)"
echo ""
echo -e "${BLUE}Logs:${NC}"
echo "  GUI:   tail -f logs/nextjs.log"
echo "  Agent: tail -f logs/agent.log"
echo ""
echo -e "${BLUE}Stop:${NC}"
echo "  kill $GUI_PID $AGENT_PID"
echo ""
echo "Press Ctrl+C to stop all services"
echo "================================"

# Save PIDs to file
echo "$GUI_PID" > .docklite-gui.pid
echo "$AGENT_PID" > .docklite-agent.pid

# Wait for interrupt
trap "echo ''; echo 'Stopping services...'; kill $GUI_PID $AGENT_PID 2>/dev/null; rm -f .docklite-gui.pid .docklite-agent.pid; exit 0" INT TERM

# Keep script running
wait
