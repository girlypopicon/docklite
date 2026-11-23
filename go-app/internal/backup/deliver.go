package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"docklite-agent/internal/store"
)

// outboxDir is where a package is built when it is going to another destination: a work folder that the backups list
// never shows, emptied again once the copy is safely there.
const outboxDir = "_outbox"

// Deliver finishes a package made at res.Path for a destination: locks it if the destination asks for that, then, for a
// remote destination, copies it there and removes the local copy. It updates res to describe what ended up where, and
// returns the path or reference to save with the backup. On any failure the local files are removed.
func Deliver(ctx context.Context, st *store.SQLiteStore, dest store.BackupDestination, res *PackageResult, subDir string) (string, error) {
	cfg := ParseDestConfig(dest.Config)
	rep := reporterFrom(ctx)
	cleanup := func(paths ...string) {
		for _, p := range paths {
			_ = os.Remove(p)
		}
	}

	if cfg.Encrypt {
		if err := ctx.Err(); err != nil {
			cleanup(res.Path, res.ManifestPath)
			return "", err
		}
		rep.Phase(kindWorking, "Locking the backup with your key", 0)
		enc := res.Path + EncryptedExt
		size, sum, err := EncryptFile(ctx, res.Path, enc, cfg.Recipient)
		if err != nil {
			cleanup(res.Path, res.ManifestPath)
			return "", fmt.Errorf("couldn't lock the backup: %w", err)
		}
		cleanup(res.Path, res.ManifestPath)
		side := Manifest{
			ID: generateID(), Type: "package", CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Files: []ManifestFile{{Name: filepath.Base(enc), Bytes: size, Sha256: sum}},
			Notes: "encrypted with age; open it with `age -d -i <key file>`",
		}
		mp, err := writeManifest(filepath.Dir(enc), filepath.Dir(enc), filepath.Base(enc), side)
		if err != nil {
			cleanup(enc)
			return "", err
		}
		res.Path, res.Size, res.Sha256, res.ManifestPath = enc, size, sum, mp
	}

	if dest.Type == "local" {
		return res.Path, nil
	}

	remote, err := OpenRemote(st, dest)
	if err != nil {
		cleanup(res.Path, res.ManifestPath)
		return "", err
	}
	rep.Phase(kindWorking, "Sending to "+dest.Name, res.Size)
	// Nothing in the outbox tells two runs in the same second apart, so a short random part keeps one from replacing
	// the other at the destination (and pruning the old one from deleting the new one).
	base := filepath.Base(res.Path)
	ext := ".litpkg" + EncryptedExt
	if !cfg.Encrypt {
		ext = filepath.Ext(base)
	}
	base = strings.TrimSuffix(base, ext) + "-" + generateID()[:6] + ext
	name := strings.Trim(subDir, "/") + "/" + base
	ref, err := remote.Put(ctx, res.Path, name)
	cleanup(res.Path, res.ManifestPath)
	if err != nil {
		return "", fmt.Errorf("couldn't send the backup to %s: %w", dest.Name, err)
	}
	res.Path, res.ManifestPath, res.RelativePath = ref, "", ""
	return ref, nil
}

// RemoveRecordArtifacts deletes the backup a history row points at, wherever it is. A remote copy that can't be deleted
// is an error, so the caller keeps the row and tries again later instead of forgetting a file that still exists.
func RemoveRecordArtifacts(ctx context.Context, st *store.SQLiteStore, rec store.BackupRecord) error {
	if !IsRemoteRef(rec.BackupPath) {
		return RemoveBackupArtifacts(rec.BackupPath)
	}
	dest, err := st.GetBackupDestinationByID(rec.Destination)
	if err != nil {
		return err
	}
	if dest == nil {
		return nil // the destination was removed, and with it the way to reach the copy
	}
	remote, err := OpenRemote(st, *dest)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return remote.Delete(ctx, rec.BackupPath)
}

// FetchRecord brings a backup to this server as a plain .litpkg (or .tar.gz) ready to inspect and restore: it copies a
// remote one down and unlocks an encrypted one with the key that was pasted. The result goes in the downloads folder
// and is the caller's to remove.
func FetchRecord(ctx context.Context, st *store.SQLiteStore, baseDir string, rec store.BackupRecord, identity string) (string, error) {
	dir := filepath.Join(baseDir, "downloads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := filepath.Base(strings.TrimRight(rec.BackupPath, "/"))
	local := filepath.Join(dir, fmt.Sprintf("fetched-%d-%s", rec.ID, SanitizeFilename(name)))
	if IsRemoteRef(rec.BackupPath) {
		dest, err := st.GetBackupDestinationByID(rec.Destination)
		if err != nil || dest == nil {
			return "", fmt.Errorf("the destination this backup was sent to is gone")
		}
		remote, err := OpenRemote(st, *dest)
		if err != nil {
			return "", err
		}
		if err := remote.Get(ctx, rec.BackupPath, local); err != nil {
			return "", err
		}
	} else {
		in, err := os.ReadFile(rec.BackupPath)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(local, in, 0o600); err != nil {
			return "", err
		}
	}
	if !IsEncrypted(local) {
		return local, nil
	}
	if strings.TrimSpace(identity) == "" {
		_ = os.Remove(local)
		return "", fmt.Errorf("this backup is locked; paste the backup key that was shown when encryption was turned on")
	}
	plain := strings.TrimSuffix(local, EncryptedExt)
	err := DecryptFile(ctx, local, plain, identity)
	_ = os.Remove(local)
	if err != nil {
		return "", err
	}
	return plain, nil
}
