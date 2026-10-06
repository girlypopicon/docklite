package backup

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"docklite-agent/internal/docker"
	"docklite-agent/internal/store"
)

const manifestExt = ".manifest.json"

var filenameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type ManifestFile struct {
	Name   string `json:"name"`
	Bytes  int64  `json:"bytes"`
	Sha256 string `json:"sha256"`
}

type ManifestSource struct {
	SiteID      *int64 `json:"site_id,omitempty"`
	Domain      string `json:"domain,omitempty"`
	DatabaseID  *int64 `json:"database_id,omitempty"`
	Name        string `json:"name,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
}

type Manifest struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	CreatedAt   string         `json:"created_at"`
	Source      ManifestSource `json:"source"`
	Files       []ManifestFile `json:"files"`
	ToolVersion string         `json:"tool_version,omitempty"`
	Notes       string         `json:"notes,omitempty"`
}

type ArtifactResult struct {
	Verified     bool
	Path         string
	Size         int64
	Sha256       string
	ManifestPath string
	RelativePath string
}

func CreateSiteBackup(ctx context.Context, storeHandle *store.SQLiteStore, baseDir string, subDir string, siteID int64, notes string) (*ArtifactResult, error) {
	site, err := storeHandle.GetSiteByID(siteID)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, fmt.Errorf("site %d not found", siteID)
	}
	if site.CodePath == "" {
		return nil, fmt.Errorf("site path not set")
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	domainSlug := sanitizeFilename(site.Domain)
	filename := fmt.Sprintf("site-%s-%s.tar.gz", domainSlug, timestamp)

	destDir := filepath.Join(baseDir, subDir)
	tmpDir := filepath.Join(baseDir, "_tmp")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, err
	}

	tempPath := filepath.Join(tmpDir, filename+".partial")
	finalPath := filepath.Join(destDir, filename)

	rep := reporterFrom(ctx)
	entries, err := writeSiteArchive(ctx, site.CodePath, tempPath, rep)
	if err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	// Check the file before it gets its real name, so a bad backup never
	// shows up in the list looking like a good one.
	sha, size, err := verifySiteArchive(tempPath, entries, rep)
	if err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}

	manifest := Manifest{
		ID:        generateID(),
		Type:      "site_backup",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Source: ManifestSource{
			SiteID: &site.ID,
			Domain: site.Domain,
		},
		Files: []ManifestFile{{
			Name:   filepath.Base(finalPath),
			Bytes:  size,
			Sha256: sha,
		}},
		Notes: notes,
	}

	manifestPath, err := writeManifest(tmpDir, destDir, filename, manifest)
	if err != nil {
		return nil, err
	}

	relative, _ := filepath.Rel(baseDir, finalPath)
	return &ArtifactResult{
		Verified:     true,
		Path:         finalPath,
		Size:         size,
		Sha256:       sha,
		ManifestPath: manifestPath,
		RelativePath: relative,
	}, nil
}

func CreateDatabaseBackup(ctx context.Context, storeHandle *store.SQLiteStore, dockerClient *docker.Client, baseDir string, subDir string, databaseID int64, notes string) (*ArtifactResult, error) {
	database, err := storeHandle.GetDatabaseByID(databaseID)
	if err != nil {
		return nil, err
	}
	if database == nil {
		return nil, fmt.Errorf("database %d not found", databaseID)
	}
	if database.ContainerID == "" {
		return nil, fmt.Errorf("database container not found")
	}

	source := ManifestSource{
		DatabaseID:  &database.ID,
		Name:        database.Name,
		ContainerID: database.ContainerID,
	}

	username, password, err := resolveDatabaseCredentials(ctx, dockerClient, database.ContainerID)
	if err != nil {
		return nil, err
	}

	return CreateDatabaseExport(ctx, dockerClient, baseDir, subDir, database.Name, database.ContainerID, username, password, source, notes)
}

func CreateDatabaseExport(ctx context.Context, dockerClient *docker.Client, baseDir string, subDir string, dbName string, containerID string, username string, password *string, source ManifestSource, notes string) (*ArtifactResult, error) {
	timestamp := time.Now().UTC().Format("20060102-150405")
	dbSlug := sanitizeFilename(dbName)
	filename := fmt.Sprintf("db-%s-%s.dump.gz", dbSlug, timestamp)

	destDir := filepath.Join(baseDir, subDir)
	tmpDir := filepath.Join(baseDir, "_tmp")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, err
	}

	tempPath := filepath.Join(tmpDir, filename+".partial")
	finalPath := filepath.Join(destDir, filename)

	rep := reporterFrom(ctx)
	rep.Phase(kindWorking, "Exporting the database", 0)

	file, err := os.Create(tempPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)

	cmd := []string{"pg_dump", "-U", username, "-d", dbName, "-F", "c"}
	var env []string
	if password != nil && *password != "" {
		env = append(env, fmt.Sprintf("PGPASSWORD=%s", *password))
	}
	if err := dockerClient.ExecCommandToWriter(ctx, containerID, cmd, env, progressWriter{w: gzipWriter, rep: rep}); err != nil {
		_ = gzipWriter.Close()
		_ = os.Remove(tempPath)
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	if err := file.Sync(); err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	sha, size, err := verifyDatabaseDump(tempPath, rep)
	if err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}

	manifest := Manifest{
		ID:        generateID(),
		Type:      "db_export",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Source:    source,
		Files: []ManifestFile{{
			Name:   filepath.Base(finalPath),
			Bytes:  size,
			Sha256: sha,
		}},
		Notes: notes,
	}

	manifestPath, err := writeManifest(tmpDir, destDir, filename, manifest)
	if err != nil {
		return nil, err
	}

	relative, _ := filepath.Rel(baseDir, finalPath)
	return &ArtifactResult{
		Verified:     true,
		Path:         finalPath,
		Size:         size,
		Sha256:       sha,
		ManifestPath: manifestPath,
		RelativePath: relative,
	}, nil
}

func resolveDatabaseCredentials(ctx context.Context, dockerClient *docker.Client, containerID string) (string, *string, error) {
	inspect, err := dockerClient.Client.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", nil, err
	}

	envMap := make(map[string]string)
	for _, entry := range inspect.Config.Env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	labels := inspect.Config.Labels
	username := envMap["POSTGRES_USER"]
	password := envMap["POSTGRES_PASSWORD"]

	if username == "" {
		username = labels["docklite.username"]
	}
	if password == "" {
		password = labels["docklite.password"]
	}
	if username == "" {
		username = "postgres"
	}
	var passwordPtr *string
	if password != "" {
		passwordPtr = &password
	}
	return username, passwordPtr, nil
}

func ManifestPathForArtifact(artifactPath string) string {
	base := artifactPath
	switch {
	case strings.HasSuffix(base, ".tar.gz"):
		base = strings.TrimSuffix(base, ".tar.gz")
	case strings.HasSuffix(base, ".dump.gz"):
		base = strings.TrimSuffix(base, ".dump.gz")
	default:
		ext := filepath.Ext(base)
		base = strings.TrimSuffix(base, ext)
	}
	return base + manifestExt
}

func sanitizeFilename(value string) string {
	sanitized := filenameSanitizer.ReplaceAllString(value, "_")
	sanitized = strings.Trim(sanitized, "._-")
	if sanitized == "" {
		return "backup"
	}
	return sanitized
}

func SanitizeFilename(value string) string {
	return sanitizeFilename(value)
}

func hashFile(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()

	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(hasher.Sum(nil)), nil
}

func writeManifest(tmpDir string, destDir string, filename string, manifest Manifest) (string, error) {
	manifestFile := ManifestPathForArtifact(filepath.Join(destDir, filename))
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	tempPath := filepath.Join(tmpDir, filepath.Base(manifestFile)+".partial")
	if err := os.WriteFile(tempPath, payload, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tempPath, manifestFile); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	return manifestFile, nil
}

func generateID() string {
	seed := make([]byte, 16)
	if _, err := rand.Read(seed); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(seed)
}
