#!/bin/bash
# DockLite Build Script
# Builds all binaries and prepares for distribution

set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$REPO_DIR"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}=== DockLite Build Script ===${NC}"
echo ""

# Check Go
if ! command -v go >/dev/null 2>&1; then
    echo -e "${RED}Go is not installed${NC}"
    exit 1
fi

# Check bun
if ! command -v bun >/dev/null 2>&1; then
    echo -e "${RED}bun is not installed${NC}"
    exit 1
fi

# Create bin directory
mkdir -p bin

# Build Go binaries
echo -e "${YELLOW}Building Go binaries...${NC}"
echo ""

echo "Building agent..."
cd go-app
go build -o ../bin/docklite-agent ./cmd/docklite-agent
cd ..
if [[ -f "bin/docklite-agent" ]]; then
    echo -e "${GREEN}✓ Agent built (${BIN_SIZE:-$(du -h bin/docklite-agent | cut -f1)})${NC}"
else
    echo -e "${RED}✗ Agent build failed${NC}"
    exit 1
fi

echo "Building TUI..."
cd cli-repo
go build -o ../bin/docklite-tui .
cd ..
if [[ -f "bin/docklite-tui" ]]; then
    echo -e "${GREEN}✓ TUI built ($(du -h bin/docklite-tui | cut -f1))${NC}"
else
    echo -e "${YELLOW}⚠ TUI build failed (optional)${NC}"
fi

echo ""
echo -e "${YELLOW}Building Next.js app...${NC}"
cd webapp
bun install
bun run build
cd ..
if [[ -d "webapp/.next" ]]; then
    echo -e "${GREEN}✓ Next.js built${NC}"
else
    echo -e "${RED}✗ Next.js build failed${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}=== Build Complete! ===${NC}"
echo ""
echo "Binaries available in ./bin/:"
ls -lh bin/
echo ""
echo "Next.js build available in ./webapp/.next/"
echo ""
echo "Run with: ./start-fullstack.sh"
