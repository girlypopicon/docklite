package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"strings"
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
		Path: []string{"backups", "create"}, Summary: "Back up a site now (its files), kept on this server",
		Usage: "backups create <domain>", AdminOnly: true,
		Examples: []string{"docklite backups create glowrom.com"},
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite backups create <domain>")
			}
			domain := strings.ToLower(strings.TrimSpace(args[0]))
			data, err := a.post("/api/backups/export", map[string]any{"target_type": "site", "domain": domain, "delivery": "local"})
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			a.say("Backed up %s. See it with: docklite backups files", domain)
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
			}
			if err := json.Unmarshal(data, &r); err != nil {
				return err
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
		Usage: "backups restore <file> [--site <domain>] --yes", AdminOnly: true, Destructive: true,
		Examples: []string{"docklite backups restore sites/site-glowrom.com-20261011-030000.tar.gz --yes"},
		Run: func(a *app, args []string) error {
			var site string
			pos, err := a.flags("backups restore", args, func(fs *flag.FlagSet) { fs.StringVar(&site, "site", "", "") })
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite backups restore <file> [--site <domain>] --yes")
			}
			target := "the site this backup belongs to"
			if site != "" {
				target = site
			}
			if err := a.confirm(fmt.Sprintf("replace the files of %s with the backup %s (the current files are saved first)", target, pos[0])); err != nil {
				return err
			}
			data, err := a.post("/api/backups/restore", map[string]any{"file": pos[0], "domain": site})
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
				what := j.TargetType
				if j.TargetID != nil {
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
		Usage: "backups schedule (<domain> | --all) [--daily-at HH:MM | --weekly sun@HH:MM | --every-hours N] [--keep N]", AdminOnly: true,
		Examples: []string{"docklite backups schedule --all --daily-at 03:00 --keep 7", "docklite backups schedule glowrom.com --weekly sun@04:00 --keep 4"},
		Run: func(a *app, args []string) error {
			var all bool
			var dailyAt, weekly string
			var everyHours, keep int
			pos, err := a.flags("backups schedule", args, func(fs *flag.FlagSet) {
				fs.BoolVar(&all, "all", false, "")
				fs.StringVar(&dailyAt, "daily-at", "", "")
				fs.StringVar(&weekly, "weekly", "", "")
				fs.IntVar(&everyHours, "every-hours", 0, "")
				fs.IntVar(&keep, "keep", 0, "")
			})
			if err != nil {
				return err
			}
			if all == (len(pos) == 1) || len(pos) > 1 {
				return usageError("name one site, or use --all")
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
			dest, err := a.get("/api/backups/destinations")
			if err != nil {
				return err
			}
			var d struct {
				Destinations []struct {
					ID   int64  `json:"id"`
					Type string `json:"type"`
				} `json:"destinations"`
			}
			_ = json.Unmarshal(dest, &d)
			var destID int64
			for _, x := range d.Destinations {
				if x.Type == "local" {
					destID = x.ID
				}
			}
			if destID == 0 {
				return fmt.Errorf("no local backup destination exists yet; make one backup first (docklite backups create <domain>) or add one in the dashboard")
			}
			body := map[string]any{"destination_id": destID, "frequency": freq, "retention_days": 0, "enabled": 1}
			if all {
				body["target_type"] = "all-sites"
			} else {
				body["target_type"], body["domain"] = "site", strings.ToLower(pos[0])
			}
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
