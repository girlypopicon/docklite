package store

import (
	"database/sql"
	"errors"
)

// GetUserAppearance returns the user's saved appearance as JSON, or "" if they never chose one.
func (s *SQLiteStore) GetUserAppearance(userID int64) (string, error) {
	var appearance sql.NullString
	err := s.DB.QueryRow(`SELECT appearance FROM user_preferences WHERE user_id = ?`, userID).Scan(&appearance)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return appearance.String, err
}

// SetUserAppearance saves the user's appearance (JSON).
func (s *SQLiteStore) SetUserAppearance(userID int64, appearance string) error {
	_, err := s.DB.Exec(`
		INSERT INTO user_preferences (user_id, appearance, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET appearance = excluded.appearance, updated_at = CURRENT_TIMESTAMP
	`, userID, appearance)
	return err
}
