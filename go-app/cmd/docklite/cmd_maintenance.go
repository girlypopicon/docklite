package main

import (
	"encoding/json"
	"flag"
	"strings"
)

func init() {
	register(command{
		Path: []string{"nginx", "maintenance"}, Summary: "Show visitors a branded \"back soon\" page (503) for a site, or for every site; no reload, instant",
		Usage: "nginx maintenance on|off|status [domain|all] [--message TEXT]", AdminOnly: true,
		Examples: []string{
			"docklite nginx maintenance status",
			"docklite nginx maintenance on example.com --message \"Moving to a faster server, back by 3pm\"",
			"docklite nginx maintenance off all",
		},
		Run: func(a *app, args []string) error {
			var message string
			pos, err := a.flags("nginx maintenance", args, func(fs *flag.FlagSet) { fs.StringVar(&message, "message", "", "") })
			if err != nil {
				return err
			}
			if len(pos) == 0 {
				return usageError("usage: docklite nginx maintenance on|off|status [domain|all] [--message TEXT]")
			}
			if pos[0] == "status" {
				data, err := a.get("/api/nginx/maintenance")
				if err != nil {
					return err
				}
				a.emitValue(json.RawMessage(data), func() {
					var r struct {
						Active  []string `json:"active"`
						Message string   `json:"message"`
					}
					_ = json.Unmarshal(data, &r)
					if len(r.Active) == 0 {
						a.say("back-soon mode is off everywhere")
						return
					}
					a.say("back-soon mode is on for: %s", strings.Join(r.Active, ", "))
					if r.Message != "" {
						a.say("message: %s", r.Message)
					}
				})
				return nil
			}
			if (pos[0] != "on" && pos[0] != "off") || len(pos) != 2 {
				return usageError("usage: docklite nginx maintenance on|off|status [domain|all] [--message TEXT]")
			}
			on := pos[0] == "on"
			target := pos[1]
			if on {
				if err := a.confirm("show the \"back soon\" page to visitors of " + target + " (they will not see the site until you turn it off)"); err != nil {
					return err
				}
			}
			payload := map[string]any{"domain": target, "on": on}
			if message != "" {
				payload["message"] = message
			}
			data, err := a.post("/api/nginx/maintenance", payload)
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() {
				if on {
					a.say("back-soon page is showing for %s. Turn it off with: docklite nginx maintenance off %s", target, target)
				} else {
					a.say("%s is live again", target)
				}
			})
			return nil
		},
	})
}
