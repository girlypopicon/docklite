package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
)

func init() {
	register(command{
		Path: []string{"ssl", "preflight"}, Summary: "Check, before issuing, whether a certificate request for a domain will reach this server (looks at DNS, Cloudflare and a test request)",
		Usage: "ssl preflight <domain>", AdminOnly: true,
		Examples: []string{"docklite ssl preflight example.com"},
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite ssl preflight <domain>")
			}
			data, err := a.get("/api/ssl/preflight?domain=" + url.QueryEscape(args[0]))
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() {
				var r struct {
					DNS            string   `json:"dns"`
					Addresses      []string `json:"addresses"`
					Zone           string   `json:"zone"`
					SSLMode        string   `json:"sslMode"`
					Recommendation string   `json:"recommendation"`
					Message        string   `json:"message"`
					Probe          *struct {
						Verdict string `json:"verdict"`
						Detail  string `json:"detail"`
					} `json:"probe"`
				}
				_ = json.Unmarshal(data, &r)
				a.say("DNS: %s %v", r.DNS, r.Addresses)
				if r.Zone != "" {
					a.say("Cloudflare zone: %s (SSL mode: %s)", r.Zone, r.SSLMode)
				}
				if r.Probe != nil {
					a.say("Test request: %s", r.Probe.Verdict)
				}
				a.say("%s", r.Message)
				switch r.Recommendation {
				case "letsencrypt":
					a.say("Next: docklite ssl issue %s", args[0])
				case "origin":
					a.say("Next: docklite ssl origin %s --strict", args[0])
				}
			})
			return nil
		},
	})
	register(command{
		Path: []string{"ssl", "origin"}, Summary: "Use a Cloudflare origin certificate for a site (15 years, no renewal; for sites proxied through Cloudflare)",
		Usage: "ssl origin <domain> [--www] [--strict]", AdminOnly: true, Destructive: true,
		Examples: []string{"docklite ssl origin example.com --www --strict --yes"},
		Run: func(a *app, args []string) error {
			var www, strict bool
			pos, err := a.flags("ssl origin", args, func(fs *flag.FlagSet) {
				fs.BoolVar(&www, "www", false, "")
				fs.BoolVar(&strict, "strict", false, "")
			})
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite ssl origin <domain> [--www] [--strict]")
			}
			what := "switch " + pos[0] + " to a Cloudflare origin certificate (its nginx file changes)"
			if strict {
				what += " and set Cloudflare's SSL mode for the whole domain to Full (strict)"
			}
			if err := a.confirm(what); err != nil {
				return err
			}
			data, err := a.post("/api/ssl/origin", map[string]any{"domain": pos[0], "includeWww": www, "strict": strict})
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() {
				var r struct{ Message, Warning string }
				_ = json.Unmarshal(data, &r)
				a.say("%s", r.Message)
				if r.Warning != "" {
					fmt.Fprintln(a.out, "warning: "+r.Warning)
				}
			})
			return nil
		},
	})
}
