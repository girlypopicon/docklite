package store

import (
	"database/sql"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

type UserRecord struct {
	ID           int64         `json:"id"`
	Username     string        `json:"username"`
	PasswordHash string        `json:"password_hash"`
	IsAdmin      int           `json:"is_admin"`
	Role         string        `json:"role"`
	IsSuperAdmin int           `json:"is_super_admin"`
	ManagedBy    sql.NullInt64 `json:"managed_by"`
	CreatedAt    string        `json:"created_at"`
}

func (s *SQLiteStore) GetUserByUsername(username string) (*UserRecord, error) {
	row := s.DB.QueryRow(`
    SELECT id, username, password_hash, is_admin, role, is_super_admin, managed_by, created_at
    FROM users WHERE username = ?
  `, username)
	return scanUser(row)
}

func (s *SQLiteStore) GetUserByIDFull(id int64) (*UserRecord, error) {
	row := s.DB.QueryRow(`
    SELECT id, username, password_hash, is_admin, role, is_super_admin, managed_by, created_at
    FROM users WHERE id = ?
  `, id)
	return scanUser(row)
}

func (s *SQLiteStore) GetUserByRole(role string) (*UserRecord, error) {
	row := s.DB.QueryRow(`
    SELECT id, username, password_hash, is_admin, role, is_super_admin, managed_by, created_at
    FROM users WHERE role = ?
    ORDER BY id ASC
    LIMIT 1
  `, role)
	return scanUser(row)
}

func (s *SQLiteStore) ListUsers() ([]UserRecord, error) {
	rows, err := s.DB.Query(`
    SELECT id, username, password_hash, is_admin, role, is_super_admin, managed_by, created_at
    FROM users
    ORDER BY created_at DESC
  `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []UserRecord
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *user)
	}
	return results, rows.Err()
}

func (s *SQLiteStore) CreateUser(username string, password string, role string, managedBy *int64) (*UserRecord, error) {
	if role == "" {
		role = "user"
	}
	isAdmin := 0
	isSuper := 0
	if role == "admin" || role == "super_admin" {
		isAdmin = 1
	}
	if role == "super_admin" {
		isSuper = 1
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	result, err := s.DB.Exec(`
    INSERT INTO users (username, password_hash, is_admin, role, is_super_admin, managed_by)
    VALUES (?, ?, ?, ?, ?, ?)
  `, username, string(passwordHash), isAdmin, role, isSuper, managedBy)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetUserByIDFull(id)
}

func (s *SQLiteStore) UpdateUserPassword(userID int64, newPassword string) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(passwordHash), userID)
	return err
}

func (s *SQLiteStore) VerifyPassword(user *UserRecord, password string) error {
	if user == nil {
		return errors.New("user not found")
	}
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
}

func (s *SQLiteStore) DeleteUserWithTransfer(fromUserID int64, toUserID int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`UPDATE sites SET user_id = ? WHERE user_id = ?`, toUserID, fromUserID); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM database_permissions WHERE user_id = ?`, fromUserID); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM users WHERE id = ?`, fromUserID); err != nil {
		return err
	}
	// Best effort: the table is missing on a database the agent hasn't set up yet, and a stale row is harmless anyway.
	_, _ = tx.Exec(`DELETE FROM user_two_factor WHERE user_id = ?`, fromUserID)

	return tx.Commit()
}

func scanUser(scanner interface {
	Scan(dest ...any) error
}) (*UserRecord, error) {
	var user UserRecord
	if err := scanner.Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsAdmin,
		&user.Role,
		&user.IsSuperAdmin,
		&user.ManagedBy,
		&user.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}
