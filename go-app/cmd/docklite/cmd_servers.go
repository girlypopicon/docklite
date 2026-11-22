package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"
)

func init() {
	register(command{
		Path: []string{"share"}, Summary: "Make a join code so another DockLite (a hub) can show this server in its dashboard",
		Usage: "share [--label NAME]", AdminOnly: true,
		Examples: []string{"docklite share", "docklite share --label home-hub"},
		Run: func(a *app, args []string) error {
			var label string
			if _, err := a.flags("share", args, func(fs *flag.FlagSet) { fs.StringVar(&label, "label", "", "") }); err != nil {
				return err
			}
			data, err := a.post("/api/servers/join-code", map[string]any{"label": label})
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() {
				var res struct {
					Join    string `json:"join"`
					URL     string `json:"url"`
					Warning string `json:"warning"`
				}
				_ = json.Unmarshal(data, &res)
				a.say("This server is at %s.", res.URL)
				a.say("On the hub, run:  docklite servers add <code>   (or paste the code into Add server in the dashboard)")
				a.say("")
				a.say("%s", res.Join)
				a.say("")
				a.say("The code contains a token that gives admin access to this server. Don't paste it anywhere public.")
				a.say("To cut the hub off later, revoke the token named \"hub: ...\" in the token list.")
				if res.Warning != "" {
					a.say("Warning: %s", res.Warning)
				}
			})
			return nil
		},
	})
	register(command{
		Path: []string{"servers", "list"}, Summary: "The other servers this hub shows", Usage: "servers list", AdminOnly: true,
		Run: func(a *app, args []string) error {
			data, err := a.get("/api/servers")
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() {
				var res struct {
					Servers []struct {
						ID        int64  `json:"id"`
						Name      string `json:"name"`
						URL       string `json:"url"`
						CreatedAt string `json:"created_at"`
					} `json:"servers"`
				}
				_ = json.Unmarshal(data, &res)
				if len(res.Servers) == 0 {
					a.say("No other servers yet. On the other server run `docklite share`, then here: docklite servers add <code>")
					return
				}
				rows := [][]string{}
				for _, s := range res.Servers {
					rows = append(rows, []string{s.Name, s.URL, s.CreatedAt})
				}
				a.table([]string{"NAME", "ADDRESS", "ADDED"}, rows)
			})
			return nil
		},
	})
	register(command{
		Path: []string{"servers", "add"}, Summary: "Show another server in this dashboard, using the join code from its `docklite share`",
		Usage: "servers add <join-code> [--name NAME]", AdminOnly: true,
		Run: func(a *app, args []string) error {
			var name string
			pos, err := a.flags("servers add", args, func(fs *flag.FlagSet) { fs.StringVar(&name, "name", "", "") })
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite servers add <join-code> [--name NAME]")
			}
			data, err := a.post("/api/servers", map[string]any{"join": pos[0], "name": name})
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() {
				var res struct {
					Server struct{ Name, URL string } `json:"server"`
				}
				_ = json.Unmarshal(data, &res)
				a.say("Added %s (%s). It now has a tab in the dashboard.", res.Server.Name, res.Server.URL)
			})
			return nil
		},
	})
	register(command{
		Path: []string{"servers", "remove"}, Summary: "Stop showing another server (it is not changed; revoke its hub token there to be sure)",
		Usage: "servers remove <name> --yes", AdminOnly: true, Destructive: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite servers remove <name> --yes")
			}
			data, err := a.get("/api/servers")
			if err != nil {
				return err
			}
			var res struct {
				Servers []struct {
					ID   int64  `json:"id"`
					Name string `json:"name"`
				} `json:"servers"`
			}
			_ = json.Unmarshal(data, &res)
			for _, s := range res.Servers {
				if strings.EqualFold(s.Name, args[0]) {
					if err := a.confirm(fmt.Sprintf("stop showing %s in this dashboard", s.Name)); err != nil {
						return err
					}
					if _, err := a.del(fmt.Sprintf("/api/servers/%d", s.ID)); err != nil {
						return err
					}
					a.say("Removed %s.", s.Name)
					return nil
				}
			}
			return usageError("no server named %q; see `docklite servers list`", args[0])
		},
	})
}
