package docker

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
	"io"
	"math/big"
	"strconv"
	"strings"

	"docklite-agent/internal/models"
)

const (
	databaseImageName = "postgres:16-alpine"
)

func (c *Client) CreateDatabaseContainer(ctx context.Context, name string, username string, password string, port int) (string, int, string, string, error) {
	return c.CreateDatabaseOfType(ctx, "postgres", name, username, password, port)
}

func (c *Client) ListDatabases(ctx context.Context) ([]models.DatabaseInfo, error) {
	containers, err := c.VisibleContainers(ctx, true)
	if err != nil {
		return nil, err
	}

	results := make([]models.DatabaseInfo, 0)
	for _, item := range containers {
		if item.Labels["docklite.type"] == "" && item.Labels["docklite.database"] == "" {
			continue
		}
		engine, known := EngineFor(item.Labels["docklite.type"])
		if !known || item.Labels["docklite.database"] == "" && item.Labels["docklite.type"] != "postgres" {
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
				if int(p.PrivatePort) == engine.Port && p.PublicPort != 0 {
					port = int(p.PublicPort)
					break
				}
			}
		}

		results = append(results, models.DatabaseInfo{
			ID:       item.ID,
			Name:     name,
			Type:     engine.Type,
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
