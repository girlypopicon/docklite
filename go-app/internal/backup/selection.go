package backup

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"docklite-agent/internal/store"
)

// Selection is what goes into one .litpkg: the dashboard's list of sites with a "Site" and a "Databases" tick box on
// each row, plus databases that don't belong to any site. The same value is saved on a schedule, so "all sites" keeps
// meaning every site, including ones added after the schedule was made.
type Selection struct {
	Sites         []int64 `json:"sites,omitempty"`          // sites whose files are included
	SiteDatabases []int64 `json:"site_databases,omitempty"` // sites whose databases are included
	Databases     []int64 `json:"databases,omitempty"`      // single databases, by id
	AllSites      bool    `json:"all_sites,omitempty"`      // the files of every site
	AllDatabases  bool    `json:"all_databases,omitempty"`  // every database, whether it belongs to a site or not
}

func (s Selection) Empty() bool {
	return !s.AllSites && !s.AllDatabases && len(s.Sites) == 0 && len(s.SiteDatabases) == 0 && len(s.Databases) == 0
}

// ParseSelection reads a selection saved as JSON. An empty string is an empty selection.
func ParseSelection(raw string) (Selection, error) {
	var s Selection
	if strings.TrimSpace(raw) == "" {
		return s, nil
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return s, fmt.Errorf("the saved selection is unreadable: %w", err)
	}
	return s, nil
}

func (s Selection) JSON() string {
	b, _ := json.Marshal(s)
	return string(b)
}

// SelectionForJob is what a schedule backs up. Schedules made before packages have no selection, only a target type,
// which maps onto one directly.
func SelectionForJob(job store.BackupJob) (Selection, error) {
	if job.Selection != "" {
		return ParseSelection(job.Selection)
	}
	var s Selection
	switch job.TargetType {
	case "site":
		if job.TargetID != nil {
			s.Sites = []int64{*job.TargetID}
		}
	case "database":
		if job.TargetID != nil {
			s.Databases = []int64{*job.TargetID}
		}
	case "all-sites":
		s.AllSites = true
	case "all-databases":
		s.AllDatabases = true
	}
	return s, nil
}

// LegacyTarget is the closest old-style target_type/target_id for a selection. The tables only allow those values, and
// a DockLite rolled back to before packages reads them; it then backs up roughly the same thing the old way.
func (s Selection) LegacyTarget(jobTable bool) (string, int64) {
	switch {
	case s.AllSites && jobTable:
		return "all-sites", 0
	case s.AllDatabases && len(s.Sites) == 0 && jobTable:
		return "all-databases", 0
	case len(s.Sites) > 0:
		if len(s.Sites) == 1 {
			return "site", s.Sites[0]
		}
		return "site", 0
	case len(s.SiteDatabases) > 0 && len(s.Databases) == 0:
		return "site", s.SiteDatabases[0]
	case len(s.Databases) == 1 && len(s.SiteDatabases) == 0:
		return "database", s.Databases[0]
	case s.AllSites:
		return "site", 0
	}
	return "database", 0
}

// Resolved is a selection turned into the actual sites and databases, each once.
type Resolved struct {
	Sites     []store.SiteRecord
	Databases []ResolvedDatabase
	Missing   []string // things that were selected but no longer exist
}

type ResolvedDatabase struct {
	store.DatabaseRecord
	SiteDomain string // the site it belongs to, if any
}

// Resolve looks up what a selection means right now.
func (s Selection) Resolve(st *store.SQLiteStore) (*Resolved, error) {
	sites, err := st.ListSites()
	if err != nil {
		return nil, err
	}
	dbs, err := st.ListDatabases()
	if err != nil {
		return nil, err
	}
	links, err := st.DatabaseSiteLinks()
	if err != nil {
		return nil, err
	}
	siteByID := map[int64]store.SiteRecord{}
	for _, site := range sites {
		siteByID[site.ID] = site
	}
	dbByID := map[int64]store.DatabaseRecord{}
	for _, db := range dbs {
		dbByID[db.ID] = db
	}

	out := &Resolved{}
	seenSite := map[int64]bool{}
	addSite := func(site store.SiteRecord) {
		if !seenSite[site.ID] {
			seenSite[site.ID] = true
			out.Sites = append(out.Sites, site)
		}
	}
	seenDB := map[int64]bool{}
	addDB := func(db store.DatabaseRecord) {
		if seenDB[db.ID] {
			return
		}
		seenDB[db.ID] = true
		rd := ResolvedDatabase{DatabaseRecord: db}
		if siteID, ok := links[db.ID]; ok {
			rd.SiteDomain = siteByID[siteID].Domain
		}
		out.Databases = append(out.Databases, rd)
	}

	if s.AllSites {
		for _, site := range sites {
			addSite(site)
		}
	}
	for _, id := range s.Sites {
		if site, ok := siteByID[id]; ok {
			addSite(site)
		} else {
			out.Missing = append(out.Missing, fmt.Sprintf("site #%d (deleted)", id))
		}
	}
	if s.AllDatabases {
		for _, db := range dbs {
			addDB(db)
		}
	}
	for _, siteID := range s.SiteDatabases {
		if _, ok := siteByID[siteID]; !ok {
			out.Missing = append(out.Missing, fmt.Sprintf("databases of site #%d (site deleted)", siteID))
			continue
		}
		for dbID, linked := range links {
			if linked == siteID {
				if db, ok := dbByID[dbID]; ok {
					addDB(db)
				}
			}
		}
	}
	for _, id := range s.Databases {
		if db, ok := dbByID[id]; ok {
			addDB(db)
		} else {
			out.Missing = append(out.Missing, fmt.Sprintf("database #%d (deleted)", id))
		}
	}
	sort.Slice(out.Sites, func(i, j int) bool { return out.Sites[i].Domain < out.Sites[j].Domain })
	sort.Slice(out.Databases, func(i, j int) bool { return out.Databases[i].Name < out.Databases[j].Name })
	return out, nil
}

// Describe says in a few words what a package holds, for the backups list: "example.com + 1 database",
// "All sites + all databases", "3 sites, 2 databases".
func (s Selection) Describe(r *Resolved) string {
	var parts []string
	switch {
	case s.AllSites:
		parts = append(parts, "All sites")
	case len(r.Sites) == 1:
		parts = append(parts, r.Sites[0].Domain)
	case len(r.Sites) > 1:
		parts = append(parts, plural(len(r.Sites), "site"))
	}
	switch {
	case s.AllDatabases:
		parts = append(parts, "all databases")
	case len(r.Databases) == 1 && len(r.Sites) == 0:
		parts = append(parts, r.Databases[0].Name)
	case len(r.Databases) > 0:
		parts = append(parts, plural(len(r.Databases), "database"))
	}
	if len(parts) == 0 {
		return "Nothing"
	}
	if parts[0] == "all databases" {
		parts[0] = "All databases"
	}
	return strings.Join(parts, " + ")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
