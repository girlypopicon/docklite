package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type transferRequest struct {
	SourcePath string `json:"sourcePath"`
	TargetDir  string `json:"targetDir"`
	Action     string `json:"action"` // move or copy
}

func (h *Handlers) UploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	targetDirParam := r.FormValue("path")
	if targetDirParam == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	targetDir, err := resolveFilesPath(targetDirParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.authorizeFilePath(r, targetDir); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	info, err := os.Stat(targetDir)
	if err != nil || !info.IsDir() {
		writeError(w, http.StatusBadRequest, "target directory not found")
		return
	}

	filename := filepath.Base(header.Filename)
	if filename == "." || filename == ".." || filename == "/" {
		writeError(w, http.StatusBadRequest, "invalid filename")
		return
	}

	// The file itself may already exist as a symlink pointing elsewhere.
	targetPath := filepath.Join(targetDir, filename)
	if err := ensureWithinBase(targetPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.authorizeFilePath(r, targetPath); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	out, err := os.Create(targetPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) DownloadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	pathParam := r.URL.Query().Get("path")
	if pathParam == "" {
		writeError(w, http.StatusBadRequest, "file path is required")
		return
	}
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
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "path is a directory")
		return
	}

	file, err := os.Open(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(target)))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (h *Handlers) TransferFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req transferRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SourcePath == "" || req.TargetDir == "" || req.Action == "" {
		writeError(w, http.StatusBadRequest, "source path, target directory, and action are required")
		return
	}
	if req.Action != "move" && req.Action != "copy" {
		writeError(w, http.StatusBadRequest, "invalid action")
		return
	}

	source, err := resolveFilesPath(req.SourcePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.authorizeFilePath(r, source); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	targetDir, err := resolveFilesPath(req.TargetDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.authorizeFilePath(r, targetDir); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	sourceInfo, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "source not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	targetInfo, err := os.Stat(targetDir)
	if err != nil || !targetInfo.IsDir() {
		writeError(w, http.StatusNotFound, "target directory not found")
		return
	}

	baseName := filepath.Base(source)
	candidate := filepath.Join(targetDir, baseName)
	if source == candidate {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "targetPath": source})
		return
	}

	targetPath := candidate
	if pathExists(candidate) {
		targetPath, err = buildUniquePath(targetDir, baseName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if req.Action == "copy" {
		if err := copyRecursive(source, targetPath, sourceInfo.IsDir()); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		if err := movePath(source, targetPath, sourceInfo.IsDir()); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "targetPath": targetPath})
}

func (h *Handlers) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
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

func buildUniquePath(targetDir string, name string) (string, error) {
	parsed := filepath.Ext(name)
	base := strings.TrimSuffix(name, parsed)
	for counter := 1; counter < 10000; counter++ {
		suffix := fmt.Sprintf("_%d", counter)
		nextName := base + suffix + parsed
		candidate := filepath.Join(targetDir, nextName)
		if !pathExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unable to generate unique filename")
}

func pathExists(targetPath string) bool {
	_, err := os.Stat(targetPath)
	return err == nil
}

func copyRecursive(source string, target string, isDir bool) error {
	if !isDir {
		return copyFile(source, target)
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		return copyFile(path, dest)
	})
}

func copyFile(source string, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(target)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Sync()
}

func movePath(source string, target string, isDir bool) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	} else if !isCrossDevice(err) {
		return err
	}
	if !isDir {
		if err := copyFile(source, target); err != nil {
			return err
		}
		return os.Remove(source)
	}
	if err := copyRecursive(source, target, true); err != nil {
		return err
	}
	return os.RemoveAll(source)
}

func isCrossDevice(err error) bool {
	if err == nil {
		return false
	}
	linkErr, ok := err.(*os.LinkError)
	if !ok {
		return false
	}
	return linkErr.Err == syscall.EXDEV
}
