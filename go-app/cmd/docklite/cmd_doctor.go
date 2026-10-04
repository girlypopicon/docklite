package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warn, fail, skip
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

func init() {
	register(command{
		Path: []string{"doctor"}, Summary: "Check that DockLite and its surroundings are healthy, and say how to fix what isn't",
		Usage: "doctor", Examples: []string{"docklite doctor", "docklite doctor --json"},
		Run: runDoctor,
	})

	register(command{
		Path: []string{"access", "list"}, Summary: "Which server (OS) users have admin shell access to DockLite",
		Usage: "access list", AdminOnly: true,
		Run: func(a *app, args []string) error {
			data, err := a.get("/api/system/shell-access")
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var resp struct {
				Members []string `json:"members"`
			}
			_ = json.Unmarshal(data, &resp)
			if len(resp.Members) == 0 {
				fmt.Fprintln(a.out, "no one besides DockLite itself has admin shell access")
				return nil
			}
			for _, m := range resp.Members {
				fmt.Fprintln(a.out, m)
			}
			return nil
		},
	})
	register(command{
		Path: []string{"access", "grant"}, Summary: "Give a server user admin shell access: they can then run docklite here without logging in",
		Usage: "access grant <os-username>", AdminOnly: true, Destructive: true,
		Examples: []string{"docklite access grant alice --yes"},
		Run: func(a *app, args []string) error {
			return changeAccess(a, args, "grant",
				"give %s admin control of DockLite from the shell (they will be able to read the server's master token)")
		},
	})
	register(command{
		Path: []string{"access", "revoke"}, Summary: "Remove a server user's admin shell access",
		Usage: "access revoke <os-username>", AdminOnly: true, Destructive: true,
		Run: func(a *app, args []string) error {
			return changeAccess(a, args, "revoke", "remove %s's admin shell access to DockLite")
		},
	})
}

func changeAccess(a *app, args []string, action, what string) error {
	if len(args) != 1 {
		return usageError("usage: docklite access %s <os-username>", action)
	}
	user := args[0]
	if err := a.confirm(fmt.Sprintf(what, user)); err != nil {
		return err
	}
	data, err := a.post("/api/system/shell-access", map[string]any{"username": user, "action": action})
	if err != nil {
		return err
	}
	a.emitValue(json.RawMessage(data), func() {
		if action == "grant" {
			a.say("%s now has admin shell access. They must log out and back in once for it to take effect.", user)
		} else {
			a.say("%s no longer has admin shell access. Any shell they already have open keeps it until it closes.", user)
		}
	})
	return nil
}

func runDoctor(a *app, args []string) error {
	var checks []check
	add := func(name, status, detail, fix string) {
		checks = append(checks, check{Name: name, Status: status, Detail: detail, Fix: fix})
	}

	// 1. Credentials and reachability.
	if a.client.Token == "" {
		add("credentials", "fail", "no token: "+a.credSource, "docklite login   (or use --token / DOCKLITE_TOKEN)")
	} else {
		add("credentials", "ok", a.credSource, "")
	}
	if _, err := a.get("/api/health"); err != nil {
		add("agent", "fail", err.Error(), "On the server: docklite start-all")
		return finishDoctor(a, checks)
	}
	add("agent", "ok", "answering at "+a.client.BaseURL, "")

	var admin bool
	if data, err := a.get("/api/auth/me"); err != nil {
		add("login", "fail", err.Error(), "docklite login")
		return finishDoctor(a, checks)
	} else {
		var me struct {
			User struct {
				Username string `json:"username"`
				Role     string `json:"role"`
				IsAdmin  bool   `json:"isAdmin"`
			} `json:"user"`
		}
		_ = json.Unmarshal(data, &me)
		admin = me.User.IsAdmin
		add("login", "ok", fmt.Sprintf("%s (%s)", me.User.Username, me.User.Role), "")
	}

	skipAdmin := func(name string) { add(name, "skip", "needs an admin account", "") }

	// 2. Server-level checks (admin only).
	if !admin {
		for _, n := range []string{"services", "nginx", "certificates", "disk", "updates"} {
			skipAdmin(n)
		}
		return finishDoctor(a, checks)
	}

	if data, err := a.get("/api/server/services"); err != nil {
		add("services", "fail", err.Error(), "")
	} else {
		var s struct {
			Docker  struct{ Status string }           `json:"docker"`
			Proxy   *struct{ Name, Status string }    `json:"proxy"`
			Traefik *struct{ Status, Warning string } `json:"traefik"`
		}
		_ = json.Unmarshal(data, &s)
		switch {
		case s.Docker.Status != "running":
			add("docker", "fail", "Docker is "+s.Docker.Status, "sudo systemctl start docker")
		default:
			add("docker", "ok", "running", "")
		}
		if s.Proxy == nil {
			add("proxy", "fail", "no web proxy found", "Install nginx, or start Traefik")
		} else if s.Proxy.Status != "active" && s.Proxy.Status != "running" {
			add("proxy", "fail", fmt.Sprintf("%s is %s", s.Proxy.Name, s.Proxy.Status), "sudo systemctl start nginx")
		} else {
			add("proxy", "ok", s.Proxy.Name+" is running", "")
		}
		if s.Traefik != nil && s.Traefik.Warning != "" {
			add("traefik", "warn", s.Traefik.Warning, "docklite server service traefik stop --yes")
		}
	}

	if data, err := a.post("/api/nginx/test", nil); err != nil {
		add("nginx config", "fail", err.Error(), "")
	} else {
		var r struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &r)
		if r.OK {
			add("nginx config", "ok", "valid", "")
		} else {
			add("nginx config", "fail", strings.TrimSpace(r.Error), "Fix the config, then: docklite nginx test")
		}
	}

	if data, err := a.get("/api/ssl/status"); err != nil {
		add("certificates", "fail", err.Error(), "")
	} else {
		var r struct {
			AllCerts []struct {
				Domain          string `json:"domain"`
				Status          string `json:"status"`
				DaysUntilExpiry *int   `json:"daysUntilExpiry"`
			} `json:"allCerts"`
		}
		_ = json.Unmarshal(data, &r)
		var problems []string
		for _, c := range r.AllCerts {
			if c.Status == "expired" {
				problems = append(problems, c.Domain+" (expired)")
			} else if c.DaysUntilExpiry != nil && *c.DaysUntilExpiry < 14 {
				problems = append(problems, fmt.Sprintf("%s (%d days left)", c.Domain, *c.DaysUntilExpiry))
			}
		}
		if len(problems) > 0 {
			add("certificates", "warn", strings.Join(problems, ", "), "docklite ssl renew <domain>   or   docklite ssl delete <domain> --yes  for ones you no longer use")
		} else {
			add("certificates", "ok", fmt.Sprintf("%d certificate(s), none expiring soon", len(r.AllCerts)), "")
		}
	}

	if data, err := a.get("/api/server/overview"); err != nil {
		add("disk", "fail", err.Error(), "")
	} else {
		var r struct {
			Disk struct {
				Total uint64 `json:"total"`
				Free  uint64 `json:"free"`
			} `json:"disk"`
		}
		_ = json.Unmarshal(data, &r)
		if r.Disk.Total > 0 {
			used := 100 - int(float64(r.Disk.Free)/float64(r.Disk.Total)*100)
			if used >= 90 {
				add("disk", "warn", fmt.Sprintf("%d%% full", used), "docklite server storage   (then prune unused images)")
			} else {
				add("disk", "ok", fmt.Sprintf("%d%% used", used), "")
			}
		}
	}

	if data, err := a.get("/api/server/updates"); err == nil {
		var r struct {
			Pending  int  `json:"pendingUpdates"`
			Security int  `json:"securityUpdates"`
			Reboot   bool `json:"rebootRequired"`
		}
		_ = json.Unmarshal(data, &r)
		switch {
		case r.Security > 0:
			add("updates", "warn", fmt.Sprintf("%d security update(s) waiting", r.Security), "sudo apt update && sudo apt upgrade")
		case r.Reboot:
			add("updates", "warn", "a reboot is required", "sudo reboot (when it's a good time)")
		default:
			add("updates", "ok", fmt.Sprintf("%d pending, none for security", r.Pending), "")
		}
	}

	// 3. Only meaningful on the server itself.
	if _, err := os.Stat("/opt/docklite"); err == nil {
		if _, err := os.Stat("/usr/local/sbin/docklite-helper"); err != nil {
			add("root helper", "fail", "/usr/local/sbin/docklite-helper is missing", "sudo bash install.sh   (re-run the installer)")
		} else {
			add("root helper", "ok", "installed", "")
		}
	}
	return finishDoctor(a, checks)
}

func finishDoctor(a *app, checks []check) error {
	var ok, warn, fail int
	for _, c := range checks {
		switch c.Status {
		case "ok":
			ok++
		case "warn":
			warn++
		case "fail":
			fail++
		}
	}
	if a.opts.JSON {
		out, _ := json.Marshal(map[string]any{"ok": fail == 0, "checks": checks, "okCount": ok, "warnCount": warn, "failCount": fail})
		fmt.Fprintln(a.out, string(out))
	} else {
		marks := map[string]string{"ok": "✓", "warn": "!", "fail": "✗", "skip": "-"}
		for _, c := range checks {
			fmt.Fprintf(a.out, " %s  %-14s %s\n", marks[c.Status], c.Name, c.Detail)
			if c.Fix != "" && c.Status != "ok" {
				fmt.Fprintf(a.out, "    fix: %s\n", c.Fix)
			}
		}
		fmt.Fprintf(a.out, "\n%d ok, %d warning(s), %d problem(s)\n", ok, warn, fail)
	}
	if fail > 0 {
		return &exitError{code: exitFailure, msg: fmt.Sprintf("%d problem(s) found", fail)}
	}
	return nil
}
