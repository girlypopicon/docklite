#!/bin/bash
# DockLite Agent Startup Script

# Configuration
LISTEN_ADDR="${LISTEN_ADDR:-:3000}"
DATABASE_PATH="${DATABASE_PATH:-./data/docklite.db}"
DOCKER_SOCKET_PATH="${DOCKER_SOCKET_PATH:-unix:///var/run/docker.sock}"
NEXTJS_URL="${NEXTJS_URL:-disabled}"
DOCKLITE_TOKEN="${DOCKLITE_TOKEN:-}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}DockLite Agent Startup${NC}"
echo "================================"

# Check if binary exists
if [ ! -f "./bin/docklite-agent" ]; then
    echo -e "${RED}Error: docklite-agent binary not found${NC}"
    echo "Run 'make build-agent' first"
    exit 1
fi

# Check if Docker is running
if ! docker info >/dev/null 2>&1; then
    echo -e "${RED}Error: Docker is not running or not accessible${NC}"
    echo "Make sure Docker is installed and you have permission to access it"
    exit 1
fi

# Create data directory if it doesn't exist
mkdir -p ./data

# Check if database exists
if [ ! -f "$DATABASE_PATH" ]; then
    echo -e "${YELLOW}Database not found at $DATABASE_PATH${NC}"
    echo -e "${YELLOW}Creating empty SQLite database...${NC}"
    # Create a valid empty SQLite database (not just an empty file)
    if command -v sqlite3 >/dev/null 2>&1; then
        sqlite3 "$DATABASE_PATH" "VACUUM;"
    else
        # Fallback: create minimal SQLite header
        printf '\x53\x51\x4c\x69\x74\x65\x20\x66\x6f\x72\x6d\x61\x74\x20\x33\x00' > "$DATABASE_PATH"
    fi
    chmod 660 "$DATABASE_PATH"
    echo -e "${GREEN}✓ Empty database created${NC}"
    echo "Note: Run with Next.js GUI first to initialize tables via migrations."
    echo ""
fi

# Generate token if not set
if [ -z "$DOCKLITE_TOKEN" ]; then
    echo -e "${YELLOW}Warning: DOCKLITE_TOKEN not set${NC}"
    echo "Generating random token..."
    DOCKLITE_TOKEN=$(openssl rand -hex 32)
    echo -e "${GREEN}Generated token: $DOCKLITE_TOKEN${NC}"
    echo "Save this token for TUI client access!"
    echo ""
fi

# Display configuration
echo "Configuration:"
echo "  Listen Address: $LISTEN_ADDR"
echo "  Database Path: $DATABASE_PATH"
echo "  Docker Socket: $DOCKER_SOCKET_PATH"
echo "  Next.js URL: $NEXTJS_URL"
echo "  Token: ${DOCKLITE_TOKEN:0:16}..."
echo ""

# Check mode
if [ "$NEXTJS_URL" = "disabled" ] || [ "$NEXTJS_URL" = "none" ]; then
    echo -e "${GREEN}Starting in HEADLESS mode (no GUI)${NC}"
else
    echo -e "${GREEN}Starting in PROXY mode (with Next.js at $NEXTJS_URL)${NC}"
fi

echo "================================"
echo ""

# Export environment variables
export LISTEN_ADDR
export DATABASE_PATH
export DOCKER_SOCKET_PATH
export NEXTJS_URL
export DOCKLITE_TOKEN

# Start the agent
exec ./bin/docklite-agent
