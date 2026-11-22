package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"
)

var databaseTypes = []string{"postgres", "mysql", "mariadb", "mongodb", "redis", "sqlite"}

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

// createSQLite registers a SQLite file that lives inside a site (DockLite also finds these by itself: see databases scan).
func (a *app) createSQLite(name, site, dbPath string) error {
	if site == "" || dbPath == "" {
		return usageError("a SQLite database is a file inside a site: add --site DOMAIN and --path /app/data/%s.db (the path as the site sees it)", name)
	}
	c, err := a.resolveContainer(site)
	if err != nil {
		return err
	}
	data, err := a.post("/api/databases", map[string]any{"name": name, "type": "sqlite", "container_id": c.ID, "db_path": dbPath})
	if err != nil {
		return err
	}
	if a.opts.JSON {
		a.emit(data)
		return nil
	}
	a.say("Registered the SQLite database %s: the file %s inside %s. It has no server or password; open it from its card in the dashboard.", name, dbPath, site)
	return nil
}

func init() {
	register(command{
		Path: []string{"databases", "scan"}, Summary: "Look inside the sites for SQLite database files and add the ones DockLite doesn't know yet",
		Usage: "databases scan", AdminOnly: true,
		Examples: []string{"docklite databases scan   # run after a site starts using a new .db / .sqlite file"},
		Run: func(a *app, args []string) error {
			data, err := a.post("/api/databases/scan", map[string]any{})
			if err != nil {
				return err
			}
			var res struct {
				Added    []string `json:"added"`
				Existing int      `json:"existing"`
			}
			_ = json.Unmarshal(data, &res)
			a.emitValue(res, func() {
				if len(res.Added) == 0 {
					a.say("No new SQLite databases found (%d already known).", res.Existing)
					return
				}
				a.say("Added %d SQLite database(s): %s", len(res.Added), strings.Join(res.Added, ", "))
			})
			return nil
		},
	})
	register(command{
		Path: []string{"databases", "types"}, Summary: "The kinds of database you can create",
		Usage: "databases types",
		Run: func(a *app, args []string) error {
			a.say("sqlite    SQLite: one file inside a site, no server or password (DockLite finds these by itself; see: databases scan)\npostgres  PostgreSQL: tables of rows and columns; a strong all-round choice\nmysql     MySQL: tables; the most widely used open-source database, what WordPress and most PHP sites expect\nmariadb   MariaDB: a MySQL-compatible fork with the same tools and many of the same users\nmongodb   MongoDB: flexible \"documents\" instead of tables (NoSQL)\nredis     Redis: very fast key/value storage in memory, for caching and sessions (NoSQL)")
			return nil
		},
	})
	register(command{
		Path: []string{"databases", "create"}, Summary: "Create a database (its own container; reachable from this server and from your sites)",
		Usage: "databases create <name> [--type postgres|mysql|mariadb|mongodb|redis|sqlite] [--username NAME] [--password-stdin] [--site DOMAIN --path /app/data/file.db]", AdminOnly: true,
		Examples: []string{"docklite databases create wordpress --type mariadb", "docklite databases create events --type mongodb --json",
			"docklite databases create board --type sqlite --site example.com --path /app/data/board.db   # a file inside a site"},
		Run: func(a *app, args []string) error {
			var typ, user, site, dbPath string
			var stdin bool
			pos, err := a.flags("databases create", args, func(fs *flag.FlagSet) {
				fs.StringVar(&typ, "type", "postgres", "")
				fs.StringVar(&user, "username", "", "")
				fs.BoolVar(&stdin, "password-stdin", false, "")
				fs.StringVar(&site, "site", "", "")
				fs.StringVar(&dbPath, "path", "", "")
			})
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite databases create <name> [--type %s]", strings.Join(databaseTypes, "|"))
			}
			typ = strings.ToLower(typ)
			if typ == "sqlite" {
				return a.createSQLite(pos[0], site, dbPath)
			}
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
			isSQLite := false
			for _, r := range rows {
				if r.Name == args[0] || fmt.Sprint(r.ID) == args[0] {
					id = r.ID
					isSQLite = r.Type == "sqlite"
				}
			}
			if id == 0 {
				return fmt.Errorf("no database called %s (see: docklite databases list)", args[0])
			}
			if isSQLite {
				if err := a.confirm(fmt.Sprintf("remove the SQLite database %s from DockLite (the file itself stays in the site and won't be re-added)", args[0])); err != nil {
					return err
				}
			} else if err := a.confirm(fmt.Sprintf("delete the database %s and ALL of its data", args[0])); err != nil {
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
