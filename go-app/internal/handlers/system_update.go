package handlers

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	updateLogFile      = "/var/log/docklite/update.log"
	updateAuditLogFile = "/var/log/docklite/update-audit.log"
	updatePIDFile      = "/tmp/docklite-update.pid"
	updateLogTail      = 80
	updateCmdTimeout   = 30 * time.Minute // 30 minute timeout for update script
)

var updateRunning atomic.Bool

func installDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "/opt/docklite"
	}
	// binary lives at <INSTALL_DIR>/bin/docklite-agent
	return filepath.Dir(filepath.Dir(exe))
}

type updateStatusResponse struct {
	Version         string   `json:"version"`
	GitHash         string   `json:"gitHash"`
	Branch          string   `json:"branch"`
	CommitsBehind   int      `json:"commitsBehind"`
	UpdateAvailable bool     `json:"updateAvailable"`
	UpdateRunning   bool     `json:"updateRunning"`
	LastUpdated     string   `json:"lastUpdated"`
	Log             []string `json:"log"`
}

func (h *Handlers) SystemUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	dir := installDir()
	resp := updateStatusResponse{
		Version:       readVersion(dir),
		GitHash:       runGit(dir, "rev-parse", "--short", "HEAD"),
		Branch:        runGit(dir, "rev-parse", "--abbrev-ref", "HEAD"),
		UpdateRunning: updateRunning.Load() || pidFileRunning(),
		Log:           tailLog(updateLogFile, updateLogTail),
	}

	// non-blocking update check with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	behind, err := commitsBeindOrigin(ctx, dir, resp.Branch)
	if err == nil {
		resp.CommitsBehind = behind
		resp.UpdateAvailable = behind > 0
	}

	resp.LastUpdated = lastModified(updateLogFile)

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) SystemUpdateRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isSuperAdminRole(r) {
		writeError(w, http.StatusForbidden, "super_admin required")
		return
	}
	if updateRunning.Swap(true) {
		writeError(w, http.StatusConflict, "update already in progress")
		return
	}

	dir := installDir()
	scriptPath := filepath.Join(dir, "scripts", "update.sh")

	// Get user info for audit logging
	var userID *int64
	var role string
	if uid, ok := readUserIDFromContext(r); ok {
		userID = &uid
	}
	if r, ok := readUserRoleFromContext(r); ok {
		role = r
	}

	// Validate script permissions and ownership
	if err := validateUpdateScript(scriptPath); err != nil {
		updateRunning.Store(false)
		auditLogUpdate(userID, role, "update_run", "permission_denied", err.Error())
		writeError(w, http.StatusInternalServerError, "script validation failed: "+err.Error())
		return
	}

	// Compute and log script hash for integrity tracking
	scriptHash, err := hashFile(scriptPath)
	if err != nil {
		updateRunning.Store(false)
		auditLogUpdate(userID, role, "update_run", "hash_failed", err.Error())
		writeError(w, http.StatusInternalServerError, "script hash computation failed")
		return
	}
	if err := validateScriptHashAllowlist(scriptHash); err != nil {
		updateRunning.Store(false)
		auditLogUpdate(userID, role, "update_run", "hash_mismatch", err.Error())
		writeError(w, http.StatusForbidden, "update script integrity check failed")
		return
	}

	// Log audit entry for update start
	auditLogUpdate(userID, role, "update_run", "started", fmt.Sprintf("script_hash=%s", scriptHash))

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})

	go func() {
		defer updateRunning.Store(false)

		// Rotate log
		_ = os.MkdirAll(filepath.Dir(updateLogFile), 0o755)
		logF, err := os.OpenFile(updateLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			auditLogUpdate(userID, role, "update_run", "failed", "could not open log file")
			return
		}
		defer logF.Close()

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), updateCmdTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "bash", scriptPath)
		cmd.Env = append(os.Environ(),
			"INSTALL_DIR="+dir,
			"LOG_FILE="+updateLogFile,
			"PID_FILE="+updatePIDFile,
		)
		cmd.Stdout = logF
		cmd.Stderr = logF
		// New session so we're not killed by the agent's cgroup stop signal
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		err = cmd.Run()
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				auditLogUpdate(userID, role, "update_run", "failed_timeout", "update command exceeded timeout")
				return
			}
			auditLogUpdate(userID, role, "update_run", "failed", err.Error())
		} else {
			auditLogUpdate(userID, role, "update_run", "completed", "success")
		}
	}()
}

// helpers

func readVersion(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

func runGit(dir string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func commitsBeindOrigin(ctx context.Context, dir, branch string) (int, error) {
	// fetch first (with timeout)
	fetch := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "origin", "--quiet")
	_ = fetch.Run()

	out, err := exec.CommandContext(ctx, "git", "-C", dir,
		"rev-list", "HEAD..origin/"+branch, "--count").Output()
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, err
	}
	return n, nil
}

func tailLog(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

func lastModified(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return info.ModTime().UTC().Format(time.RFC3339)
}

func pidFileRunning() bool {
	data, err := os.ReadFile(updatePIDFile)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// validateUpdateScript checks script ownership, permissions, and integrity
func validateUpdateScript(scriptPath string) error {
	fileInfo, err := os.Lstat(scriptPath)
	if err != nil {
		return fmt.Errorf("script not found: %w", err)
	}
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("script must not be a symlink")
	}

	// Check if file exists
	fileInfo, err = os.Stat(scriptPath)
	if err != nil {
		return fmt.Errorf("script not found: %w", err)
	}

	// Check if it's a regular file (not a symlink or directory)
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("script is not a regular file")
	}

	// Check permissions (should not be world-writable)
	permissions := fileInfo.Mode().Perm()
	if permissions&0o002 != 0 {
		return fmt.Errorf("script is world-writable (permissions: %o)", permissions)
	}
	if permissions&0o111 == 0 {
		return fmt.Errorf("script is not executable (permissions: %o)", permissions)
	}

	// Check ownership (should be root or docklite user if possible)
	stat := fileInfo.Sys().(*syscall.Stat_t)
	currentUser, err := user.Current()
	if err == nil {
		currentUID, _ := strconv.ParseUint(currentUser.Uid, 10, 32)
		// Allow if owned by root (0) or current user
		if stat.Uid != 0 && stat.Uid != uint32(currentUID) {
			return fmt.Errorf("script not owned by root or current user (owner UID: %d)", stat.Uid)
		}
	}

	return nil
}

func validateScriptHashAllowlist(scriptHash string) error {
	allowlist := strings.TrimSpace(os.Getenv("DOCKLITE_UPDATE_SCRIPT_SHA256"))
	if allowlist == "" {
		return nil
	}
	for _, item := range strings.Split(allowlist, ",") {
		if strings.EqualFold(strings.TrimSpace(item), scriptHash) {
			return nil
		}
	}
	return fmt.Errorf("script hash %s does not match allowlist", scriptHash)
}

// hashFile computes SHA256 hash of a file
func hashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// auditLogUpdate records security-relevant update operations
func auditLogUpdate(userID *int64, role string, action string, status string, details string) error {
	// Create audit log directory
	logDir := filepath.Dir(updateAuditLogFile)
	_ = os.MkdirAll(logDir, 0o755)

	// Open or create audit log file
	file, err := os.OpenFile(updateAuditLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Build audit log entry
	timestamp := time.Now().UTC().Format(time.RFC3339)
	userStr := "unknown"
	if userID != nil {
		userStr = fmt.Sprintf("uid:%d", *userID)
	}

	auditEntry := fmt.Sprintf("[%s] user=%s role=%s action=%s status=%s details=%s\n",
		timestamp, userStr, role, action, status, details)

	_, err = file.WriteString(auditEntry)
	return err
}
