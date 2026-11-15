package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// TwoFactor is one user's authenticator-app setup. A row with Enabled false is a setup that was started (QR code shown)
// but not yet confirmed with a code; it does nothing at login.
type TwoFactor struct {
	UserID         int64
	Secret         string
	Enabled        bool
	RecoveryHashes []string // sha256 of each unused recovery code
	LastStep       int64    // the last 30-second step whose code was accepted; that code can't be used again
	EnabledAt      string
}

// EnsureTwoFactorSchema adds the user_two_factor table. Additive, so an older DockLite keeps working with the same file
// (it simply doesn't ask for codes). The GUI's migration 017 does the same; whichever runs first wins.
func (s *SQLiteStore) EnsureTwoFactorSchema() error {
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS user_two_factor (
			user_id INTEGER PRIMARY KEY,
			secret TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 0,
			recovery_hashes TEXT NOT NULL DEFAULT '[]',
			last_step INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			enabled_at TEXT
		)
	`)
	return err
}

// GetTwoFactor returns the user's row, or nil when they have never started setting it up.
func (s *SQLiteStore) GetTwoFactor(userID int64) (*TwoFactor, error) {
	var tf TwoFactor
	var enabled int
	var hashes string
	var enabledAt sql.NullString
	err := s.DB.QueryRow(`SELECT user_id, secret, enabled, recovery_hashes, last_step, enabled_at FROM user_two_factor WHERE user_id = ?`, userID).
		Scan(&tf.UserID, &tf.Secret, &enabled, &hashes, &tf.LastStep, &enabledAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	tf.Enabled = enabled == 1
	tf.EnabledAt = enabledAt.String
	_ = json.Unmarshal([]byte(hashes), &tf.RecoveryHashes)
	return &tf, nil
}

// StartTwoFactor stores a new, not-yet-confirmed secret, replacing an unfinished one. It refuses when 2FA is already on:
// turning it off first (with a code) is the only way to change the secret.
func (s *SQLiteStore) StartTwoFactor(userID int64, secret string) error {
	res, err := s.DB.Exec(`
		INSERT INTO user_two_factor (user_id, secret, enabled) VALUES (?, ?, 0)
		ON CONFLICT(user_id) DO UPDATE SET secret = excluded.secret, recovery_hashes = '[]', last_step = 0, created_at = CURRENT_TIMESTAMP
		WHERE user_two_factor.enabled = 0
	`, userID, secret)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("two-factor login is already on")
	}
	return nil
}

// EnableTwoFactor turns it on after the first good code, with that code's step and the new recovery codes' hashes.
func (s *SQLiteStore) EnableTwoFactor(userID, step int64, recoveryHashes []string) error {
	h, _ := json.Marshal(recoveryHashes)
	_, err := s.DB.Exec(`UPDATE user_two_factor SET enabled = 1, last_step = ?, recovery_hashes = ?, enabled_at = CURRENT_TIMESTAMP WHERE user_id = ?`,
		step, string(h), userID)
	return err
}

// MarkTwoFactorStep records that a step's code was used, so it can't be used again. Only moves forward.
func (s *SQLiteStore) MarkTwoFactorStep(userID, step int64) error {
	_, err := s.DB.Exec(`UPDATE user_two_factor SET last_step = ? WHERE user_id = ? AND last_step < ?`, step, userID, step)
	return err
}

// SetRecoveryHashes replaces the unused recovery codes (after one is used, or when new ones are made).
func (s *SQLiteStore) SetRecoveryHashes(userID int64, hashes []string) error {
	if hashes == nil {
		hashes = []string{}
	}
	h, _ := json.Marshal(hashes)
	_, err := s.DB.Exec(`UPDATE user_two_factor SET recovery_hashes = ? WHERE user_id = ?`, string(h), userID)
	return err
}

// DeleteTwoFactor turns it off and forgets the secret (the user turned it off, or an admin reset it).
func (s *SQLiteStore) DeleteTwoFactor(userID int64) error {
	_, err := s.DB.Exec(`DELETE FROM user_two_factor WHERE user_id = ?`, userID)
	return err
}
