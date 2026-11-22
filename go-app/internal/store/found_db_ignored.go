package store

// Databases DockLite found by itself (SQLite files inside a site) come back on the next scan if they are only deleted
// from the list, so "remove" also records the file here. Keyed by the site's container name, which survives the
// container being recreated (a transfer, an update), unlike its id.

func (s *SQLiteStore) EnsureSQLiteIgnoreSchema() error {
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS found_db_ignored (
			container_name TEXT NOT NULL,
			db_path TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (container_name, db_path)
		)`)
	return err
}

func (s *SQLiteStore) IgnoreSQLiteFile(containerName, dbPath string) error {
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO found_db_ignored (container_name, db_path) VALUES (?, ?)`, containerName, dbPath)
	return err
}

func (s *SQLiteStore) UnignoreSQLiteFile(containerName, dbPath string) error {
	_, err := s.DB.Exec(`DELETE FROM found_db_ignored WHERE container_name = ? AND db_path = ?`, containerName, dbPath)
	return err
}

// IgnoredSQLiteFiles is the set of "container name\x00path" keys the owner removed.
func (s *SQLiteStore) IgnoredSQLiteFiles() (map[string]bool, error) {
	rows, err := s.DB.Query(`SELECT container_name, db_path FROM found_db_ignored`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n, p string
		if err := rows.Scan(&n, &p); err != nil {
			return nil, err
		}
		out[n+"\x00"+p] = true
	}
	return out, rows.Err()
}
