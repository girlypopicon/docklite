package store

// What a box (a container) looks like on the dashboard: the colour of its card and the big icon behind it. Keyed by the
// container's name so it survives the box being recreated (its id changes, its name doesn't). Cosmetic and shared: everyone
// who can see a box sees the same card.

type BoxStyle struct {
	Color string `json:"color,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

func (s *SQLiteStore) EnsureBoxStyleSchema() error {
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS box_styles (
			container_name TEXT PRIMARY KEY,
			color TEXT NOT NULL DEFAULT '',
			icon TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// ListBoxStyles returns every saved style by container name (one query, for building the whole list).
func (s *SQLiteStore) ListBoxStyles() (map[string]BoxStyle, error) {
	rows, err := s.DB.Query(`SELECT container_name, color, icon FROM box_styles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]BoxStyle{}
	for rows.Next() {
		var name string
		var st BoxStyle
		if err := rows.Scan(&name, &st.Color, &st.Icon); err != nil {
			return nil, err
		}
		out[name] = st
	}
	return out, rows.Err()
}

// SetBoxStyle saves a box's style; empty colour and icon (the defaults) remove the row.
func (s *SQLiteStore) SetBoxStyle(name string, st BoxStyle) error {
	if st.Color == "" && st.Icon == "" {
		_, err := s.DB.Exec(`DELETE FROM box_styles WHERE container_name = ?`, name)
		return err
	}
	_, err := s.DB.Exec(`
		INSERT INTO box_styles (container_name, color, icon, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(container_name) DO UPDATE SET color = excluded.color, icon = excluded.icon, updated_at = CURRENT_TIMESTAMP
	`, name, st.Color, st.Icon)
	return err
}
