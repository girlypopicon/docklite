import type Database from 'better-sqlite3';
import crypto from 'crypto';
import fs from 'fs';
import path from 'path';
import type { Migration } from './types';
import { validateMigration } from './validate';

// Import all migrations in order
import * as migration001 from './001_initial_schema';
import * as migration002 from './002_add_user_roles';
import * as migration003 from './003_add_site_extensions';
import * as migration004 from './004_create_folders';
import * as migration005 from './005_migrate_existing_data';
import * as migration006 from './006_drop_sessions_table';
import * as migration007 from './007_create_backups_system';
import * as migration008 from './008_create_dns_system';
import * as migration009 from './009_add_container_positions';
import * as migration010 from './010_add_nested_folders';
import * as migration011 from './011_promote_admin_super_admin';
import * as migration012 from './012_rename_admin_user';
import * as migration013 from './013_add_container_tracking';
import * as migration014 from './014_create_tokens';
import * as migration015 from './015_add_sqlite_support';
import * as migration016 from './016_backup_packages';

const allMigrations: Migration[] = [
  migration001,
  migration002,
  migration003,
  migration004,
  migration005,
  migration006,
  migration007,
  migration008,
  migration009,
  migration010,
  migration011,
  migration012,
  migration013,
  migration014,
  migration015,
  migration016,
];

function ensureMigrationsTable(db: Database.Database): void {
  db.exec(`
    CREATE TABLE IF NOT EXISTS migrations (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      version TEXT UNIQUE NOT NULL,
      name TEXT NOT NULL,
      executed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
      checksum TEXT NOT NULL
    )
  `);
}

function calculateChecksum(migration: Migration): string {
  const content = migration.up.toString() + migration.down.toString();
  return crypto.createHash('sha256').update(content).digest('hex');
}

function getAppliedMigrations(db: Database.Database): Set<string> {
  const rows = db.prepare(`SELECT version FROM migrations ORDER BY version`).all() as Array<{version: string}>;
  return new Set(rows.map(r => r.version));
}

function createBackup(dbPath: string): void {
  if (!fs.existsSync(dbPath)) return;

  const backupPath = `${dbPath}.pre-migration-${Date.now()}`;
  fs.copyFileSync(dbPath, backupPath);
  console.log(`✓ Backup created: ${backupPath}`);
}

export function runMigrations(db: Database.Database, dbPath?: string): void {
  console.log('Starting database migrations...');

  ensureMigrationsTable(db);
  const applied = getAppliedMigrations(db);

  // Create backup before first migration
  if (applied.size === 0 && dbPath) {
    createBackup(dbPath);
  }

  let executedCount = 0;

  for (const migration of allMigrations) {
    if (applied.has(migration.version)) {
      console.log(`Skipping ${migration.version}_${migration.name} (already applied)`);
      continue;
    }

    console.log(`Running ${migration.version}_${migration.name}...`);

    // Execute in transaction for safety
    const transaction = db.transaction(() => {
      migration.up(db);

      const checksum = calculateChecksum(migration);
      db.prepare(`
        INSERT OR IGNORE INTO migrations (version, name, checksum)
        VALUES (?, ?, ?)
      `).run(migration.version, migration.name, checksum);
    });

    try {
      transaction();
      console.log(`Completed ${migration.version}_${migration.name}`);
      executedCount++;
    } catch (error) {
      console.error(`Migration ${migration.version}_${migration.name} failed:`, error);
      throw error;
    }
  }

  if (executedCount === 0) {
    console.log('✓ Database is up to date (no migrations needed)');
  } else {
    console.log(`Applied ${executedCount} migration(s) successfully`);

    // Run validation
    const valid = validateMigration(db);
    if (!valid) {
      console.warn('Some validation checks failed. Please review the results above.');
    }
  }
}

export function rollbackMigration(db: Database.Database, targetVersion?: string): void {
  console.log('Rolling back migrations...');

  const applied = db.prepare(`
    SELECT version, name FROM migrations
    ORDER BY version DESC
  `).all() as Array<{version: string, name: string}>;

  if (applied.length === 0) {
    console.log('No migrations to rollback');
    return;
  }

  for (const record of applied) {
    if (targetVersion && record.version < targetVersion) {
      break;
    }

    const migration = allMigrations.find(m => m.version === record.version);
    if (!migration) {
      throw new Error(`Migration ${record.version} not found in code`);
    }

    console.log(`Rolling back ${record.version}_${record.name}...`);

    const transaction = db.transaction(() => {
      migration.down(db);
      db.prepare(`DELETE FROM migrations WHERE version = ?`).run(record.version);
    });

    transaction();
    console.log(`Rolled back ${record.version}_${record.name}`);

    if (!targetVersion) {
      // Only rollback one migration if no target specified
      break;
    }
  }
}
