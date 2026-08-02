package handlers

// .dklpkg format — a gzipped tar archive with the following layout:
//
//   manifest.dkl        ← the site manifest (JSON)
//   files/              ← full snapshot of the site directory
//     index.html
//     ...
//
// This is enough to fully recreate the site on a different machine without
// needing the original files to already be present on disk.
// Contrast with .dkl (manifest-only) which assumes the files are already there.

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	dklpkgManifestEntry = "manifest.dkl"
	dklpkgFilesPrefix   = "files/"
	maxImportSize       = 500 << 20 // 500 MB
)

// ExportSite streams a .dklpkg archive for a container identified by its
// site domain (query param ?domain=) or container ID path segment.
func (h *Handlers) ExportSite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract container ID from path: /api/containers/<id>/export
	path := strings.TrimPrefix(r.URL.Path, "/api/containers/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "container id required")
		return
	}
	containerID := parts[0]

	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	if _, err := h.authorizeContainerAccess(ctx, r, containerID); err != nil {
		if err == errForbidden {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	site, err := h.store.GetSiteByContainerIDRecord(containerID)
	if err != nil || site == nil {
		writeError(w, http.StatusNotFound, "site not found for container")
		return
	}

	dklPath := filepath.Join(site.CodePath, dklFilename)
	dklData, err := os.ReadFile(dklPath)
	if err != nil {
		// Manifest missing — generate one on the fly from the site record.
		port := 3000
		_ = WriteDKLManifest(site.CodePath, site.Domain, site.TemplateType, "", port, false, nil)
		dklData, err = os.ReadFile(dklPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not read or generate manifest")
			return
		}
	}

	sanitized := strings.ReplaceAll(site.Domain, "/", "_")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.dklpkg"`, sanitized))

	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// 1. Write manifest.dkl
	if err := addBytesToTar(tw, dklpkgManifestEntry, dklData); err != nil {
		// Headers already sent; nothing we can do except log.
		return
	}

	// 2. Walk site files and add them under files/
	_ = filepath.Walk(site.CodePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Skip the manifest itself — it's already written above.
		if info.Name() == dklFilename {
			return nil
		}
		rel, err := filepath.Rel(site.CodePath, path)
		if err != nil {
			return nil
		}
		return addFileToTar(tw, path, dklpkgFilesPrefix+rel, info)
	})
}

// ImportSite accepts a multipart upload of a .dklpkg file, extracts it, and
// provisions the site (files + container) on this machine.
func (h *Handlers) ImportSite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImportSize)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse upload")
		return
	}

	file, _, err := r.FormFile("package")
	if err != nil {
		writeError(w, http.StatusBadRequest, "package field required")
		return
	}
	defer file.Close()

	// Extract to a temp directory first so we can validate before committing.
	tmpDir, err := os.MkdirTemp("", "dklpkg-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create temp dir")
		return
	}
	defer os.RemoveAll(tmpDir)

	manifest, err := extractDKLPkg(file, tmpDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid .dklpkg: %v", err))
		return
	}

	// Resolve or create target user.
	var targetUser interface{ GetID() int64 }
	userID := int64(0)
	if uid, err := readUserIDFromRequest(r); err == nil {
		userID = uid
	}

	var resolvedUserID int64
	if manifest.Username != "" {
		if u, _ := h.store.GetUserByUsername(manifest.Username); u != nil {
			resolvedUserID = u.ID
		}
	}
	if resolvedUserID == 0 && userID > 0 {
		resolvedUserID = userID
	}
	if resolvedUserID == 0 {
		resolvedUserID = 1
	}
	_ = targetUser

	resolvedUser, err := h.store.GetUserByIDFull(resolvedUserID)
	if err != nil || resolvedUser == nil {
		writeError(w, http.StatusInternalServerError, "could not resolve target user")
		return
	}

	// Move files into place.
	sitePath := getSitePath(resolvedUser.Username, manifest.Domain)
	if err := os.MkdirAll(sitePath, 0o775); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create site directory")
		return
	}
	chownDocklite(filepath.Join(siteBaseDir, resolvedUser.Username))
	chownDocklite(sitePath)

	filesDir := filepath.Join(tmpDir, "files")
	if info, err := os.Stat(filesDir); err == nil && info.IsDir() {
		if err := copyDir(filesDir, sitePath); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to copy site files")
			return
		}
	}

	// Write the manifest into the destination.
	_ = WriteDKLManifest(sitePath, manifest.Domain, manifest.TemplateType,
		resolvedUser.Username, manifest.Port, manifest.IncludeWww, manifest.Env)

	// Reuse onboard logic: create site record + container.
	onboardReq := &http.Request{
		Method: http.MethodPost,
		Header: r.Header,
	}
	onboardBody, _ := json.Marshal(map[string]any{
		"path":    sitePath,
		"user_id": resolvedUserID,
	})
	onboardReq.Body = io.NopCloser(strings.NewReader(string(onboardBody)))
	onboardReq = onboardReq.WithContext(r.Context())

	h.OnboardSite(w, onboardReq)
}

// ── tar helpers ───────────────────────────────────────────────────────────────

func addBytesToTar(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{
		Name: name,
		Mode: 0o644,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func addFileToTar(tw *tar.Writer, srcPath, tarName string, info os.FileInfo) error {
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = tarName

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}

func extractDKLPkg(r io.Reader, destDir string) (*dklManifest, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("not a valid gzip archive: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	var manifest *dklManifest

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Sanitise path to prevent directory traversal.
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") {
			continue
		}

		destPath := filepath.Join(destDir, clean)

		switch hdr.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(destPath, 0o775)

		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(destPath), 0o775); err != nil {
				return nil, err
			}
			f, err := os.Create(destPath)
			if err != nil {
				return nil, err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return nil, err
			}
			f.Close()

			if hdr.Name == dklpkgManifestEntry {
				data, err := os.ReadFile(destPath)
				if err != nil {
					return nil, err
				}
				var m dklManifest
				if err := json.Unmarshal(data, &m); err != nil {
					return nil, fmt.Errorf("invalid manifest.dkl: %w", err)
				}
				manifest = &m
			}
		}
	}

	if manifest == nil {
		return nil, fmt.Errorf("archive is missing manifest.dkl")
	}
	return manifest, nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target)
	})
}

// readUserIDFromRequest pulls the authenticated user's ID from context.
func readUserIDFromRequest(r *http.Request) (int64, error) {
	id, ok := readUserIDFromContext(r)
	if !ok || id <= 0 {
		return 0, fmt.Errorf("no user in context")
	}
	return id, nil
}
