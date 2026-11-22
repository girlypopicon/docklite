package store

import (
	"database/sql"
	"errors"
	"strings"
)

// Other DockLite servers this one (the hub) shows next to itself. Each is an address and a token that server made for
// the hub; the token is what lets the hub ask that server's agent for things, so it never leaves this database.
// Additive: an older DockLite ignores the table.

type RemoteServer struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Token     string `json:"-"` // never sent to the browser
	CreatedAt string `json:"created_at"`
}

var ErrServerNameTaken = errors.New("a server with that name is already connected")

func (s *SQLiteStore) EnsureRemoteServerSchema() error {
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS remote_servers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			token TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	return err
}

func (s *SQLiteStore) ListRemoteServers() ([]RemoteServer, error) {
	rows, err := s.DB.Query(`SELECT id, name, url, token, created_at FROM remote_servers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RemoteServer{}
	for rows.Next() {
		var r RemoteServer
		if err := rows.Scan(&r.ID, &r.Name, &r.URL, &r.Token, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) GetRemoteServer(id int64) (*RemoteServer, error) {
	var r RemoteServer
	err := s.DB.QueryRow(`SELECT id, name, url, token, created_at FROM remote_servers WHERE id = ?`, id).
		Scan(&r.ID, &r.Name, &r.URL, &r.Token, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &r, err
}

func (s *SQLiteStore) AddRemoteServer(name, url, token string) (*RemoteServer, error) {
	name = strings.TrimSpace(name)
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM remote_servers WHERE LOWER(name) = LOWER(?)`, name).Scan(&n); err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, ErrServerNameTaken
	}
	res, err := s.DB.Exec(`INSERT INTO remote_servers (name, url, token) VALUES (?, ?, ?)`, name, strings.TrimRight(url, "/"), token)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetRemoteServer(id)
}

func (s *SQLiteStore) DeleteRemoteServer(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM remote_servers WHERE id = ?`, id)
	return err
}
