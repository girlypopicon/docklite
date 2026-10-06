package store

import (
	"database/sql"
	"time"
)

type TokenRecord struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	TokenHash        string  `json:"token_hash"`
	TokenFingerprint string  `json:"token_fingerprint"`
	CreatedAt        string  `json:"created_at"`
	LastUsedAt       *string `json:"last_used_at"`
	ExpiresAt        *string `json:"expires_at"`
	Scopes           *string `json:"scopes"`
	Disabled         int     `json:"disabled"`
	RevokedAt        *string `json:"revoked_at"`
	IssuedFor        *string `json:"issued_for"`
	UserID           *int64  `json:"user_id"`
	Role             *string `json:"role"`
}

func (s *SQLiteStore) CreateToken(record TokenRecord) (*TokenRecord, error) {
	result, err := s.DB.Exec(`
    INSERT INTO tokens (name, token_hash, token_fingerprint, user_id, role, expires_at, scopes, issued_for, disabled)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
  `, record.Name, record.TokenHash, record.TokenFingerprint, record.UserID, record.Role, record.ExpiresAt, record.Scopes, record.IssuedFor, record.Disabled)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetTokenByID(id)
}

func (s *SQLiteStore) GetTokenByID(id int64) (*TokenRecord, error) {
	row := s.DB.QueryRow(`
    SELECT id, name, token_hash, token_fingerprint, created_at, last_used_at, expires_at, scopes, disabled, revoked_at, issued_for, user_id, role
    FROM tokens
    WHERE id = ?
  `, id)
	return scanToken(row)
}

func (s *SQLiteStore) GetTokenByFingerprint(fingerprint string) (*TokenRecord, error) {
	row := s.DB.QueryRow(`
    SELECT id, name, token_hash, token_fingerprint, created_at, last_used_at, expires_at, scopes, disabled, revoked_at, issued_for, user_id, role
    FROM tokens
    WHERE token_fingerprint = ?
  `, fingerprint)
	return scanToken(row)
}

func (s *SQLiteStore) ListTokens() ([]TokenRecord, error) {
	rows, err := s.DB.Query(`
    SELECT id, name, token_hash, token_fingerprint, created_at, last_used_at, expires_at, scopes, disabled, revoked_at, issued_for, user_id, role
    FROM tokens
    ORDER BY created_at DESC
  `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TokenRecord
	for rows.Next() {
		record, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *record)
	}
	return results, rows.Err()
}

func (s *SQLiteStore) UpdateTokenLastUsed(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec(`UPDATE tokens SET last_used_at = ? WHERE id = ?`, now, id)
	return err
}

func (s *SQLiteStore) RevokeToken(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec(`UPDATE tokens SET revoked_at = ?, disabled = 1 WHERE id = ?`, now, id)
	return err
}

func scanToken(scanner interface {
	Scan(dest ...any) error
}) (*TokenRecord, error) {
	var record TokenRecord
	var lastUsed sql.NullString
	var expires sql.NullString
	var scopes sql.NullString
	var revoked sql.NullString
	var issuedFor sql.NullString
	var userID sql.NullInt64
	var role sql.NullString
	if err := scanner.Scan(
		&record.ID,
		&record.Name,
		&record.TokenHash,
		&record.TokenFingerprint,
		&record.CreatedAt,
		&lastUsed,
		&expires,
		&scopes,
		&record.Disabled,
		&revoked,
		&issuedFor,
		&userID,
		&role,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if lastUsed.Valid {
		record.LastUsedAt = &lastUsed.String
	}
	if expires.Valid {
		record.ExpiresAt = &expires.String
	}
	if scopes.Valid {
		record.Scopes = &scopes.String
	}
	if revoked.Valid {
		record.RevokedAt = &revoked.String
	}
	if issuedFor.Valid {
		record.IssuedFor = &issuedFor.String
	}
	if userID.Valid {
		record.UserID = &userID.Int64
	}
	if role.Valid {
		record.Role = &role.String
	}
	return &record, nil
}

// LinkTokenUser attaches a token that has no user yet to one. The bootstrap
// token is created when the agent first starts, which can be before the web
// app has created the first super admin, so it starts out unlinked.
func (s *SQLiteStore) LinkTokenUser(tokenID int64, userID int64) error {
	_, err := s.DB.Exec(`UPDATE tokens SET user_id = ? WHERE id = ? AND user_id IS NULL`, userID, tokenID)
	return err
}
