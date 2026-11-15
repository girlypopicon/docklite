import type Database from 'better-sqlite3';

export const version = '017';
export const name = 'two_factor';

// Two-factor login (authenticator app codes and recovery codes), one row per user who has started setting it up. The
// agent creates the same table on start (store.EnsureTwoFactorSchema); whichever runs first does it. Additive, so the
// previous release keeps working with the same file: it simply doesn't ask for codes.
export function up(db: Database.Database) {
  db.exec(`
    CREATE TABLE IF NOT EXISTS user_two_factor (
      user_id INTEGER PRIMARY KEY,
      secret TEXT NOT NULL,
      enabled INTEGER NOT NULL DEFAULT 0,
      recovery_hashes TEXT NOT NULL DEFAULT '[]',
      last_step INTEGER NOT NULL DEFAULT 0,
      created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
      enabled_at TEXT
    )
  `);
}

export function down(db: Database.Database) {
  db.exec('DROP TABLE IF EXISTS user_two_factor');
}
