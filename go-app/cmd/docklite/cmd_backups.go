package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func queryEscape(s string) string { return url.QueryEscape(s) }

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

func init() {
	register(command{
		Path: []string{"backups", "files"}, Summary: "The backup files on this server (site and database backups), newest first",
		Usage: "backups files", AdminOnly: true,
		Run: func(a *app, args []string) error {
			data, err := a.get("/api/backups/local")
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var resp struct {
				Path  string `json:"path"`
				Files []struct {
					Name       string `json:"name"`
					Size       int64  `json:"size"`
					ModifiedAt string `json:"modified_at"`
				} `json:"files"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return err
			}
			if len(resp.Files) == 0 {
				a.say("No backup files yet in %s. Make one with: docklite backups create <domain>", resp.Path)
				return nil
			}
			rows := [][]string{}
			for _, f := range resp.Files {
				rows = append(rows, []string{f.Name, humanSize(f.Size), f.ModifiedAt})
			}
			a.table([]string{"FILE (use this name with inspect/restore)", "SIZE", "MADE"}, rows)
			return nil
		},
	})
	register(command{
		Path: []string{"backups", "create"}, Summary: "Back up sites and/or databases now, all into one .litpkg kept on this server",
		Usage: "backups create [<domain>...] [--with-databases] [--database <name>,...] [--all-sites] [--all-databases] [--all] [--notes TEXT]", AdminOnly: true,
		Examples: []string{
			"docklite backups create glowrom.com",
			"docklite backups create glowrom.com --with-databases",
			"docklite backups create --database shopdb",
			"docklite backups create --all",
		},
		Run: func(a *app, args []string) error {
			var pick selectionFlags
			var notes string
			pos, err := a.flags("backups create", args, func(fs *flag.FlagSet) {
				pick.define(fs)
				fs.StringVar(&notes, "notes", "", "")
			})
			if err != nil {
				return err
			}
			sel, err := pick.selection(a, pos)
			if err != nil {
				return err
			}
			data, err := a.post("/api/backups/export", map[string]any{"selection": sel, "delivery": "local", "notes": notes})
			if err != nil {
				return err
			}
			var started struct {
				BackupID int64 `json:"backup_id"`
			}
			if err := json.Unmarshal(data, &started); err != nil {
				return err
			}
			done, raw, err := a.followBackup(started.BackupID)
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(raw)
				return nil
			}
			if done.Status != "success" {
				return fmt.Errorf("the backup failed: %s", done.Error)
			}
			a.say("Backed up into %s (%s). See it with: docklite backups files", done.Location, humanSize(done.Size))
			for _, w := range done.Warnings {
				a.say("Warning: %s", w)
			}
			return nil
		},
	})
	register(command{
		Path: []string{"backups", "inspect"}, Summary: "Look inside a backup without changing anything: which site, how many files, whether it is intact",
		Usage: "backups inspect <file>", AdminOnly: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite backups inspect <file>   (names come from: docklite backups files)")
			}
			data, err := a.get("/api/backups/inspect?file=" + queryEscape(args[0]))
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var r struct {
				Domain   string   `json:"domain"`
				Problem  string   `json:"problem"`
				Files    int      `json:"files"`
				Bytes    int64    `json:"bytes"`
				Verified bool     `json:"verified"`
				Skipped  []string `json:"skipped"`
				// a .litpkg
				Package     bool   `json:"package"`
				Description string `json:"description"`
				Sites       []struct {
					Domain  string `json:"domain"`
					Files   int    `json:"files"`
					Problem string `json:"problem"`
				} `json:"sites"`
				Databases []struct {
					Name   string `json:"name"`
					Engine string `json:"engine"`
					File   string `json:"file"`
					Bytes  int64  `json:"bytes"`
				} `json:"databases"`
				Problems []string `json:"problems"`
			}
			if err := json.Unmarshal(data, &r); err != nil {
				return err
			}
			if r.Package {
				a.say("Package: %s (%s unpacked)\nChecksum matches what was recorded: %v", r.Description, humanSize(r.Bytes), r.Verified)
				for _, s := range r.Sites {
					note := ""
					if s.Problem != "" {
						note = "  (can't restore here: " + s.Problem + ")"
					}
					a.say("Site: %s, %d files%s", s.Domain, s.Files, note)
				}
				for _, d := range r.Databases {
					a.say("Database: %s (%s, %s) at %s", d.Name, d.Engine, humanSize(d.Bytes), d.File)
				}
				for _, p := range r.Problems {
					a.say("Left out when it was made: %s", p)
				}
				return nil
			}
			site := r.Domain
			if site == "" {
				site = "(unknown: " + r.Problem + ")"
			}
			a.say("Site: %s\nFiles: %d (%s unpacked)\nChecksum matches what was recorded: %v", site, r.Files, humanSize(r.Bytes), r.Verified)
			for _, s := range r.Skipped {
				a.say("Would be left out: %s", s)
			}
			return nil
		},
	})
	register(command{
		Path: []string{"backups", "restore"}, Summary: "Put a site back the way a backup had it (what is there now is saved first)",
		Usage: "backups restore <file> [--from <domain>] [--site <domain>] --yes", AdminOnly: true, Destructive: true,
		Examples: []string{
			"docklite backups restore packages/glowrom.com-20261011-030000.litpkg --yes",
			"docklite backups restore packages/everything-20261011-030000.litpkg --from glowrom.com --yes",
		},
		Run: func(a *app, args []string) error {
			var site, from string
			pos, err := a.flags("backups restore", args, func(fs *flag.FlagSet) {
				fs.StringVar(&site, "site", "", "")
				fs.StringVar(&from, "from", "", "")
			})
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite backups restore <file> [--from <domain>] [--site <domain>] --yes")
			}
			target := "the site this backup belongs to"
			if from != "" {
				target = from
			}
			if site != "" {
				target = site
			}
			if err := a.confirm(fmt.Sprintf("replace the files of %s with the backup %s (the current files are saved first)", target, pos[0])); err != nil {
				return err
			}
			data, err := a.post("/api/backups/restore", map[string]any{"file": pos[0], "domain": site, "from_site": from})
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var r struct {
				Domain       string   `json:"domain"`
				Files        int      `json:"files"`
				SafetyBackup string   `json:"safety_backup"`
				Skipped      []string `json:"skipped"`
			}
			_ = json.Unmarshal(data, &r)
			a.say("Restored %s (%d files).", r.Domain, r.Files)
			if r.SafetyBackup != "" {
				a.say("What was there before is saved at %s", r.SafetyBackup)
			}
			for _, s := range r.Skipped {
				a.say("Left out: %s", s)
			}
			return nil
		},
	})
	register(command{
		Path: []string{"backups", "schedules"}, Summary: "The automatic backup schedules, in plain words",
		Usage: "backups schedules", AdminOnly: true,
		Run: func(a *app, args []string) error {
			data, err := a.get("/api/backups/jobs")
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var resp struct {
				Jobs []struct {
					ID           int64   `json:"id"`
					TargetType   string  `json:"target_type"`
					TargetID     *int64  `json:"target_id"`
					What         string  `json:"what"`
					Enabled      int     `json:"enabled"`
					ScheduleText string  `json:"schedule_text"`
					LastRunAt    *string `json:"last_run_at"`
					NextRunAt    *string `json:"next_run_at"`
				} `json:"jobs"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return err
			}
			if len(resp.Jobs) == 0 {
				a.say("No automatic backups yet. Set one up with: docklite backups schedule --all --daily-at 03:00 --keep 7")
				return nil
			}
			rows := [][]string{}
			for _, j := range resp.Jobs {
				what := j.What
				if what == "" && j.TargetID != nil {
					what = fmt.Sprintf("%s #%d", j.TargetType, *j.TargetID)
				}
				state := "on"
				if j.Enabled == 0 {
					state = "paused"
				}
				last, next := "never", "-"
				if j.LastRunAt != nil && *j.LastRunAt != "" {
					last = *j.LastRunAt
				}
				if j.NextRunAt != nil && *j.NextRunAt != "" {
					next = *j.NextRunAt
				}
				rows = append(rows, []string{fmt.Sprint(j.ID), what, j.ScheduleText, state, last, next})
			}
			a.table([]string{"ID", "BACKS UP", "WHEN", "STATE", "LAST RUN", "NEXT RUN"}, rows)
			return nil
		},
	})
	register(command{
		Path: []string{"backups", "schedule"}, Summary: "Back up automatically: every day (or week) at a time you choose, keeping the newest N",
		Usage: "backups schedule [<domain>...] [--with-databases] [--database <name>,...] [--all-sites] [--all-databases] [--all] [--daily-at HH:MM | --weekly sun@HH:MM | --every-hours N] [--keep N]", AdminOnly: true,
		Examples: []string{"docklite backups schedule --all --daily-at 03:00 --keep 7", "docklite backups schedule glowrom.com --with-databases --weekly sun@04:00 --keep 4"},
		Run: func(a *app, args []string) error {
			var pick selectionFlags
			var dailyAt, weekly string
			var everyHours, keep int
			pos, err := a.flags("backups schedule", args, func(fs *flag.FlagSet) {
				pick.define(fs)
				fs.StringVar(&dailyAt, "daily-at", "", "")
				fs.StringVar(&weekly, "weekly", "", "")
				fs.IntVar(&everyHours, "every-hours", 0, "")
				fs.IntVar(&keep, "keep", 0, "")
			})
			if err != nil {
				return err
			}
			sel, err := pick.selection(a, pos)
			if err != nil {
				return err
			}
			freq := ""
			switch {
			case dailyAt != "" && weekly == "" && everyHours == 0:
				freq = "daily@" + dailyAt
			case weekly != "" && dailyAt == "" && everyHours == 0:
				freq = "weekly@" + weekly
			case everyHours > 0 && dailyAt == "" && weekly == "":
				freq = fmt.Sprint(everyHours)
			default:
				return usageError("choose one of --daily-at HH:MM, --weekly day@HH:MM, or --every-hours N")
			}
			if keep > 0 {
				freq += fmt.Sprintf(";keep=%d", keep)
			}
			// destination_id 0: the agent uses (and if need be creates) its local destination
			body := map[string]any{"selection": sel, "frequency": freq, "retention_days": 0, "enabled": 1}
			if _, err := a.post("/api/backups/jobs", body); err != nil {
				return err
			}
			a.say("Scheduled. See it with: docklite backups schedules")
			return nil
		},
	})
	register(command{
		Path: []string{"backups", "unschedule"}, Summary: "Stop and remove an automatic backup schedule (the backups already made are kept)",
		Usage: "backups unschedule <id>", AdminOnly: true, Destructive: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite backups unschedule <id>   (ids come from: docklite backups schedules)")
			}
			if err := a.confirm("remove backup schedule " + args[0]); err != nil {
				return err
			}
			if _, err := a.del("/api/backups/jobs?id=" + queryEscape(args[0])); err != nil {
				return err
			}
			a.say("schedule %s removed", args[0])
			return nil
		},
	})
}

// selectionFlags are the ways to say what goes into a package, shared by backups create and backups schedule: the
// same choices as the tick boxes in the dashboard.
type selectionFlags struct {
	withDatabases, allSites, allDatabases, all bool
	databases                                  string
}

func (p *selectionFlags) define(fs *flag.FlagSet) {
	fs.BoolVar(&p.withDatabases, "with-databases", false, "")
	fs.StringVar(&p.databases, "database", "", "")
	fs.BoolVar(&p.allSites, "all-sites", false, "")
	fs.BoolVar(&p.allDatabases, "all-databases", false, "")
	fs.BoolVar(&p.all, "all", false, "")
}

// selection looks the named sites and databases up and builds the selection the agent takes.
func (p *selectionFlags) selection(a *app, domains []string) (map[string]any, error) {
	var dbNames []string
	for _, n := range strings.Split(p.databases, ",") {
		if n = strings.TrimSpace(n); n != "" {
			dbNames = append(dbNames, n)
		}
	}
	if len(domains) == 0 && len(dbNames) == 0 && !p.allSites && !p.allDatabases && !p.all {
		return nil, usageError("say what to back up: one or more domains, --database <name>, --all-sites, --all-databases or --all")
	}
	data, err := a.get("/api/backups/targets")
	if err != nil {
		return nil, err
	}
	var t struct {
		Sites []struct {
			ID     int64  `json:"id"`
			Domain string `json:"domain"`
		} `json:"sites"`
		Databases []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"databases"`
	}
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	sel := map[string]any{}
	var siteIDs, dbIDs []int64
	for _, d := range domains {
		found := false
		for _, s := range t.Sites {
			if strings.EqualFold(s.Domain, d) {
				siteIDs, found = append(siteIDs, s.ID), true
			}
		}
		if !found {
			return nil, fmt.Errorf("there is no site called %s (see: docklite sites list)", d)
		}
	}
	for _, n := range dbNames {
		found := false
		for _, db := range t.Databases {
			if db.Name == n {
				dbIDs, found = append(dbIDs, db.ID), true
			}
		}
		if !found {
			return nil, fmt.Errorf("there is no database called %s (see: docklite databases list)", n)
		}
	}
	if len(siteIDs) > 0 {
		sel["sites"] = siteIDs
		if p.withDatabases {
			sel["site_databases"] = siteIDs
		}
	}
	if len(dbIDs) > 0 {
		sel["databases"] = dbIDs
	}
	if p.all || p.allSites {
		sel["all_sites"] = true
	}
	if p.all || p.allDatabases || (p.allSites && p.withDatabases) {
		sel["all_databases"] = true
	}
	return sel, nil
}

type backupProgress struct {
	Status   string   `json:"status"`
	Phase    string   `json:"phase"`
	Error    string   `json:"error"`
	Location string   `json:"location"`
	Size     int64    `json:"size"`
	Warnings []string `json:"warnings"`
}

// followBackup waits for a backup running on the agent to finish, asking how it is going every second, so a big backup
// isn't cut off by the request timeout.
func (a *app) followBackup(id int64) (*backupProgress, []byte, error) {
	last := ""
	for {
		time.Sleep(time.Second)
		data, err := a.get(fmt.Sprintf("/api/backups/progress?id=%d", id))
		if err != nil {
			return nil, nil, err
		}
		var p backupProgress
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, nil, err
		}
		if p.Status != "running" {
			return &p, data, nil
		}
		if p.Phase != last && !a.opts.JSON {
			a.say("%s…", p.Phase)
			last = p.Phase
		}
	}
}
