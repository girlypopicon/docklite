package docker

import (
	"bytes"
	"context"
	"crypto/rand"
	"docklite-agent/internal/demo"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
	"io"
	"math/big"
	"strconv"
	"strings"

	"docklite-agent/internal/models"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

const (
	databaseImageName = "postgres:16-alpine"
)

func (c *Client) CreateDatabaseContainer(ctx context.Context, name string, username string, password string, port int) (string, int, string, string, error) {
	if err := c.ensureNetwork(ctx, dockliteNetworkName); err != nil {
		return "", 0, "", "", err
	}
	if err := c.pullImage(ctx, databaseImageName); err != nil {
		return "", 0, "", "", err
	}

	if username == "" {
		username = "docklite"
	}
	if password == "" {
		password = generatePassword(24)
	}

	hostPort := ""
	if port > 0 {
		hostPort = strconv.Itoa(port)
	}

	labels := map[string]string{
		"docklite.managed":  "true",
		"docklite.database": name,
		"docklite.type":     "postgres",
		"docklite.username": username,
		"docklite.password": password,
	}
	if port > 0 {
		labels["docklite.db.port"] = strconv.Itoa(port)
	}
	if demo.On {
		labels[demo.Label] = "1"
	}

	portKey := nat.Port("5432/tcp")
	config := &container.Config{
		Image: databaseImageName,
		Env: []string{
			fmt.Sprintf("POSTGRES_DB=%s", name),
			fmt.Sprintf("POSTGRES_USER=%s", username),
			fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
			"POSTGRES_INITDB_ARGS=--auth-local=scram-sha-256 --auth-host=scram-sha-256",
		},
		ExposedPorts: nat.PortSet{portKey: struct{}{}},
		Labels:       labels,
	}
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			portKey: []nat.PortBinding{{HostPort: hostPort}},
		},
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		NetworkMode:   container.NetworkMode(dockliteNetworkName),
	}

	containerName := fmt.Sprintf("docklite-db-%s", sanitizeDomain(name))
	resp, err := c.Client.ContainerCreate(ctx, config, hostConfig, &network.NetworkingConfig{}, nil, containerName)
	if err != nil && strings.Contains(err.Error(), "Conflict.") {
		containerName = fmt.Sprintf("docklite-db-%s-%d", sanitizeDomain(name), randomSuffix())
		resp, err = c.Client.ContainerCreate(ctx, config, hostConfig, &network.NetworkingConfig{}, nil, containerName)
	}
	if err != nil {
		return "", 0, "", "", err
	}
	if err := c.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", 0, "", "", err
	}

	if port == 0 {
		inspected, err := c.Client.ContainerInspect(ctx, resp.ID)
		if err != nil {
			return resp.ID, 0, username, password, nil
		}
		if bindings, ok := inspected.NetworkSettings.Ports[portKey]; ok && len(bindings) > 0 {
			if parsed, err := strconv.Atoi(bindings[0].HostPort); err == nil {
				port = parsed
			}
		}
	}

	return resp.ID, port, username, password, nil
}

func (c *Client) ListDatabases(ctx context.Context) ([]models.DatabaseInfo, error) {
	containers, err := c.VisibleContainers(ctx, true)
	if err != nil {
		return nil, err
	}

	results := make([]models.DatabaseInfo, 0)
	for _, item := range containers {
		if item.Labels["docklite.type"] != "postgres" && item.Labels["docklite.database"] == "" {
			continue
		}
		name := item.Labels["docklite.database"]
		if name == "" {
			name = strings.TrimPrefix(item.Names[0], "/")
		}
		username := item.Labels["docklite.username"]
		password := item.Labels["docklite.password"]
		port := 0
		if label := item.Labels["docklite.db.port"]; label != "" {
			if parsed, err := strconv.Atoi(label); err == nil {
				port = parsed
			}
		}
		if port == 0 {
			for _, p := range item.Ports {
				if p.PrivatePort == 5432 && p.PublicPort != 0 {
					port = int(p.PublicPort)
					break
				}
			}
		}

		results = append(results, models.DatabaseInfo{
			ID:       item.ID,
			Name:     name,
			Port:     port,
			Username: username,
			Password: password,
			Status:   item.Status,
		})
	}

	return results, nil
}

func (c *Client) ExecPostgres(ctx context.Context, containerID string, username string, dbName string, password string, sql string, jsonOutput bool) (string, error) {
	if username == "" {
		username = "docklite"
	}
	if dbName == "" {
		dbName = "postgres"
	}

	cmd := []string{
		"psql",
		"-U", username,
		"-d", dbName,
		"-h", "127.0.0.1",
		"-p", "5432",
		"-v", "ON_ERROR_STOP=1",
		"-X",
	}
	if jsonOutput {
		cmd = append(cmd, "-t", "-A")
	}
	cmd = append(cmd, "-c", sql)

	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}
	if password != "" {
		execConfig.Env = []string{fmt.Sprintf("PGPASSWORD=%s", password)}
	}

	execResp, err := c.Client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", err
	}

	attachResp, err := c.Client.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", err
	}
	defer attachResp.Close()

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, attachResp.Reader); err != nil && err != io.EOF {
		return "", err
	}

	inspect, err := c.Client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return "", err
	}
	if inspect.ExitCode != 0 {
		message := strings.TrimSpace(stderrBuf.String())
		if message == "" {
			message = fmt.Sprintf("psql exited with code %d", inspect.ExitCode)
		}
		return "", fmt.Errorf(message)
	}
	return strings.TrimSpace(stdoutBuf.String()), nil
}

func (c *Client) ExecPostgresToWriter(ctx context.Context, containerID string, username string, dbName string, password string, sql string, writer io.Writer) error {
	if username == "" {
		username = "docklite"
	}
	if dbName == "" {
		dbName = "postgres"
	}

	cmd := []string{
		"psql",
		"-U", username,
		"-d", dbName,
		"-h", "127.0.0.1",
		"-p", "5432",
		"-v", "ON_ERROR_STOP=1",
		"-X",
		"-c", sql,
	}

	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}
	if password != "" {
		execConfig.Env = []string{fmt.Sprintf("PGPASSWORD=%s", password)}
	}

	execResp, err := c.Client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return err
	}

	attachResp, err := c.Client.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return err
	}
	defer attachResp.Close()

	var stderrBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(writer, &stderrBuf, attachResp.Reader); err != nil && err != io.EOF {
		return err
	}

	inspect, err := c.Client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return err
	}
	if inspect.ExitCode != 0 {
		message := strings.TrimSpace(stderrBuf.String())
		if message == "" {
			message = fmt.Sprintf("psql exited with code %d", inspect.ExitCode)
		}
		return fmt.Errorf(message)
	}
	return nil
}

func (c *Client) ExecCommandToWriter(ctx context.Context, containerID string, cmd []string, env []string, writer io.Writer) error {
	if len(cmd) == 0 {
		return fmt.Errorf("command is required")
	}

	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
		Env:          env,
	}

	execResp, err := c.Client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return err
	}

	attachResp, err := c.Client.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return err
	}
	defer attachResp.Close()

	var stderrBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(writer, &stderrBuf, attachResp.Reader); err != nil && err != io.EOF {
		return err
	}

	inspect, err := c.Client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return err
	}
	if inspect.ExitCode != 0 {
		message := strings.TrimSpace(stderrBuf.String())
		if message == "" {
			message = fmt.Sprintf("command exited with code %d", inspect.ExitCode)
		}
		return fmt.Errorf(message)
	}
	return nil
}

func generatePassword(length int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if length <= 0 {
		length = 16
	}
	var b strings.Builder
	for i := 0; i < length; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		b.WriteByte(letters[n.Int64()])
	}
	return b.String()
}

func randomSuffix() int64 {
	n, err := rand.Int(rand.Reader, big.NewInt(99999))
	if err != nil {
		return 0
	}
	return n.Int64()
}
