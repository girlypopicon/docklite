package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// secretArg reads a secret given on the command line. "-" reads one line from standard input, so the secret stays out
// of the process list and the shell history.
func secretArg(v string) (string, error) {
	if v != "-" {
		return v, nil
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("expected the secret on standard input")
	}
	return strings.TrimSpace(line), nil
}

func init() {
	register(command{
		Path: []string{"backups", "destinations"}, Summary: "Where backups can be sent: this server, a bucket (R2, B2, S3), another server",
		Usage: "backups destinations", AdminOnly: true,
		Run: func(a *app, args []string) error {
			data, err := a.get("/api/backups/destinations")
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var resp struct {
				Destinations []struct {
					ID      int64  `json:"id"`
					Name    string `json:"name"`
					Type    string `json:"type"`
					Config  string `json:"config"`
					Enabled int    `json:"enabled"`
				} `json:"destinations"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return err
			}
			rows := [][]string{}
			for _, d := range resp.Destinations {
				var c struct {
					Bucket  string `json:"bucket"`
					Host    string `json:"host"`
					Path    string `json:"path"`
					Encrypt bool   `json:"encrypt"`
				}
				_ = json.Unmarshal([]byte(d.Config), &c)
				where := firstNonEmpty(c.Bucket, c.Host, c.Path, "this server")
				lock := "no"
				if c.Encrypt {
					lock = "yes"
				}
				rows = append(rows, []string{d.Name, d.Type, where, lock})
			}
			if len(rows) == 0 {
				a.say("Only this server's own folder so far. Add one with: docklite backups destination add --help")
				return nil
			}
			a.table([]string{"NAME", "TYPE", "WHERE", "LOCKED"}, rows)
			return nil
		},
	})
	register(command{
		Path: []string{"backups", "destination", "add"}, Summary: "Add a place to send backups (s3, sftp, or a connected DockLite server), optionally locked with a key",
		Usage: "backups destination add <name> --type s3|sftp|server|local [options] [--encrypt | --key age1…]", AdminOnly: true,
		Examples: []string{
			"docklite backups destination add \"Cloudflare R2\" --type s3 --endpoint https://<account>.r2.cloudflarestorage.com --region auto --bucket my-backups --access-key AKIA… --secret-key - --encrypt",
			"docklite backups destination add \"Second server\" --type sftp --host 203.0.113.9 --user backup --private-key-file ~/.ssh/id_ed25519 --dir /srv/docklite-backups",
			"docklite backups destination add \"smoll\" --type server --server smoll",
		},
		Run: func(a *app, args []string) error {
			var typ, endpoint, region, bucket, prefix, access, secret, host, user, password, keyFile, dir, server, key, path string
			var port int
			var encrypt bool
			pos, err := a.flags("backups destination add", args, func(fs *flag.FlagSet) {
				fs.StringVar(&typ, "type", "", "")
				fs.StringVar(&endpoint, "endpoint", "", "")
				fs.StringVar(&region, "region", "", "")
				fs.StringVar(&bucket, "bucket", "", "")
				fs.StringVar(&prefix, "prefix", "", "")
				fs.StringVar(&access, "access-key", "", "")
				fs.StringVar(&secret, "secret-key", "", "")
				fs.StringVar(&host, "host", "", "")
				fs.IntVar(&port, "port", 0, "")
				fs.StringVar(&user, "user", "", "")
				fs.StringVar(&password, "password", "", "")
				fs.StringVar(&keyFile, "private-key-file", "", "")
				fs.StringVar(&dir, "dir", "", "")
				fs.StringVar(&server, "server", "", "")
				fs.StringVar(&path, "path", "", "")
				fs.StringVar(&key, "key", "", "")
				fs.BoolVar(&encrypt, "encrypt", false, "")
			})
			if err != nil {
				return err
			}
			if len(pos) != 1 || typ == "" {
				return usageError("usage: docklite backups destination add <name> --type s3|sftp|server|local [options]")
			}
			cfg := map[string]any{}
			switch typ {
			case "s3":
				sk, err := secretArg(secret)
				if err != nil {
					return err
				}
				cfg["endpoint"], cfg["region"], cfg["bucket"], cfg["prefix"], cfg["access_key"], cfg["secret_key"] = endpoint, region, bucket, prefix, access, sk
			case "sftp":
				pw, err := secretArg(password)
				if err != nil {
					return err
				}
				cfg["host"], cfg["port"], cfg["user"], cfg["password"], cfg["dir"] = host, port, user, pw, dir
				if keyFile != "" {
					b, err := os.ReadFile(keyFile)
					if err != nil {
						return err
					}
					cfg["private_key"] = string(b)
				}
			case "server":
				id, err := a.serverID(server)
				if err != nil {
					return err
				}
				cfg["server_id"] = id
			case "local":
				cfg["path"] = path
			default:
				return usageError("--type must be s3, sftp, server or local")
			}
			var identity string
			switch {
			case key != "" && encrypt:
				return usageError("use either --encrypt (make a new key) or --key (use one you have), not both")
			case key != "":
				cfg["encrypt"], cfg["recipient"] = true, key
			case encrypt:
				data, err := a.post("/api/backups/keys", map[string]any{})
				if err != nil {
					return err
				}
				var k struct{ Recipient, Identity string }
				if err := json.Unmarshal(data, &k); err != nil {
					return err
				}
				cfg["encrypt"], cfg["recipient"], identity = true, k.Recipient, k.Identity
			}
			data, err := a.post("/api/backups/destinations", map[string]any{"name": pos[0], "type": typ, "config": cfg})
			if err != nil {
				if identity != "" {
					a.say("The destination wasn't saved, so the key made for it is not needed.")
				}
				return err
			}
			if a.opts.JSON {
				out := map[string]any{}
				_ = json.Unmarshal(data, &out)
				if identity != "" {
					out["identity"] = identity
				}
				b, _ := json.Marshal(out)
				a.emit(b)
				return nil
			}
			var r struct {
				HostKey string `json:"host_key"`
			}
			_ = json.Unmarshal(data, &r)
			a.say("Added %s. Check it works with: docklite backups destination test %q", pos[0], pos[0])
			if r.HostKey != "" {
				a.say("The server's key was saved as %s. If you know what it should be, check it matches.", r.HostKey)
			}
			if identity != "" {
				a.say("")
				a.say("Backups sent there are locked. This is the ONLY copy of the key that opens them;")
				a.say("DockLite does not keep it. Save it in a password manager now:")
				a.say("")
				a.say("%s", identity)
				a.say("")
				a.say("To restore later: docklite backups fetch <id> --key-file <file holding that key>")
			}
			return nil
		},
	})
	register(command{
		Path: []string{"backups", "destination", "test"}, Summary: "Check a destination works: connect, write a tiny file, delete it",
		Usage: "backups destination test <name>", AdminOnly: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite backups destination test <name>")
			}
			id, err := a.destinationID(args[0])
			if err != nil {
				return err
			}
			data, err := a.post("/api/backups/destinations/test", map[string]any{"id": id})
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var r struct {
				OK      bool   `json:"ok"`
				Message string `json:"message"`
				Error   string `json:"error"`
			}
			_ = json.Unmarshal(data, &r)
			if !r.OK {
				return fmt.Errorf("%s", r.Error)
			}
			a.say("%s", r.Message)
			return nil
		},
	})
	register(command{
		Path: []string{"backups", "destination", "remove"}, Summary: "Remove a destination (backups already sent there stay where they are, but DockLite stops tracking how to reach them)",
		Usage: "backups destination remove <name>", AdminOnly: true, Destructive: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite backups destination remove <name>")
			}
			id, err := a.destinationID(args[0])
			if err != nil {
				return err
			}
			if err := a.confirm("remove the backup destination " + args[0]); err != nil {
				return err
			}
			if _, err := a.del(fmt.Sprintf("/api/backups/destinations?id=%d", id)); err != nil {
				return err
			}
			a.say("%s removed", args[0])
			return nil
		},
	})
	register(command{
		Path: []string{"backups", "key", "new"}, Summary: "Make a backup key: the public half locks backups, the private half is the only way to open them",
		Usage: "backups key new", AdminOnly: true,
		Run: func(a *app, args []string) error {
			data, err := a.post("/api/backups/keys", map[string]any{})
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var k struct{ Recipient, Identity string }
			_ = json.Unmarshal(data, &k)
			a.say("Public key (give this to a destination with --key):")
			a.say("  %s", k.Recipient)
			a.say("Private key (save it in a password manager; DockLite does not keep it):")
			a.say("  %s", k.Identity)
			return nil
		},
	})
	register(command{
		Path: []string{"backups", "fetch"}, Summary: "Bring a backup from another destination (or a locked one) to this server, ready to inspect and restore",
		Usage: "backups fetch <id> [--key-file FILE]", AdminOnly: true,
		Run: func(a *app, args []string) error {
			var keyFile string
			pos, err := a.flags("backups fetch", args, func(fs *flag.FlagSet) { fs.StringVar(&keyFile, "key-file", "", "") })
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite backups fetch <id> [--key-file FILE]   (ids come from: docklite backups list)")
			}
			var id int64
			if _, err := fmt.Sscan(pos[0], &id); err != nil || id <= 0 {
				return usageError("the id is the number in the first column of: docklite backups list")
			}
			identity := ""
			if keyFile != "" {
				b, err := os.ReadFile(keyFile)
				if err != nil {
					return err
				}
				identity = string(b)
			}
			data, err := a.post("/api/backups/fetch", map[string]any{"backup_id": id, "identity": identity})
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var r struct {
				File string `json:"file"`
			}
			_ = json.Unmarshal(data, &r)
			a.say("Ready: %s", r.File)
			a.say("Look inside with: docklite backups inspect %s", r.File)
			a.say("Restore a site with: docklite backups restore %s --yes", r.File)
			return nil
		},
	})
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// serverID finds a connected DockLite server by name.
func (a *app) serverID(name string) (int64, error) {
	if name == "" {
		return 0, usageError("--server <name> is needed (see: docklite servers)")
	}
	data, err := a.get("/api/servers")
	if err != nil {
		return 0, err
	}
	var resp struct {
		Servers []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, err
	}
	for _, s := range resp.Servers {
		if strings.EqualFold(s.Name, name) {
			return s.ID, nil
		}
	}
	return 0, fmt.Errorf("no connected server called %s (see: docklite servers)", name)
}
