package main

import (
	"encoding/json"
	"flag"
	"strconv"
)

func init() {
	// ---- automatic certificate renewal ----
	register(command{
		Path: []string{"ssl", "auto-renew"}, Summary: "Show or change automatic certificate renewal (all/none, or one domain on/off)",
		Usage: "ssl auto-renew [all|none|run|<domain> on|off]", AdminOnly: true,
		Examples: []string{"docklite ssl auto-renew", "docklite ssl auto-renew none", "docklite ssl auto-renew example.com off", "docklite ssl auto-renew run"},
		Run: func(a *app, args []string) error {
			usage := usageError("usage: docklite ssl auto-renew [all|none|run|<domain> on|off]")
			switch {
			case len(args) == 0:
				return showAutoRenew(a)
			case len(args) == 1 && (args[0] == "all" || args[0] == "none"):
				data, err := a.put("/api/ssl/autorenew", map[string]any{"enabled": args[0] == "all"})
				if err != nil {
					return err
				}
				a.emitValue(json.RawMessage(data), func() {
					if args[0] == "all" {
						a.say("Automatic renewal is on for every certificate (a domain you switched off yourself stays off).")
					} else {
						a.say("Automatic renewal is off everywhere. Turn it back on with: docklite ssl auto-renew all")
					}
				})
				return nil
			case len(args) == 1 && args[0] == "run":
				data, err := a.post("/api/ssl/autorenew", map[string]any{})
				if err != nil {
					return err
				}
				a.emitValue(json.RawMessage(data), func() {
					var res struct {
						Renewed []string `json:"renewed"`
						Failed  []string `json:"failed"`
					}
					_ = json.Unmarshal(data, &res)
					a.say("Renewed: %d. Failed: %d.", len(res.Renewed), len(res.Failed))
					for _, d := range res.Failed {
						a.say("  failed: %s (see `docklite ssl auto-renew` for why)", d)
					}
				})
				return nil
			case len(args) == 2 && (args[1] == "on" || args[1] == "off"):
				data, err := a.put("/api/ssl/autorenew", map[string]any{"domain": args[0], "enabled": args[1] == "on"})
				if err != nil {
					return err
				}
				a.emitValue(json.RawMessage(data), func() { a.say("Automatic renewal for %s is %s.", args[0], args[1]) })
				return nil
			}
			return usage
		},
	})

	// ---- the dashboard's own domain ----
	register(command{
		Path: []string{"panel", "show"}, Summary: "Show the domain the dashboard is reached at, and the address that works without it",
		Usage: "panel show",
		Run: func(a *app, args []string) error {
			data, err := a.get("/api/panel-domain")
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() { printPanel(a, data) })
			return nil
		},
	})
	register(command{
		Path: []string{"panel", "set"}, Summary: "Set the dashboard's domain (use --enable to switch it on)",
		Usage: "panel set <domain> [--enable]", AdminOnly: true,
		Examples: []string{"docklite panel set smoll.docklite.net --enable"},
		Run: func(a *app, args []string) error {
			var enable bool
			pos, err := a.flags("panel set", args, func(fs *flag.FlagSet) { fs.BoolVar(&enable, "enable", false, "") })
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite panel set <domain> [--enable]")
			}
			payload := map[string]any{"domain": pos[0]}
			if enable {
				payload["enabled"] = true
			}
			data, err := a.put("/api/panel-domain", payload)
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() { printPanel(a, data) })
			return nil
		},
	})
	for _, on := range []bool{true, false} {
		on := on
		name, summary := "disable", "Turn the dashboard's domain off (the dashboard is then at the server's address and port)"
		if on {
			name, summary = "enable", "Turn the dashboard's domain on"
		}
		register(command{
			Path: []string{"panel", name}, Summary: summary, Usage: "panel " + name, AdminOnly: true,
			Run: func(a *app, args []string) error {
				data, err := a.put("/api/panel-domain", map[string]any{"enabled": on})
				if err != nil {
					return err
				}
				a.emitValue(json.RawMessage(data), func() { printPanel(a, data) })
				return nil
			},
		})
	}
}

func printPanel(a *app, data []byte) {
	var st struct {
		Domain      string `json:"domain"`
		Enabled     bool   `json:"enabled"`
		HTTPS       bool   `json:"https"`
		URL         string `json:"url"`
		FallbackURL string `json:"fallback_url"`
	}
	_ = json.Unmarshal(data, &st)
	switch {
	case st.Domain == "":
		a.say("No domain is set. The dashboard is at %s", st.FallbackURL)
	case st.Enabled:
		a.say("%s is on (%s). Open %s", st.Domain, map[bool]string{true: "HTTPS", false: "HTTP only; `docklite ssl issue " + st.Domain + "` adds HTTPS"}[st.HTTPS], st.URL)
		a.say("Without the domain it would be at %s", st.FallbackURL)
	default:
		a.say("%s is set but switched off. The dashboard is at %s", st.Domain, st.FallbackURL)
	}
}

// showAutoRenew prints the master switch and, per certificate, its switch, days left and how the last renewal went.
func showAutoRenew(a *app) error {
	data, err := a.get("/api/ssl/status")
	if err != nil {
		return err
	}
	if a.opts.JSON {
		a.emit(data)
		return nil
	}
	var st struct {
		AllCerts []struct {
			Domain          string `json:"domain"`
			DaysUntilExpiry *int   `json:"daysUntilExpiry"`
			AutoRenew       bool   `json:"autoRenew"`
			Renewal         *struct {
				At    string `json:"at"`
				OK    bool   `json:"ok"`
				Error string `json:"error"`
			} `json:"renewal"`
		} `json:"allCerts"`
		Meta struct {
			All bool `json:"autoRenewAll"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return err
	}
	if st.Meta.All {
		a.say("Automatic renewal: all (each certificate has its own switch).")
	} else {
		a.say("Automatic renewal: none. Turn it on with: docklite ssl auto-renew all")
	}
	rows := [][]string{}
	for _, c := range st.AllCerts {
		days, state, last := "-", "off", "-"
		if c.DaysUntilExpiry != nil {
			days = strconv.Itoa(*c.DaysUntilExpiry)
		}
		if c.AutoRenew {
			state = "on"
		}
		if c.Renewal != nil {
			if c.Renewal.OK {
				last = "renewed " + c.Renewal.At
			} else {
				last = "FAILED " + c.Renewal.At + ": " + c.Renewal.Error
			}
		}
		rows = append(rows, []string{c.Domain, state, days, last})
	}
	if len(rows) > 0 {
		a.table([]string{"DOMAIN", "AUTO-RENEW", "DAYS LEFT", "LAST RENEWAL"}, rows)
	}
	return nil
}

func init() {
	register(command{
		Path: []string{"sites", "forward"}, Summary: "Send another domain's visitors to a site (redirect, or proxy to keep the new address); both go in a linked group",
		Usage: "sites forward <new-domain> --to <site-domain> [--mode redirect|proxy] [--no-www]", AdminOnly: true,
		Examples: []string{"docklite sites forward old-name.com --to example.com", "docklite sites forward www2.example.org --to example.com --mode proxy"},
		Run: func(a *app, args []string) error {
			var to, mode string
			var noWWW bool
			pos, err := a.flags("sites forward", args, func(fs *flag.FlagSet) {
				fs.StringVar(&to, "to", "", "")
				fs.StringVar(&mode, "mode", "redirect", "")
				fs.BoolVar(&noWWW, "no-www", false, "")
			})
			if err != nil {
				return err
			}
			if len(pos) != 1 || to == "" {
				return usageError("usage: docklite sites forward <new-domain> --to <site-domain> [--mode redirect|proxy] [--no-www]")
			}
			c, err := a.resolveContainer(to)
			if err != nil {
				return err
			}
			data, err := a.post("/api/containers/"+c.ID+"/forward", map[string]any{"domain": pos[0], "mode": mode, "include_www": !noWWW})
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() {
				var res struct {
					Group   string `json:"group"`
					Warning string `json:"warning"`
				}
				_ = json.Unmarshal(data, &res)
				a.say("%s now %ss to %s.", pos[0], mode, to)
				if res.Group != "" {
					a.say("Both are in the linked group %q.", res.Group)
				}
				a.say("Point %s's DNS at this server; its own HTTPS certificate is requested once DNS is ready.", pos[0])
				if res.Warning != "" {
					a.say("Note: %s", res.Warning)
				}
			})
			return nil
		},
	})
	register(command{
		Path: []string{"sites", "forwards"}, Summary: "List the domains forwarded to a site", Usage: "sites forwards <site-domain>",
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite sites forwards <site-domain>")
			}
			c, err := a.resolveContainer(args[0])
			if err != nil {
				return err
			}
			data, err := a.get("/api/containers/" + c.ID + "/forward")
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() {
				var res struct {
					Forwards []struct{ Domain, Mode string } `json:"forwards"`
				}
				_ = json.Unmarshal(data, &res)
				if len(res.Forwards) == 0 {
					a.say("Nothing is forwarded to %s.", args[0])
					return
				}
				rows := [][]string{}
				for _, f := range res.Forwards {
					rows = append(rows, []string{f.Domain, f.Mode})
				}
				a.table([]string{"DOMAIN", "HOW"}, rows)
			})
			return nil
		},
	})
}
