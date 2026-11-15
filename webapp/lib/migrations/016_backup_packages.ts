import type Database from 'better-sqlite3';

export const version = '016';
export const name = 'backup_packages';

// .litpkg backups: which databases belong to which site (so "this site and its databases" knows what to include), what
// each backup holds in a few words, and what each schedule backs up. Additions only, so the previous release keeps
// working with the same file. The agent adds the same on start (store.EnsureBackupPackageSchema); whichever runs first
// does it and the other finds nothing to do.
export function up(db: Database.Database) {
  db.exec(`
    CREATE TABLE IF NOT EXISTS site_databases (
      database_id INTEGER PRIMARY KEY,
      site_id INTEGER NOT NULL,
      created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    )
  `);
  const has = (table: string, column: string) =>
    (db.prepare(`PRAGMA table_info(${table})`).all() as { name: string }[]).some((c) => c.name === column);
  if (!has('backups', 'scope')) {
    db.prepare('ALTER TABLE backups ADD COLUMN scope TEXT').run();
  }
  if (!has('backup_jobs', 'selection')) {
    db.prepare('ALTER TABLE backup_jobs ADD COLUMN selection TEXT').run();
  }
}

export function down(db: Database.Database) {
  db.exec('DROP TABLE IF EXISTS site_databases');
  try {
    db.prepare('ALTER TABLE backups DROP COLUMN scope').run();
    db.prepare('ALTER TABLE backup_jobs DROP COLUMN selection').run();
  } catch (error) {
    console.warn('Failed to drop columns (SQLite version might be too old):', error);
  }
}
