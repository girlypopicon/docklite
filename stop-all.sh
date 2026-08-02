#!/bin/bash
# Legacy wrapper — delegates to ./docklite
exec "$(dirname "$0")/docklite" stop "$@"
