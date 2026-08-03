import type Database from 'better-sqlite3';

export const version = '015';
export const name = 'add_sqlite_support';

export function up(db: Database.Database) {
  const cols = db.prepare("PRAGMA table_info(databases)").all() as { name: string }[];
  const names = cols.map(c => c.name);
  if (!names.includes('type')) {
    db.prepare("ALTER TABLE databases ADD COLUMN type TEXT DEFAULT 'postgres'").run();
  }
  if (!names.includes('db_path')) {
    db.prepare("ALTER TABLE databases ADD COLUMN db_path TEXT").run();
  }
}

export function down(db: Database.Database) {
  try {
    db.prepare("ALTER TABLE databases DROP COLUMN type").run();
    db.prepare("ALTER TABLE databases DROP COLUMN db_path").run();
  } catch (error) {
    console.warn('Failed to drop columns (SQLite version might be too old):', error);
  }
}
