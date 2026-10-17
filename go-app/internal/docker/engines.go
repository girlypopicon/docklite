package docker

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"docklite-agent/internal/demo"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// Engine describes one kind of database DockLite can run: which image, which port inside the container, and where
// it keeps its data. Everything else (creating the container, listing, backups) is shared, so adding a type is
// adding an entry here plus how to dump and restore it (internal/backup).
type Engine struct {
	Type    string // stored in the "docklite.type" label and the databases table
	Label   string // for people
	Image   string
	Port    int    // port inside the container
	DataDir string // where the image keeps data; a named volume is mounted here ("" = leave the image's own volume)
	// Loopback publishes the port on 127.0.0.1 only. New engines default to it: a Redis or MongoDB open to the whole
	// internet is how servers get hacked, and sites in containers reach the database by name on DockLite's network.
	Loopback bool
}

var engines = map[string]Engine{
	"postgres": {Type: "postgres", Label: "PostgreSQL", Image: databaseImageName, Port: 5432},
	"mariadb":  {Type: "mariadb", Label: "MariaDB", Image: "mariadb:11", Port: 3306, DataDir: "/var/lib/mysql", Loopback: true},
	"mongodb":  {Type: "mongodb", Label: "MongoDB", Image: "mongo:7", Port: 27017, DataDir: "/data/db", Loopback: true},
	"redis":    {Type: "redis", Label: "Redis", Image: "redis:7-alpine", Port: 6379, DataDir: "/data", Loopback: true},
}

// EngineFor looks up a database type ("" means postgres, which is what every database was before there were others).
func EngineFor(dbType string) (Engine, bool) {
	if dbType == "" {
		dbType = "postgres"
	}
	e, ok := engines[strings.ToLower(dbType)]
	return e, ok
}

// EngineTypes lists the types a person can create, in the order the dashboard shows them.
func EngineTypes() []Engine {
	return []Engine{engines["postgres"], engines["mariadb"], engines["mongodb"], engines["redis"]}
}

// ContainerEnv is the environment that makes the image create the database, user and password on first start.
func (e Engine) ContainerEnv(name, user, pass, rootPass string) []string {
	switch e.Type {
	case "postgres":
		return []string{
			fmt.Sprintf("POSTGRES_DB=%s", name),
			fmt.Sprintf("POSTGRES_USER=%s", user),
			fmt.Sprintf("POSTGRES_PASSWORD=%s", pass),
			"POSTGRES_INITDB_ARGS=--auth-local=scram-sha-256 --auth-host=scram-sha-256",
		}
	case "mariadb":
		return []string{
			fmt.Sprintf("MARIADB_DATABASE=%s", name),
			fmt.Sprintf("MARIADB_USER=%s", user),
			fmt.Sprintf("MARIADB_PASSWORD=%s", pass),
			fmt.Sprintf("MARIADB_ROOT_PASSWORD=%s", rootPass),
		}
	case "mongodb":
		return []string{
			fmt.Sprintf("MONGO_INITDB_ROOT_USERNAME=%s", user),
			fmt.Sprintf("MONGO_INITDB_ROOT_PASSWORD=%s", pass),
			fmt.Sprintf("MONGO_INITDB_DATABASE=%s", name),
		}
	}
	return nil
}

// ContainerCmd is the command line the container runs (only Redis needs one: it has no user, just a password).
func (e Engine) ContainerCmd(pass string) []string {
	if e.Type == "redis" {
		return []string{"redis-server", "--requirepass", pass, "--appendonly", "yes"}
	}
	return nil
}

// ConnectionURL is what an app puts in its settings, written for reaching the database from another container on
// DockLite's network (host = the container's name, the engine's own port) or from this machine (host = localhost).
func (e Engine) ConnectionURL(host string, port int, name, user, pass string) string {
	switch e.Type {
	case "postgres":
		return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", user, pass, host, port, name)
	case "mariadb":
		return fmt.Sprintf("mysql://%s:%s@%s:%d/%s", user, pass, host, port, name)
	case "mongodb":
		return fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin", user, pass, host, port, name)
	case "redis":
		return fmt.Sprintf("redis://:%s@%s:%d", pass, host, port)
	}
	return ""
}

// ContainerNameFor is the container's name on DockLite's network, which other containers use as the host.
func ContainerNameFor(dbName string) string {
	return fmt.Sprintf("docklite-db-%s", sanitizeDomain(dbName))
}

// CreateDatabaseOfType starts a database container of any engine. For postgres it behaves exactly as before.
func (c *Client) CreateDatabaseOfType(ctx context.Context, dbType, name, username, password string, port int) (id string, hostPort int, user, pass string, err error) {
	e, ok := EngineFor(dbType)
	if !ok {
		return "", 0, "", "", fmt.Errorf("unsupported database type %q", dbType)
	}
	if err := c.ensureNetwork(ctx, dockliteNetworkName); err != nil {
		return "", 0, "", "", err
	}
	if err := c.pullImage(ctx, e.Image); err != nil {
		return "", 0, "", "", err
	}
	if username == "" {
		username = "docklite"
	}
	if password == "" {
		password = generatePassword(24)
	}
	rootPass := ""
	if e.Type == "mariadb" {
		rootPass = generatePassword(24)
	}

	labels := map[string]string{
		"docklite.managed":  "true",
		"docklite.database": name,
		"docklite.type":     e.Type,
		"docklite.username": username,
		"docklite.password": password,
	}
	if rootPass != "" {
		labels["docklite.root_password"] = rootPass
	}
	if port > 0 {
		labels["docklite.db.port"] = strconv.Itoa(port)
	}
	if demo.On {
		labels[demo.Label] = "1"
	}

	portKey := nat.Port(fmt.Sprintf("%d/tcp", e.Port))
	hostPortStr := ""
	if port > 0 {
		hostPortStr = strconv.Itoa(port)
	}
	binding := nat.PortBinding{HostPort: hostPortStr}
	if e.Loopback {
		binding.HostIP = "127.0.0.1"
	}

	containerName := ContainerNameFor(name)
	if e.DataDir != "" {
		// MariaDB and MongoDB ignore the new user and password when their data folder already holds a database. Starting
		// on leftover data would hand back an old database with an old password, so refuse and say why.
		if _, verr := c.Client.VolumeInspect(ctx, containerName+"-data"); verr == nil {
			return "", 0, "", "", fmt.Errorf("data from an earlier %s database named %q still exists on this server; delete it first or choose another name", e.Label, name)
		}
	}
	config := &container.Config{
		Image:        e.Image,
		Env:          e.ContainerEnv(name, username, password, rootPass),
		Cmd:          e.ContainerCmd(password),
		ExposedPorts: nat.PortSet{portKey: struct{}{}},
		Labels:       labels,
	}
	hostConfig := &container.HostConfig{
		PortBindings:  nat.PortMap{portKey: []nat.PortBinding{binding}},
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		NetworkMode:   container.NetworkMode(dockliteNetworkName),
	}
	volumeName := ""
	if e.DataDir != "" {
		// A named volume keeps the data when the container is recreated; it is removed with the database (see RemoveDatabaseContainer).
		volumeName = containerName + "-data"
		volLabels := map[string]string{"docklite.managed": "true", "docklite.database": name}
		if demo.On {
			volLabels[demo.Label] = "1"
		}
		hostConfig.Mounts = []mount.Mount{{Type: mount.TypeVolume, Source: volumeName, Target: e.DataDir, VolumeOptions: &mount.VolumeOptions{Labels: volLabels}}}
	}

	resp, err := c.Client.ContainerCreate(ctx, config, hostConfig, &network.NetworkingConfig{}, nil, containerName)
	if err != nil && strings.Contains(err.Error(), "Conflict.") {
		containerName = fmt.Sprintf("%s-%d", containerName, randomSuffix())
		if e.DataDir != "" {
			volumeName = containerName + "-data"
			hostConfig.Mounts[0].Source = volumeName
		}
		resp, err = c.Client.ContainerCreate(ctx, config, hostConfig, &network.NetworkingConfig{}, nil, containerName)
	}
	if err != nil {
		return "", 0, "", "", err
	}
	if err := c.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", 0, "", "", err
	}

	if port == 0 {
		if inspected, ierr := c.Client.ContainerInspect(ctx, resp.ID); ierr == nil {
			if bindings, ok := inspected.NetworkSettings.Ports[portKey]; ok && len(bindings) > 0 {
				if parsed, perr := strconv.Atoi(bindings[0].HostPort); perr == nil {
					port = parsed
				}
			}
		}
	}
	return resp.ID, port, username, password, nil
}

// RemoveDatabaseContainer deletes the container and, for engines that use one, its data volume. The caller has already
// asked the person to confirm: deleting a database deletes its data. A container that is already gone (removed from
// outside DockLite) is not an error: the leftover data volume is still cleaned up, so the entry can be cleared.
func (c *Client) RemoveDatabaseContainer(ctx context.Context, id, name string) error {
	volumes := map[string]bool{}
	if inspected, err := c.Client.ContainerInspect(ctx, id); err == nil {
		if e, ok := EngineFor(inspected.Config.Labels["docklite.type"]); ok && e.DataDir != "" {
			for _, m := range inspected.Mounts {
				if m.Type == mount.TypeVolume && m.Destination == e.DataDir && strings.HasPrefix(m.Name, "docklite-db-") {
					volumes[m.Name] = true
				}
			}
		}
		if err := c.RemoveContainer(ctx, id); err != nil && !isNoSuchContainer(err) {
			return err
		}
	} else if !isNoSuchContainer(err) {
		return err
	}
	if name != "" {
		// the volume DockLite would have named for this database, if it is one of ours
		vol := ContainerNameFor(name) + "-data"
		if v, err := c.Client.VolumeInspect(ctx, vol); err == nil && v.Labels["docklite.managed"] == "true" {
			volumes[vol] = true
		}
	}
	for v := range volumes {
		_ = c.Client.VolumeRemove(ctx, v, true) // best effort: the database is gone either way
	}
	return nil
}

func isNoSuchContainer(err error) bool {
	return err != nil && (client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container"))
}

// EngineSize asks a running database how much data it holds, in bytes (for Redis: memory in use).
func (c *Client) EngineSize(ctx context.Context, containerID, dbType, name, user, pass string) (int64, error) {
	var cmd, env []string
	switch dbType {
	case "mariadb":
		cmd = []string{"mariadb", "-N", "-u", user, "-e",
			fmt.Sprintf("SELECT COALESCE(SUM(data_length+index_length),0) FROM information_schema.tables WHERE table_schema='%s'", strings.ReplaceAll(name, "'", ""))}
		env = []string{"MYSQL_PWD=" + pass}
	case "mongodb":
		cmd = []string{"mongosh", "--quiet", "-u", user, "-p", pass, "--authenticationDatabase", "admin", name, "--eval", "db.stats().dataSize"}
	case "redis":
		cmd = []string{"redis-cli", "-a", pass, "--no-auth-warning", "INFO", "memory"}
	default:
		return 0, fmt.Errorf("no size check for %q", dbType)
	}
	var out bytes.Buffer
	if err := c.ExecCommandToWriter(ctx, containerID, cmd, env, &out); err != nil {
		return 0, err
	}
	text := strings.TrimSpace(out.String())
	if dbType == "redis" {
		for _, line := range strings.Split(text, "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "used_memory:"); ok {
				return strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			}
		}
		return 0, fmt.Errorf("couldn't read redis memory use")
	}
	return strconv.ParseInt(strings.Fields(text + " 0")[0], 10, 64)
}
