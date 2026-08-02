package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	filesBaseDir   = "/var/www/sites"
	maxEditSize    = 256 * 1024
	maxListEntries = 5000
)

type fileEntry struct {
	Name        string `json:"name"`
	IsDirectory bool   `json:"isDirectory"`
}

type fileContentResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type createFileRequest struct {
	BasePath string `json:"basePath"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"` // file or folder
}

type saveFileRequest struct {
	Path     string `json:"path"`
	FilePath string `json:"filePath"`
	Content  string `json:"content"`
}

type deletePathRequest struct {
	Path string `json:"path"`
}

type renamePathRequest struct {
	Path    string `json:"path"`
	NewName string `json:"new_name"`
}

func (h *Handlers) Files(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listFiles(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) FileContent(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.readFile(w, r)
	case http.MethodPost, http.MethodPut:
		h.saveFile(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) CreatePath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req createFileRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	target := ""
	if req.BasePath != "" && req.Name != "" {
		if strings.Contains(req.Name, "/") || strings.Contains(req.Name, "\\") {
			writeError(w, http.StatusBadRequest, "invalid name")
			return
		}
		base, err := resolveFilesPath(req.BasePath)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		target = filepath.Join(base, req.Name)
		if err := ensureWithinBase(target); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		resolved, err := resolveFilesPath(req.Path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		target = resolved
	}
	if err := h.authorizeFilePath(r, target); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	switch strings.ToLower(req.Type) {
	case "folder":
		if err := os.MkdirAll(target, 0o775); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	case "file":
		if err := os.MkdirAll(filepath.Dir(target), 0o775); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := os.Stat(target); err == nil {
			writeError(w, http.StatusBadRequest, "file already exists")
			return
		}
		if err := os.WriteFile(target, []byte(""), 0o644); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "type must be file or folder")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) DeletePath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req deletePathRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	target, err := resolveFilesPath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.authorizeFilePath(r, target); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	if err := os.RemoveAll(target); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) RenamePath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req renamePathRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.NewName == "" || strings.Contains(req.NewName, "/") || strings.Contains(req.NewName, "\\") {
		writeError(w, http.StatusBadRequest, "invalid new name")
		return
	}
	source, err := resolveFilesPath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.authorizeFilePath(r, source); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	target := filepath.Join(filepath.Dir(source), req.NewName)
	if err := ensureWithinBase(target); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.Rename(source, target); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) listFiles(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		writeError(w, http.StatusRequestTimeout, "timeout")
		return
	default:
	}

	pathParam := r.URL.Query().Get("path")
	target, err := resolveFilesPath(pathParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.authorizeFilePath(r, target); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	results := make([]fileEntry, 0, len(entries))
	for i, entry := range entries {
		if i >= maxListEntries {
			break
		}
		results = append(results, fileEntry{
			Name:        entry.Name(),
			IsDirectory: entry.IsDir(),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":    pathParam,
		"entries": results,
	})
}

func (h *Handlers) readFile(w http.ResponseWriter, r *http.Request) {
	pathParam := r.URL.Query().Get("path")
	target, err := resolveFilesPath(pathParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.authorizeFilePath(r, target); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "path is a directory")
		return
	}
	if info.Size() > maxEditSize {
		writeError(w, http.StatusRequestEntityTooLarge, "file too large to edit")
		return
	}

	data, err := os.ReadFile(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, fileContentResponse{
		Path:    pathParam,
		Content: string(data),
	})
}

func (h *Handlers) saveFile(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxEditSize+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(body) > maxEditSize {
		writeError(w, http.StatusRequestEntityTooLarge, "file too large to save")
		return
	}
	var req saveFileRequest
	if err := jsonUnmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	pathInput := req.Path
	if pathInput == "" {
		pathInput = req.FilePath
	}
	target, err := resolveFilesPath(pathInput)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.authorizeFilePath(r, target); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o775); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(req.Content) > maxEditSize {
		writeError(w, http.StatusRequestEntityTooLarge, "file too large to save")
		return
	}
	if err := os.WriteFile(target, []byte(req.Content), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// authorizeFilePath enforces that non-admin users can only access their own
// subdirectory under filesBaseDir. All OS-level ownership belongs to the
// 'docklite' system user; this function implements DockLite's application-
// layer access control.
func (h *Handlers) authorizeFilePath(r *http.Request, resolvedPath string) error {
	if isAdminRole(r) {
		return nil
	}
	userID, ok := readUserIDFromContext(r)
	if !ok {
		return errForbidden
	}
	user, err := h.store.GetUserByIDFull(userID)
	if err != nil || user == nil {
		return errForbidden
	}
	allowedBase := filepath.Clean(filepath.Join(filesBaseDir, user.Username)) + string(filepath.Separator)
	cleaned := filepath.Clean(resolvedPath) + string(filepath.Separator)
	if !strings.HasPrefix(cleaned, allowedBase) {
		return errForbidden
	}
	return nil
}

func resolveFilesPath(pathParam string) (string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(pathParam))
	if cleaned == "." || cleaned == "" || cleaned == "/" {
		return filesBaseDir, nil
	}
	if strings.HasPrefix(cleaned, "..") {
		return "", errors.New("invalid path")
	}
	if filepath.IsAbs(cleaned) {
		if err := ensureWithinBase(cleaned); err != nil {
			return "", err
		}
		return cleaned, nil
	}
	joined := filepath.Join(filesBaseDir, cleaned)
	if err := ensureWithinBase(joined); err != nil {
		return "", err
	}
	return joined, nil
}

func ensureWithinBase(path string) error {
	base := filepath.Clean(filesBaseDir) + string(filepath.Separator)
	cleaned := filepath.Clean(path) + string(filepath.Separator)
	if !strings.HasPrefix(cleaned, base) {
		return errors.New("path outside base directory")
	}
	return nil
}

func jsonUnmarshal(data []byte, v any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}
