package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// progressReader reports how many bytes pass through it.
type progressReader struct {
	r   io.Reader
	rep *Reporter
}

func (p progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.rep.Add(int64(n))
	}
	return n, err
}

// progressWriter reports how many bytes were written through it.
type progressWriter struct {
	w   io.Writer
	rep *Reporter
}

func (p progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	if n > 0 {
		p.rep.Add(int64(n))
	}
	return n, err
}

// writeSiteArchive packs srcDir into a .tar.gz at dest. Entries are named
// "<dirname>/..." (relative to srcDir's parent) so the archive unpacks into
// a single folder, exactly like `tar -C parent dirname` did before.
//
// Files that can't be read (permissions) are skipped with a warning rather
// than failing the whole backup; special files (sockets, devices) are
// skipped silently. Symlinks are stored as links, never followed.
// It returns the number of entries written.
func writeSiteArchive(ctx context.Context, srcDir, dest string, rep *Reporter) (int, error) {
	info, err := os.Stat(srcDir)
	if err != nil {
		return 0, fmt.Errorf("site folder not found: %s", srcDir)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("site path is not a folder: %s", srcDir)
	}

	rep.Phase(kindCounting, "Counting files", 0)
	var total int64
	_ = filepath.WalkDir(srcDir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if fi, err := d.Info(); err == nil {
				total += fi.Size()
			}
		}
		return nil
	})

	rep.Phase(kindWorking, "Compressing files", total)
	out, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)

	parent := filepath.Dir(srcDir)
	entries := 0

	walkErr := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if err != nil {
			rep.Warn(fmt.Sprintf("Skipped %s: %v", relOrPath(parent, path), unwrapPathError(err)))
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			rep.Warn(fmt.Sprintf("Skipped %s: %v", relOrPath(parent, path), unwrapPathError(err)))
			return nil
		}
		mode := fi.Mode()
		if !(mode.IsRegular() || mode.IsDir() || mode&fs.ModeSymlink != 0) {
			return nil // sockets, devices, pipes
		}
		rel, err := filepath.Rel(parent, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)

		link := ""
		if mode&fs.ModeSymlink != 0 {
			if link, err = os.Readlink(path); err != nil {
				rep.Warn(fmt.Sprintf("Skipped link %s: %v", name, unwrapPathError(err)))
				return nil
			}
		}
		hdr, err := tar.FileInfoHeader(fi, link)
		if err != nil {
			rep.Warn(fmt.Sprintf("Skipped %s: %v", name, err))
			return nil
		}
		hdr.Name = name
		if mode.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}

		var src *os.File
		if mode.IsRegular() {
			// Open before writing the header so an unreadable file leaves
			// no half-written entry behind.
			if src, err = os.Open(path); err != nil {
				rep.Warn(fmt.Sprintf("Skipped %s: %v", name, unwrapPathError(err)))
				return nil
			}
			defer src.Close()
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		entries++
		if src == nil {
			return nil
		}

		// Copy exactly the size recorded in the header. A file that grew
		// meanwhile (a log) is cut at that size; one that shrank is padded
		// with zeros so the archive stays valid, with a warning.
		written, err := io.CopyN(tw, progressReader{r: src, rep: rep}, hdr.Size)
		if err != nil && err != io.EOF {
			return err
		}
		if written < hdr.Size {
			rep.Warn(fmt.Sprintf("%s changed while it was being backed up", name))
			if _, err := io.CopyN(tw, zeroReader{}, hdr.Size-written); err != nil {
				return err
			}
		}
		return nil
	})
	if walkErr != nil {
		return 0, walkErr
	}

	rep.SetFiles(entries)
	if err := tw.Close(); err != nil {
		return 0, err
	}
	if err := gz.Close(); err != nil {
		return 0, err
	}
	if err := out.Sync(); err != nil {
		return 0, err
	}
	return entries, out.Close()
}

type zeroReader struct{}

func (zeroReader) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = 0
	}
	return len(b), nil
}

func relOrPath(parent, path string) string {
	if rel, err := filepath.Rel(parent, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return path
}

func unwrapPathError(err error) error {
	if pe, ok := err.(*fs.PathError); ok {
		return pe.Err
	}
	return err
}

// verifySiteArchive reads a finished .tar.gz back from start to end: the
// gzip checksum, every tar entry and its contents must be readable, and the
// entry count must match what was written. It returns the file's SHA-256.
func verifySiteArchive(path string, wantEntries int, rep *Reporter) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	rep.Phase(kindVerifying, "Checking the backup", fi.Size())

	hasher := sha256.New()
	counted := progressReader{r: io.TeeReader(f, hasher), rep: rep}
	gz, err := gzip.NewReader(counted)
	if err != nil {
		return "", 0, fmt.Errorf("the backup file isn't a valid archive: %w", err)
	}
	tr := tar.NewReader(gz)
	entries := 0
	for {
		_, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", 0, fmt.Errorf("the backup file is damaged: %w", err)
		}
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return "", 0, fmt.Errorf("the backup file is damaged: %w", err)
		}
		entries++
	}
	// Drain anything after the tar trailer so the hash covers the whole file.
	if _, err := io.Copy(io.Discard, counted); err != nil {
		return "", 0, fmt.Errorf("the backup file is damaged: %w", err)
	}
	if entries != wantEntries {
		return "", 0, fmt.Errorf("the backup is incomplete: %d of %d files made it into the archive", entries, wantEntries)
	}
	return hex.EncodeToString(hasher.Sum(nil)), fi.Size(), nil
}

// verifyDatabaseDump reads a gzipped pg_dump (custom format) back and
// checks it starts with the PGDMP signature, so an empty or truncated dump
// can't be reported as a good backup. It returns the file's SHA-256.
func verifyDatabaseDump(path string, rep *Reporter) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	rep.Phase(kindVerifying, "Checking the backup", fi.Size())

	hasher := sha256.New()
	counted := progressReader{r: io.TeeReader(f, hasher), rep: rep}
	gz, err := gzip.NewReader(counted)
	if err != nil {
		return "", 0, fmt.Errorf("the backup file isn't a valid archive: %w", err)
	}
	head := make([]byte, 5)
	if _, err := io.ReadFull(gz, head); err != nil || !bytes.Equal(head, []byte("PGDMP")) {
		return "", 0, fmt.Errorf("the dump is empty or isn't a PostgreSQL backup")
	}
	if _, err := io.Copy(io.Discard, gz); err != nil {
		return "", 0, fmt.Errorf("the backup file is damaged: %w", err)
	}
	if _, err := io.Copy(io.Discard, counted); err != nil {
		return "", 0, fmt.Errorf("the backup file is damaged: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), fi.Size(), nil
}
