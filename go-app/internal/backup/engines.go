package backup

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// DumpCommand is the command that writes a complete dump of one database to its standard output, run inside the
// database's container, and the environment it needs. Passwords go in the environment, not on the command line.
func DumpCommand(engine, dbName, user, pass string) (cmd []string, env []string, err error) {
	switch engine {
	case "", "postgres":
		env = nil
		if pass != "" {
			env = []string{"PGPASSWORD=" + pass}
		}
		return []string{"pg_dump", "-U", user, "-d", dbName, "-F", "c"}, env, nil
	case "mariadb":
		return []string{"mariadb-dump", "--single-transaction", "--routines", "--triggers", "-u", user, dbName}, []string{"MYSQL_PWD=" + pass}, nil
	case "mongodb":
		return []string{"mongodump", "--archive", "-u", user, "-p", pass, "--authenticationDatabase", "admin", "--db", dbName}, nil, nil
	case "redis":
		// SAVE writes a snapshot to /data/dump.rdb; the snapshot is then streamed out.
		return []string{"sh", "-c", "redis-cli SAVE >/dev/null && cat /data/dump.rdb"}, []string{"REDISCLI_AUTH=" + pass}, nil
	}
	return nil, nil, fmt.Errorf("no backup method for database type %q", engine)
}

// verifyDatabaseDumpOfType reads a gzipped dump back from start to end and checks it looks like a complete dump of
// its engine, so an empty or cut-off dump can never be reported as a good backup. It returns the file's SHA-256.
func verifyDatabaseDumpOfType(path, engine string, rep *Reporter) (string, int64, error) {
	if engine == "" || engine == "postgres" {
		return verifyDatabaseDump(path, rep)
	}
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
	head := make([]byte, 8)
	n, _ := io.ReadFull(gz, head)
	head = head[:n]
	tail := &tailBuffer{max: 4096}
	tail.Write(head)
	if _, err := io.Copy(tail, gz); err != nil {
		return "", 0, fmt.Errorf("the backup file is damaged: %w", err)
	}
	if _, err := io.Copy(io.Discard, counted); err != nil {
		return "", 0, fmt.Errorf("the backup file is damaged: %w", err)
	}
	if err := checkDumpShape(engine, head, tail.Bytes()); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hasher.Sum(nil)), fi.Size(), nil
}

// checkDumpShape recognises each engine's dump by its first and last bytes.
func checkDumpShape(engine string, head, tail []byte) error {
	switch engine {
	case "mariadb":
		// mariadb-dump ends with "-- Dump completed on <date>" when it finished; a cut-off dump lacks it.
		if len(head) == 0 || !bytes.Contains(tail, []byte("-- Dump completed")) {
			return fmt.Errorf("the dump is empty or was cut off, so it isn't a complete MariaDB backup")
		}
	case "mongodb":
		// every mongodump archive starts with the magic number 0x8199e26d (little endian)
		if len(head) < 4 || !bytes.Equal(head[:4], []byte{0x6d, 0xe2, 0x99, 0x81}) {
			return fmt.Errorf("the dump is empty or isn't a MongoDB backup")
		}
	case "redis":
		if len(head) < 5 || !bytes.Equal(head[:5], []byte("REDIS")) {
			return fmt.Errorf("the dump is empty or isn't a Redis snapshot")
		}
	default:
		return fmt.Errorf("no way to check a %q backup", engine)
	}
	return nil
}

// tailBuffer keeps only the last max bytes written to it.
type tailBuffer struct {
	max int
	buf []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *tailBuffer) Bytes() []byte { return t.buf }
