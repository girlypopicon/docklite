package store

import (
	"database/sql"
	"errors"
	"strings"
)

// Groups replace folders. A group is one card on the dashboard that holds boxes. It is flat (no groups inside groups) and a
// box is in at most one group. `Linked` is the switch between the two uses: off, the group only keeps things tidy; on, its
// boxes are meant to talk to each other (a website and its database). Boxes are named by container name, not id, so a box
// stays in its group when it is recreated. All additions: an older DockLite ignores these tables.

type BoxGroup struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	OwnerID   int64    `json:"owner_id"`
	Linked    bool     `json:"linked"`
	Color     string   `json:"color,omitempty"`
	Icon      string   `json:"icon,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
	Members   []string `json:"members"` // container names, in order
}

var ErrGroupNameTaken = errors.New("you already have a group with that name")

func (s *SQLiteStore) EnsureGroupSchema() error {
	if _, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS box_groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			owner_id INTEGER NOT NULL,
			linked INTEGER NOT NULL DEFAULT 0,
			color TEXT NOT NULL DEFAULT '',
			icon TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS box_group_members (
			container_name TEXT PRIMARY KEY,
			group_id INTEGER NOT NULL,
			position INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_box_group_members_group ON box_group_members(group_id)`)
	return err
}

// ListGroups returns every group with its members. ownerID 0 means all owners (an admin's view).
func (s *SQLiteStore) ListGroups(ownerID int64) ([]BoxGroup, error) {
	q := `SELECT id, name, owner_id, linked, color, icon, created_at FROM box_groups`
	var args []any
	if ownerID > 0 {
		q += ` WHERE owner_id = ?`
		args = append(args, ownerID)
	}
	q += ` ORDER BY name COLLATE NOCASE, id`
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	var groups []BoxGroup
	byID := map[int64]int{}
	for rows.Next() {
		var g BoxGroup
		var linked int
		if err := rows.Scan(&g.ID, &g.Name, &g.OwnerID, &linked, &g.Color, &g.Icon, &g.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		g.Linked = linked == 1
		g.Members = []string{}
		byID[g.ID] = len(groups)
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	mrows, err := s.DB.Query(`SELECT group_id, container_name FROM box_group_members ORDER BY group_id, position, container_name`)
	if err != nil {
		return nil, err
	}
	defer mrows.Close()
	for mrows.Next() {
		var gid int64
		var name string
		if err := mrows.Scan(&gid, &name); err != nil {
			return nil, err
		}
		if i, ok := byID[gid]; ok {
			groups[i].Members = append(groups[i].Members, name)
		}
	}
	return groups, mrows.Err()
}

func (s *SQLiteStore) GetGroup(id int64) (*BoxGroup, error) {
	groups, err := s.ListGroups(0)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		if groups[i].ID == id {
			return &groups[i], nil
		}
	}
	return nil, nil
}

// GroupOfBox returns the group a box is in, or nil.
func (s *SQLiteStore) GroupOfBox(containerName string) (*BoxGroup, error) {
	var gid int64
	err := s.DB.QueryRow(`SELECT group_id FROM box_group_members WHERE container_name = ?`, containerName).Scan(&gid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.GetGroup(gid)
}

// CreateGroup makes a group owned by ownerID. Names are unique per owner, ignoring case.
func (s *SQLiteStore) CreateGroup(ownerID int64, name string) (*BoxGroup, error) {
	name = strings.TrimSpace(name)
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM box_groups WHERE owner_id = ? AND LOWER(name) = LOWER(?)`, ownerID, name).Scan(&n); err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, ErrGroupNameTaken
	}
	res, err := s.DB.Exec(`INSERT INTO box_groups (name, owner_id) VALUES (?, ?)`, name, ownerID)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetGroup(id)
}

// UpdateGroup changes the fields that are non-nil.
func (s *SQLiteStore) UpdateGroup(id int64, name *string, linked *bool, color, icon *string) error {
	g, err := s.GetGroup(id)
	if err != nil || g == nil {
		if err == nil {
			err = sql.ErrNoRows
		}
		return err
	}
	if name != nil {
		n := strings.TrimSpace(*name)
		var c int
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM box_groups WHERE owner_id = ? AND LOWER(name) = LOWER(?) AND id <> ?`, g.OwnerID, n, id).Scan(&c); err != nil {
			return err
		}
		if c > 0 {
			return ErrGroupNameTaken
		}
		g.Name = n
	}
	if linked != nil {
		g.Linked = *linked
	}
	if color != nil {
		g.Color = *color
	}
	if icon != nil {
		g.Icon = *icon
	}
	l := 0
	if g.Linked {
		l = 1
	}
	_, err = s.DB.Exec(`UPDATE box_groups SET name = ?, linked = ?, color = ?, icon = ? WHERE id = ?`, g.Name, l, g.Color, g.Icon, id)
	return err
}

// DeleteGroup removes a group; its boxes go back to the desktop (they are not deleted).
func (s *SQLiteStore) DeleteGroup(id int64) error {
	if _, err := s.DB.Exec(`DELETE FROM box_group_members WHERE group_id = ?`, id); err != nil {
		return err
	}
	_, err := s.DB.Exec(`DELETE FROM box_groups WHERE id = ?`, id)
	return err
}

// AddToGroup puts a box in a group, taking it out of any other group first.
func (s *SQLiteStore) AddToGroup(groupID int64, containerName string) error {
	var pos int
	_ = s.DB.QueryRow(`SELECT COALESCE(MAX(position), -1) + 1 FROM box_group_members WHERE group_id = ?`, groupID).Scan(&pos)
	_, err := s.DB.Exec(`
		INSERT INTO box_group_members (container_name, group_id, position) VALUES (?, ?, ?)
		ON CONFLICT(container_name) DO UPDATE SET group_id = excluded.group_id, position = excluded.position
	`, containerName, groupID, pos)
	return err
}

// RemoveFromGroup takes a box out of whatever group it is in (back to the desktop).
func (s *SQLiteStore) RemoveFromGroup(containerName string) error {
	_, err := s.DB.Exec(`DELETE FROM box_group_members WHERE container_name = ?`, containerName)
	return err
}
