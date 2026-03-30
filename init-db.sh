#!/bin/bash
# Initialize DockLite database
# This creates an empty SQLite database that the GUI will populate with migrations

DATABASE_PATH="${1:-./data/docklite.db}"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Initializing DockLite database...${NC}"

# Create data directory
mkdir -p "$(dirname "$DATABASE_PATH")"

# Create empty SQLite database
sqlite3 "$DATABASE_PATH" "VACUUM;" 2>/dev/null

if [ -f "$DATABASE_PATH" ]; then
    chmod 600 "$DATABASE_PATH"
    echo -e "${GREEN}✓ Database initialized at $DATABASE_PATH${NC}"
    echo "The Next.js GUI will populate it with tables on first startup."
else
    echo "Failed to create database"
    exit 1
fi
