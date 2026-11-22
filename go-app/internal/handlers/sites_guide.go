package handlers

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// A note for AI assistants (and people) working inside /var/www/sites: Claude Code reads CLAUDE.md from the folder it was
// opened in and from every folder above it, so one file here covers every site. It says: make DockLite things with the
// docklite command, so nothing exists that DockLite can't see (a database container made with plain docker is invisible
// to the dashboard, backups and the TUI).

const sitesGuideMarker = "<!-- docklite-guide:1 -->"

const sitesGuide = sitesGuideMarker + `
# Working in a DockLite sites folder

These folders are websites run by **DockLite**, a control panel. Each site is ` + "`<user>/<domain>/`" + ` and runs in its own
container. DockLite keeps the list of sites, databases, certificates and backups; **anything made outside DockLite is
invisible to it** (not shown, not backed up, not in the dashboard or the terminal app).

## Rules

1. **Make and manage DockLite things with the ` + "`docklite`" + ` command, never with plain ` + "`docker`" + `, ` + "`nginx`" + ` or ` + "`certbot`" + `.**
   Run ` + "`docklite docs`" + ` once for the whole guide, ` + "`docklite doctor`" + ` to check health, ` + "`docklite help <command>`" + ` for any command.
   Every command takes ` + "`--json`" + `.
2. **A database for a site** is created with ` + "`docklite databases create <name> --type postgres|mysql|mariadb|mongodb|redis`" + `.
   It prints the connection details once; put them in the site's own config. Never start a database container by hand.
3. **SQLite** needs no server: keep the file inside this site's folder. DockLite finds it by itself
   (` + "`docklite databases scan`" + ` finds it right away) and shows it on the site's card. To register one by hand:
   ` + "`docklite databases create <name> --type sqlite --site <domain> --path /app/<file>.db`" + `.
4. **A new site:** ` + "`docklite sites create <domain> [--type static|php|node]`" + `. To change a site's kind:
   ` + "`docklite sites type <domain> node --port 3000 --yes`" + `. To send one domain to another:
   ` + "`docklite sites forward <new-domain> --to <site-domain> [--mode redirect|proxy]`" + `.
5. **HTTPS:** ` + "`docklite ssl issue <domain> --www`" + ` once the domain's DNS points at this server. Renewal is automatic.
6. **Files** here you may edit directly (this folder is group-writable for the docklite group). Don't move or rename a
   site's folder yourself; use ` + "`docklite sites ...`" + ` so the container and nginx follow.
7. **Destructive commands** (delete, restore, type change) refuse without ` + "`--yes`" + `. Only pass ` + "`--yes`" + ` when the person
   you are working for asked for that exact action.
8. Before finishing, run ` + "`docklite containers list`" + ` and ` + "`docklite databases list`" + ` and check what you made appears there.
`

// EnsureSitesGuide writes CLAUDE.md and AGENTS.md into the sites folder. A file that exists without DockLite's marker is
// someone's own and is left alone; one with the marker is refreshed when the text changes.
func (h *Handlers) EnsureSitesGuide() {
	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		path := filepath.Join(siteBaseDir, name)
		if existing, err := os.ReadFile(path); err == nil {
			if !strings.HasPrefix(string(existing), sitesGuideMarker) || string(existing) == sitesGuide {
				continue
			}
		} else if !os.IsNotExist(err) {
			continue
		}
		if _, err := os.Stat(siteBaseDir); err != nil {
			return // no sites folder on this machine (a dev checkout)
		}
		if err := os.WriteFile(path, []byte(sitesGuide), 0o664); err != nil {
			log.Printf("sites guide: could not write %s: %v", path, err)
		}
	}
}
