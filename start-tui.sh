#!/bin/bash
# DockLite TUI Client Startup Script

# Configuration
DOCKLITE_URL="${DOCKLITE_URL:-http://localhost:3000}"
DOCKLITE_TOKEN="${DOCKLITE_TOKEN:-}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}DockLite TUI Client${NC}"
echo "================================"

# Check if binary exists
if [ ! -f "./bin/docklite-tui" ]; then
    echo -e "${RED}Error: docklite-tui binary not found${NC}"
    echo "Run 'make build-tui' first"
    exit 1
fi

# Check if token is set
if [ -z "$DOCKLITE_TOKEN" ]; then
    echo -e "${YELLOW}No DOCKLITE_TOKEN set${NC}"
    echo "You will be prompted to enter connection details in the TUI"
    echo ""
fi

# Display configuration
echo "Configuration:"
echo "  Agent URL: $DOCKLITE_URL"
if [ -n "$DOCKLITE_TOKEN" ]; then
    echo "  Token: (set)"
else
    echo "  Token: (will prompt)"
fi
echo ""

# Test connection if token is set
if [ -n "$DOCKLITE_TOKEN" ]; then
    echo "Testing connection to agent..."
    if curl -s -f -H "Authorization: Bearer $DOCKLITE_TOKEN" "$DOCKLITE_URL/api/health" >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Connection successful${NC}"
    else
        echo -e "${RED}✗ Cannot connect to agent${NC}"
        echo "Make sure:"
        echo "  1. Agent is running: ./start-agent.sh"
        echo "  2. URL is correct: $DOCKLITE_URL"
        echo "  3. Token is correct"
        echo ""
        read -p "Continue anyway? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
fi

echo "================================"
echo ""

# Export environment variables
export DOCKLITE_URL
export DOCKLITE_TOKEN

# Start the TUI
exec ./bin/docklite-tui
