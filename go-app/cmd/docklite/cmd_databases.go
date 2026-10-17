package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"
)

var databaseTypes = []string{"postgres", "mariadb", "mongodb", "redis"}

type dbStatsRow struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Port  int    `json:"port"`
	Size  int64  `json:"size"`
	Owner string `json:"username"`
}

func (a *app) databaseRows() ([]dbStatsRow, error) {
	data, err := a.get("/api/databases/stats")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Databases []dbStatsRow `json:"databases"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Databases, nil
}

func init() {
	register(command{
		Path: []string{"databases", "types"}, Summary: "The kinds of database you can create",
		Usage: "databases types",
		Run: func(a *app, args []string) error {
			a.say("postgres  PostgreSQL: tables of rows and columns; a strong all-round choice\nmariadb   MariaDB: tables, MySQL-compatible; what WordPress and most PHP sites expect\nmongodb   MongoDB: flexible \"documents\" instead of tables (NoSQL)\nredis     Redis: very fast key/value storage in memory, for caching and sessions (NoSQL)")
			return nil
		},
	})
	register(command{
		Path: []string{"databases", "create"}, Summary: "Create a database (its own container; reachable from this server and from your sites)",
		Usage: "databases create <name> [--type postgres|mariadb|mongodb|redis] [--username NAME] [--password-stdin]", AdminOnly: true,
		Examples: []string{"docklite databases create wordpress --type mariadb", "docklite databases create events --type mongodb --json"},
		Run: func(a *app, args []string) error {
			var typ, user string
			var stdin bool
			pos, err := a.flags("databases create", args, func(fs *flag.FlagSet) {
				fs.StringVar(&typ, "type", "postgres", "")
				fs.StringVar(&user, "username", "", "")
				fs.BoolVar(&stdin, "password-stdin", false, "")
			})
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite databases create <name> [--type %s]", strings.Join(databaseTypes, "|"))
			}
			typ = strings.ToLower(typ)
			known := false
			for _, t := range databaseTypes {
				known = known || t == typ
			}
			if !known {
				return usageError("unknown type %q; choose one of: %s (see: docklite databases types)", typ, strings.Join(databaseTypes, ", "))
			}
			body := map[string]any{"name": pos[0], "type": typ}
			if user != "" {
				body["username"] = user
			}
			if stdin {
				pw, err := a.readLine("")
				if err != nil {
					return err
				}
				if pw != "" {
					body["password"] = pw
				}
			}
			data, err := a.post("/api/databases", body)
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var resp struct {
				Connection struct {
					Type         string `json:"type"`
					Port         int    `json:"port"`
					URL          string `json:"url"`
					ContainerURL string `json:"container_url"`
				} `json:"connection"`
			}
			_ = json.Unmarshal(data, &resp)
			a.say("Created the %s database %s.", typ, pos[0])
			a.say("From this server:  %s", resp.Connection.URL)
			a.say("From a site (another container on DockLite's network):  %s", resp.Connection.ContainerURL)
			a.say("The password is shown only now: save it.")
			return nil
		},
	})
	register(command{
		Path: []string{"databases", "delete"}, Summary: "Delete a database AND its data (name or id from `databases list`)",
		Usage: "databases delete <name|id> --yes", AdminOnly: true, Destructive: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite databases delete <name|id> --yes")
			}
			rows, err := a.databaseRows()
			if err != nil {
				return err
			}
			var id int64
			for _, r := range rows {
				if r.Name == args[0] || fmt.Sprint(r.ID) == args[0] {
					id = r.ID
				}
			}
			if id == 0 {
				return fmt.Errorf("no database called %s (see: docklite databases list)", args[0])
			}
			if err := a.confirm(fmt.Sprintf("delete the database %s and ALL of its data", args[0])); err != nil {
				return err
			}
			if _, err := a.del(fmt.Sprintf("/api/databases/%d", id)); err != nil {
				return err
			}
			a.say("database %s deleted", args[0])
			return nil
		},
	})
}
