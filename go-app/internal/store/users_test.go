package store

import (
	"database/sql"
	"testing"

	"docklite-agent/internal/testhelpers"
)

func setupUserTestStore(t *testing.T) *SQLiteStore {
	db := testhelpers.TestStoreWithTables(t)
	return &SQLiteStore{DB: db, Path: ":memory:"}
}

func TestSQLiteStore_CreateUser(t *testing.T) {
	store := setupUserTestStore(t)
	defer store.Close()

	t.Run("creates user with default role", func(t *testing.T) {
		user, err := store.CreateUser("testuser", "password123", "", nil)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertNotEqual(t, nil, user)
		testhelpers.AssertEqual(t, "testuser", user.Username)
		testhelpers.AssertEqual(t, "user", user.Role)
		testhelpers.AssertEqual(t, 0, user.IsAdmin)
		testhelpers.AssertEqual(t, 0, user.IsSuperAdmin)
	})

	t.Run("creates admin user", func(t *testing.T) {
		user, err := store.CreateUser("adminuser", "password123", "admin", nil)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertNotEqual(t, nil, user)
		testhelpers.AssertEqual(t, "admin", user.Role)
		testhelpers.AssertEqual(t, 1, user.IsAdmin)
		testhelpers.AssertEqual(t, 0, user.IsSuperAdmin)
	})

	t.Run("creates super_admin user", func(t *testing.T) {
		user, err := store.CreateUser("superuser", "password123", "super_admin", nil)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertNotEqual(t, nil, user)
		testhelpers.AssertEqual(t, "super_admin", user.Role)
		testhelpers.AssertEqual(t, 1, user.IsAdmin)
		testhelpers.AssertEqual(t, 1, user.IsSuperAdmin)
	})

	t.Run("creates user with managed_by", func(t *testing.T) {
		manager, err := store.CreateUser("manager", "password123", "admin", nil)
		testhelpers.AssertNoError(t, err)

		user, err := store.CreateUser("manageduser", "password123", "user", &manager.ID)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertTrue(t, user.ManagedBy.Valid, "ManagedBy should be valid")
		testhelpers.AssertEqual(t, manager.ID, user.ManagedBy.Int64)
	})

	t.Run("rejects duplicate username", func(t *testing.T) {
		_, err := store.CreateUser("duplicate", "password123", "", nil)
		testhelpers.AssertNoError(t, err)

		_, err = store.CreateUser("duplicate", "password456", "", nil)
		testhelpers.AssertError(t, err)
	})

	t.Run("hashes password", func(t *testing.T) {
		user, err := store.CreateUser("hashuser", "password123", "", nil)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertNotEqual(t, "password123", user.PasswordHash)
		testhelpers.AssertTrue(t, len(user.PasswordHash) > 20, "Hash should be long enough")
	})
}

func TestSQLiteStore_GetUserByUsername(t *testing.T) {
	store := setupUserTestStore(t)
	defer store.Close()

	t.Run("finds existing user", func(t *testing.T) {
		created, err := store.CreateUser("findme", "password123", "user", nil)
		testhelpers.AssertNoError(t, err)

		user, err := store.GetUserByUsername("findme")
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertNotEqual(t, nil, user)
		testhelpers.AssertEqual(t, created.ID, user.ID)
		testhelpers.AssertEqual(t, "findme", user.Username)
	})

	t.Run("returns nil for non-existent user", func(t *testing.T) {
		user, err := store.GetUserByUsername("nonexistent")
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, (*UserRecord)(nil), user)
	})
}

func TestSQLiteStore_GetUserByIDFull(t *testing.T) {
	store := setupUserTestStore(t)
	defer store.Close()

	t.Run("finds existing user", func(t *testing.T) {
		created, err := store.CreateUser("byid", "password123", "admin", nil)
		testhelpers.AssertNoError(t, err)

		user, err := store.GetUserByIDFull(created.ID)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertNotEqual(t, nil, user)
		testhelpers.AssertEqual(t, created.ID, user.ID)
		testhelpers.AssertEqual(t, "admin", user.Role)
	})

	t.Run("returns nil for non-existent ID", func(t *testing.T) {
		user, err := store.GetUserByIDFull(99999)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, (*UserRecord)(nil), user)
	})
}

func TestSQLiteStore_VerifyPassword(t *testing.T) {
	store := setupUserTestStore(t)
	defer store.Close()

	t.Run("verifies correct password", func(t *testing.T) {
		user, err := store.CreateUser("verifyuser", "correctpassword", "user", nil)
		testhelpers.AssertNoError(t, err)

		err = store.VerifyPassword(user, "correctpassword")
		testhelpers.AssertNoError(t, err)
	})

	t.Run("rejects incorrect password", func(t *testing.T) {
		user, err := store.CreateUser("verifyuser2", "correctpassword", "user", nil)
		testhelpers.AssertNoError(t, err)

		err = store.VerifyPassword(user, "wrongpassword")
		testhelpers.AssertError(t, err)
	})

	t.Run("handles nil user", func(t *testing.T) {
		err := store.VerifyPassword(nil, "password")
		testhelpers.AssertError(t, err)
		testhelpers.AssertErrorContains(t, err, "user not found")
	})
}

func TestUserRecord_RoleChecks(t *testing.T) {
	store := setupUserTestStore(t)
	defer store.Close()

	t.Run("user role has no admin flags", func(t *testing.T) {
		user, err := store.CreateUser("regular", "password", "user", nil)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, 0, user.IsAdmin)
		testhelpers.AssertEqual(t, 0, user.IsSuperAdmin)
	})

	t.Run("admin role has is_admin flag", func(t *testing.T) {
		user, err := store.CreateUser("adminrole", "password", "admin", nil)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, 1, user.IsAdmin)
		testhelpers.AssertEqual(t, 0, user.IsSuperAdmin)
	})

	t.Run("super_admin role has both flags", func(t *testing.T) {
		user, err := store.CreateUser("superrole", "password", "super_admin", nil)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, 1, user.IsAdmin)
		testhelpers.AssertEqual(t, 1, user.IsSuperAdmin)
	})
}

func TestUserRecord_ManagedBy(t *testing.T) {
	store := setupUserTestStore(t)
	defer store.Close()

	t.Run("null managed_by for unmanaged user", func(t *testing.T) {
		user, err := store.CreateUser("unmanaged", "password", "user", nil)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertFalse(t, user.ManagedBy.Valid, "ManagedBy should be NULL")
	})

	t.Run("managed_by set for managed user", func(t *testing.T) {
		manager, err := store.CreateUser("manager", "password", "admin", nil)
		testhelpers.AssertNoError(t, err)

		user, err := store.CreateUser("managed", "password", "user", &manager.ID)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertTrue(t, user.ManagedBy.Valid, "ManagedBy should be set")
		testhelpers.AssertEqual(t, manager.ID, user.ManagedBy.Int64)
	})

	t.Run("managed_by is sql.NullInt64", func(t *testing.T) {
		var user UserRecord
		_ = user.ManagedBy
		var _ sql.NullInt64 = user.ManagedBy
	})
}
