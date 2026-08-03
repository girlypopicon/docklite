import Database from 'better-sqlite3';
import bcrypt from 'bcrypt';
import path from 'path';
import {
  User,
  Site,
  CreateSiteParams,
  Database as DatabaseType,
  CreateDatabaseParams,
  DatabasePermission,
  Folder,
  FolderContainer,
  UserRole,
  BackupDestination,
  BackupJob,
  Backup,
  BackupStatus,
  CreateBackupDestinationParams,
  CreateBackupJobParams,
  CloudflareConfig,
  DNSZone,
  DNSRecord,
  CreateDNSZoneParams,
  CreateDNSRecordParams
} from '@/types';
import { runMigrations } from './migrations';

// Initialize database connection
const dbPath = process.env.DATABASE_PATH || path.join(process.cwd(), 'data', 'docklite.db');
const db = new Database(dbPath);

// Enable foreign keys
db.pragma('foreign_keys = ON');

// Initialize tables
export function initializeDatabase() {
  // Users table
  db.exec(`
    CREATE TABLE IF NOT EXISTS users (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      username TEXT UNIQUE NOT NULL,
      password_hash TEXT NOT NULL,
      is_admin INTEGER DEFAULT 0,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    )
  `);

  // Sites table
  db.exec(`
    CREATE TABLE IF NOT EXISTS sites (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      domain TEXT UNIQUE NOT NULL,
      user_id INTEGER NOT NULL,
      container_id TEXT,
      template_type TEXT NOT NULL,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
      FOREIGN KEY (user_id) REFERENCES users(id)
    )
  `);

  // Databases table
  db.exec(`
    CREATE TABLE IF NOT EXISTS databases (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      name TEXT UNIQUE NOT NULL,
      type TEXT DEFAULT 'postgres',
      container_id TEXT UNIQUE NOT NULL,
      postgres_port INTEGER NOT NULL,
      db_path TEXT,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    )
  `);

  // Database permissions table
  db.exec(`
    CREATE TABLE IF NOT EXISTS database_permissions (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      user_id INTEGER NOT NULL,
      database_id INTEGER NOT NULL,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
      FOREIGN KEY (user_id) REFERENCES users(id),
      FOREIGN KEY (database_id) REFERENCES databases(id),
      UNIQUE(user_id, database_id)
    )
  `);

  // Sessions table
  // Run migrations after initial schema creation
  runMigrations(db, dbPath);

  // Seed admin user if not exists
  seedAdminUser();

  // Ensure all users have their folders created
  ensureUserFoldersOnStartup();
}

// Ensure all users have folders on startup
async function ensureUserFoldersOnStartup() {
  try {
    const { ensureAllUserFolders } = await import('./user-helpers');
    const users = getAllUsers();
    if (users.length > 0) {
      await ensureAllUserFolders(users);
    }
  } catch (error) {
    console.error('Failed to check user folders on startup:', error);
  }
}

// Seed initial admin user
function seedAdminUser() {
  try {
    const existingSuperAdmin = db.prepare('SELECT id FROM users WHERE username = ?').get('superadmin') as { id: number } | undefined;
    const existingAdmin = db.prepare('SELECT id FROM users WHERE username = ?').get('admin') as { id: number } | undefined;

    if (!existingSuperAdmin && !existingAdmin) {
      const seedUsername = process.env.SEED_ADMIN_USERNAME;
      const seedPassword = process.env.SEED_ADMIN_PASSWORD;

      if (!seedUsername || !seedPassword) {
        const devUsername = 'superadmin';
        const devPassword = 'password123';
        const passwordHash = bcrypt.hashSync(devPassword, 10);
        db.prepare(`
          INSERT INTO users (username, password_hash, is_admin, role, is_super_admin, managed_by)
          VALUES (?, ?, 1, 'super_admin', 1, NULL)
        `).run(devUsername, passwordHash);
        console.log(`✓ Superadmin user created — CHANGE THE DEFAULT PASSWORD IMMEDIATELY`);
        return;
      }

      const passwordHash = bcrypt.hashSync(seedPassword, 10);
      db.prepare(`
        INSERT INTO users (username, password_hash, is_admin, role, is_super_admin, managed_by)
        VALUES (?, ?, 1, 'super_admin', 1, NULL)
      `).run(seedUsername, passwordHash);
      console.log(`✓ Superadmin user created (username: ${seedUsername})`);
    } else if (existingSuperAdmin) {
      db.prepare(`
        UPDATE users
        SET role = 'super_admin', is_super_admin = 1, is_admin = 1, managed_by = NULL
        WHERE id = ?
      `).run(existingSuperAdmin.id);
    } else if (existingAdmin) {
      db.prepare(`
        UPDATE users
        SET role = 'super_admin', is_super_admin = 1, is_admin = 1, managed_by = NULL
        WHERE id = ?
      `).run(existingAdmin.id);
    }
  } catch (error: any) {
    // Ignore UNIQUE constraint errors (admin already exists)
    if (error.code !== 'SQLITE_CONSTRAINT_UNIQUE') {
      throw error;
    }
  }
}

// ============================================
// USER FUNCTIONS
// ============================================

export function getUser(username: string): User | undefined {
  return db.prepare('SELECT * FROM users WHERE username = ?').get(username) as User | undefined;
}

export function getUserById(id: number): User | undefined {
  return db.prepare('SELECT * FROM users WHERE id = ?').get(id) as User | undefined;
}

export function getAllUsers(): User[] {
  return db.prepare('SELECT * FROM users ORDER BY created_at DESC').all() as User[];
}

export function createUser(
  username: string,
  password: string,
  isAdmin: boolean = false,
  role?: UserRole,
  managedBy?: number | null
): User {
  const passwordHash = bcrypt.hashSync(password, 10);

  // Determine role if not explicitly provided
  const userRole: UserRole = role || (isAdmin ? 'admin' : 'user');
  const isSuperAdmin = userRole === 'super_admin' ? 1 : 0;
  const isAdminValue = (userRole === 'admin' || userRole === 'super_admin') ? 1 : 0;

  const result = db.prepare(`
    INSERT INTO users (username, password_hash, is_admin, role, is_super_admin, managed_by)
    VALUES (?, ?, ?, ?, ?, ?)
  `).run(username, passwordHash, isAdminValue, userRole, isSuperAdmin, managedBy || null);

  return getUserById(result.lastInsertRowid as number)!;
}

export function verifyPassword(user: User, password: string): boolean {
  return bcrypt.compareSync(password, user.password_hash);
}

export function updateUserPassword(userId: number, password: string): void {
  const passwordHash = bcrypt.hashSync(password, 10);
  db.prepare('UPDATE users SET password_hash = ? WHERE id = ?').run(passwordHash, userId);
}

export function clearUserSessions(userId: number): void {
  console.warn(`clearUserSessions(${userId}) called: session storage uses cookies only.`);
}

export function getUserSiteCount(userId: number): number {
  const row = db.prepare('SELECT COUNT(*) as count FROM sites WHERE user_id = ?').get(userId) as { count: number };
  return row?.count || 0;
}

export function deleteUser(userId: number): void {
  db.prepare('DELETE FROM database_permissions WHERE user_id = ?').run(userId);
  db.prepare('DELETE FROM users WHERE id = ?').run(userId);
}

export function deleteUserWithTransfer(fromUserId: number, toUserId: number): void {
  const transaction = db.transaction(() => {
    transferUserSites(fromUserId, toUserId);
    deleteUser(fromUserId);
  });
  transaction();
}

export function getUsersByManager(managerId: number): User[] {
  return db.prepare(`
    SELECT * FROM users WHERE managed_by = ? ORDER BY created_at DESC
  `).all(managerId) as User[];
}

// ============================================
// SITE FUNCTIONS
// ============================================

export function getSitesByUser(userId: number, isAdmin: boolean): Site[] {
  if (isAdmin) {
    return db.prepare('SELECT * FROM sites ORDER BY created_at DESC').all() as Site[];
  }
  return db.prepare('SELECT * FROM sites WHERE user_id = ? ORDER BY created_at DESC').all(userId) as Site[];
}

export function transferUserSites(fromUserId: number, toUserId: number): number {
  const result = db.prepare('UPDATE sites SET user_id = ? WHERE user_id = ?').run(toUserId, fromUserId);
  return result.changes;
}

export function getAllSites(): Site[] {
  return db.prepare('SELECT * FROM sites ORDER BY created_at DESC').all() as Site[];
}

export function getSiteByDomain(domain: string): Site | undefined {
  return db.prepare('SELECT * FROM sites WHERE domain = ?').get(domain) as Site | undefined;
}

export function getSiteById(id: number, userId: number, isAdmin: boolean): Site | undefined {
  if (isAdmin) {
    return db.prepare('SELECT * FROM sites WHERE id = ?').get(id) as Site | undefined;
  }
  return db.prepare('SELECT * FROM sites WHERE id = ? AND user_id = ?').get(id, userId) as Site | undefined;
}

export function getSiteByContainerId(containerId: string): Site | undefined {
  return db.prepare('SELECT * FROM sites WHERE container_id = ?').get(containerId) as Site | undefined;
}

export function updateSiteUserIdByContainerId(containerId: string, userId: number): void {
  db.prepare('UPDATE sites SET user_id = ? WHERE container_id = ?').run(userId, containerId);
}



export function createSite(params: CreateSiteParams): Site {
  const columns = ['domain', 'user_id', 'template_type'];
  const values: (string | number)[] = [params.domain, params.user_id, params.template_type];

  if (params.container_id) {
    columns.push('container_id');
    values.push(params.container_id);
  }

  if (params.code_path) {
    columns.push('code_path');
    values.push(params.code_path);
  }

  const placeholders = values.map(() => '?').join(', ');
  const sql = `INSERT INTO sites (${columns.join(', ')}) VALUES (${placeholders})`;

  const result = db.prepare(sql).run(...values);

  return getSiteById(result.lastInsertRowid as number, params.user_id, true)!;
}

export function updateSiteContainerId(id: number, containerId: string): void {
  db.prepare('UPDATE sites SET container_id = ? WHERE id = ?').run(containerId, id);
}

export function updateSiteStatus(id: number, status: string): void {
  db.prepare('UPDATE sites SET status = ? WHERE id = ?').run(status, id);
}

export function deleteSite(id: number): void {
  db.prepare('DELETE FROM sites WHERE id = ?').run(id);
}

// ============================================
// DATABASE FUNCTIONS
// ============================================

export function getDatabasesByUser(userId: number, isAdmin: boolean): DatabaseType[] {
  if (isAdmin) {
    return db.prepare('SELECT * FROM databases ORDER BY created_at DESC').all() as DatabaseType[];
  }

  // Get databases user has permission to access
  return db.prepare(`
    SELECT d.* FROM databases d
    JOIN database_permissions dp ON d.id = dp.database_id
    WHERE dp.user_id = ?
    ORDER BY d.created_at DESC
  `).all(userId) as DatabaseType[];
}

export function getDatabaseById(id: number): DatabaseType | undefined {
  return db.prepare('SELECT * FROM databases WHERE id = ?').get(id) as DatabaseType | undefined;
}

export function getAllDatabases(): DatabaseType[] {
  return db.prepare('SELECT * FROM databases ORDER BY created_at DESC').all() as DatabaseType[];
}

export function createDatabase(params: CreateDatabaseParams): DatabaseType {
  const result = db.prepare(`
    INSERT INTO databases (name, type, container_id, postgres_port, db_path)
    VALUES (?, ?, ?, ?, ?)
  `).run(params.name, params.type, params.container_id, params.postgres_port, params.db_path || null);

  return getDatabaseById(result.lastInsertRowid as number)!;
}

export function getNextAvailablePort(): number {
  const lastDb = db.prepare('SELECT MAX(postgres_port) as max_port FROM databases').get() as { max_port: number | null };
  return lastDb.max_port ? lastDb.max_port + 1 : 5432;
}

export function deleteDatabase(id: number): void {
  // Delete permissions first
  db.prepare('DELETE FROM database_permissions WHERE database_id = ?').run(id);
  // Delete database
  db.prepare('DELETE FROM databases WHERE id = ?').run(id);
}

// ============================================
// PERMISSION FUNCTIONS
// ============================================

export function grantDatabaseAccess(userId: number, databaseId: number): void {
  try {
    db.prepare(`
      INSERT INTO database_permissions (user_id, database_id)
      VALUES (?, ?)
    `).run(userId, databaseId);
  } catch (error) {
    // Permission already exists - ignore
  }
}

export function revokeDatabaseAccess(userId: number, databaseId: number): void {
  db.prepare(`
    DELETE FROM database_permissions
    WHERE user_id = ? AND database_id = ?
  `).run(userId, databaseId);
}

export function hasAccess(userId: number, databaseId: number, isAdmin: boolean): boolean {
  if (isAdmin) return true;

  const permission = db.prepare(`
    SELECT id FROM database_permissions
    WHERE user_id = ? AND database_id = ?
  `).get(userId, databaseId);

  return !!permission;
}

export function getDatabasePermissions(databaseId: number): DatabasePermission[] {
  return db.prepare(`
    SELECT * FROM database_permissions WHERE database_id = ?
  `).all(databaseId) as DatabasePermission[];
}

// ============================================
// FOLDER FUNCTIONS
// ============================================

export function createFolder(userId: number, name: string, parentFolderId?: number): Folder {
  // Get depth and position based on parent
  let depth = 0;
  let position = 0;

  if (parentFolderId) {
    const parent = getFolderById(parentFolderId);
    if (parent) {
      depth = parent.depth + 1;

      // Get next position within parent
      const maxPos = db.prepare(`
        SELECT COALESCE(MAX(position), -1) as max_pos
        FROM folders
        WHERE user_id = ? AND parent_folder_id = ?
      `).get(userId, parentFolderId) as { max_pos: number };

      position = maxPos.max_pos + 1;
    }
  } else {
    // Get next position at root level
    const maxPos = db.prepare(`
      SELECT COALESCE(MAX(position), -1) as max_pos
      FROM folders
      WHERE user_id = ? AND parent_folder_id IS NULL
    `).get(userId) as { max_pos: number };

    position = maxPos.max_pos + 1;
  }

  const result = db.prepare(`
    INSERT INTO folders (user_id, name, parent_folder_id, depth, position)
    VALUES (?, ?, ?, ?, ?)
  `).run(userId, name, parentFolderId || null, depth, position);

  return getFolderById(result.lastInsertRowid as number)!;
}

export function getFoldersByUser(userId: number): Folder[] {
  return db.prepare(`
    SELECT * FROM folders
    WHERE user_id = ?
    ORDER BY parent_folder_id NULLS FIRST, position ASC
  `).all(userId) as Folder[];
}

export function getFolderById(id: number): Folder | undefined {
  return db.prepare(`SELECT * FROM folders WHERE id = ?`).get(id) as Folder | undefined;
}

export function deleteFolder(id: number): void {
  // Foreign key constraints will automatically delete folder_containers entries
  db.prepare(`DELETE FROM folders WHERE id = ?`).run(id);
}

export function linkContainerToFolder(folderId: number, containerId: string): void {
  try {
    // Get the max position for this folder
    const maxPosition = db.prepare(`
      SELECT COALESCE(MAX(position), -1) as max_pos
      FROM folder_containers
      WHERE folder_id = ?
    `).get(folderId) as { max_pos: number };

    const newPosition = maxPosition.max_pos + 1;

    db.prepare(`
      INSERT OR IGNORE INTO folder_containers (folder_id, container_id, position)
      VALUES (?, ?, ?)
    `).run(folderId, containerId, newPosition);
  } catch (error) {
    // Ignore if already linked
  }
}

export function unlinkContainerFromFolder(folderId: number, containerId: string): void {
  db.prepare(`
    DELETE FROM folder_containers WHERE folder_id = ? AND container_id = ?
  `).run(folderId, containerId);
}

export function unlinkContainerFromAllFolders(containerId: string): void {
  db.prepare(`
    DELETE FROM folder_containers WHERE container_id = ?
  `).run(containerId);
}

export function getContainersByFolder(folderId: number): string[] {
  const rows = db.prepare(`
    SELECT container_id FROM folder_containers
    WHERE folder_id = ?
    ORDER BY position ASC
  `).all(folderId) as Array<{container_id: string}>;
  return rows.map(r => r.container_id);
}

export function getFolderByContainerId(containerId: string): Folder | undefined {
  return db.prepare(`
    SELECT f.* FROM folders f
    JOIN folder_containers fc ON f.id = fc.folder_id
    WHERE fc.container_id = ?
  `).get(containerId) as Folder | undefined;
}

export function moveContainerToFolder(containerId: string, targetFolderId: number): void {
  const transaction = db.transaction(() => {
    // Get source folder and position
    const sourceLink = db.prepare(`
      SELECT folder_id, position FROM folder_containers WHERE container_id = ?
    `).get(containerId) as { folder_id: number; position: number } | undefined;

    // Remove from source folder
    db.prepare(`
      DELETE FROM folder_containers WHERE container_id = ?
    `).run(containerId);

    // Reindex remaining containers in source folder
    if (sourceLink) {
      db.prepare(`
        UPDATE folder_containers
        SET position = position - 1
        WHERE folder_id = ? AND position > ?
      `).run(sourceLink.folder_id, sourceLink.position);
    }

    // Add to target folder at the end
    linkContainerToFolder(targetFolderId, containerId);
  });

  transaction();
}

// ============================================
// CONTAINER TRACKING
// ============================================

export function getUntrackedContainerIds(): string[] {
  const rows = db.prepare('SELECT container_id FROM untracked_containers').all() as Array<{ container_id: string }>;
  return rows.map(row => row.container_id);
}

export function markContainerUntracked(containerId: string): void {
  db.prepare(`
    INSERT OR IGNORE INTO untracked_containers (container_id)
    VALUES (?)
  `).run(containerId);
}

export function markContainerTracked(containerId: string): void {
  db.prepare('DELETE FROM untracked_containers WHERE container_id = ?').run(containerId);
}

export function reorderContainerInFolder(folderId: number, containerId: string, newPosition: number): void {
  const transaction = db.transaction(() => {
    // Get current position
    const currentLink = db.prepare(`
      SELECT position FROM folder_containers
      WHERE folder_id = ? AND container_id = ?
    `).get(folderId, containerId) as { position: number } | undefined;

    if (!currentLink) {
      throw new Error('Container not found in folder');
    }

    const oldPosition = currentLink.position;

    // Get total count to clamp newPosition
    const countResult = db.prepare(`
      SELECT COUNT(*) as count FROM folder_containers WHERE folder_id = ?
    `).get(folderId) as { count: number };

    // Clamp newPosition to valid range
    const clampedPosition = Math.max(0, Math.min(newPosition, countResult.count - 1));

    if (oldPosition === clampedPosition) {
      return; // No change needed
    }

    if (oldPosition < clampedPosition) {
      // Moving down: shift items between old and new position up
      db.prepare(`
        UPDATE folder_containers
        SET position = position - 1
        WHERE folder_id = ? AND position > ? AND position <= ?
      `).run(folderId, oldPosition, clampedPosition);
    } else {
      // Moving up: shift items between new and old position down
      db.prepare(`
        UPDATE folder_containers
        SET position = position + 1
        WHERE folder_id = ? AND position >= ? AND position < ?
      `).run(folderId, clampedPosition, oldPosition);
    }

    // Update the target container's position
    db.prepare(`
      UPDATE folder_containers
      SET position = ?
      WHERE folder_id = ? AND container_id = ?
    `).run(clampedPosition, folderId, containerId);
  });

  transaction();
}

// Update descendant depths recursively when a folder is moved
function updateDescendantDepths(folderId: number, newDepth: number): void {
  // Get all direct children
  const children = db.prepare(`
    SELECT id FROM folders WHERE parent_folder_id = ?
  `).all(folderId) as { id: number }[];

  for (const child of children) {
    const childDepth = newDepth + 1;

    // Update child depth
    db.prepare(`
      UPDATE folders SET depth = ? WHERE id = ?
    `).run(childDepth, child.id);

    // Recursively update grandchildren
    updateDescendantDepths(child.id, childDepth);
  }
}

export function moveFolderToParent(folderId: number, newParentId: number | null): void {
  const transaction = db.transaction(() => {
    const folder = getFolderById(folderId);
    if (!folder) throw new Error('Folder not found');

    // Calculate new depth
    let newDepth = 0;
    if (newParentId) {
      const newParent = getFolderById(newParentId);
      if (!newParent) throw new Error('Parent folder not found');
      newDepth = newParent.depth + 1;
    }

    // Prevent nesting beyond depth 1 (2 layers total)
    if (newDepth > 1) {
      throw new Error('Maximum nesting depth is 2 layers');
    }

    // Get current position to reindex old siblings
    const oldParentId = folder.parent_folder_id;
    const oldPosition = folder.position;

    // Get next position in new parent
    const maxPos = db.prepare(`
      SELECT COALESCE(MAX(position), -1) as max_pos
      FROM folders
      WHERE user_id = ? AND ${newParentId ? 'parent_folder_id = ?' : 'parent_folder_id IS NULL'}
    `).get(newParentId ? [folder.user_id, newParentId] : [folder.user_id]) as { max_pos: number };

    const newPosition = maxPos.max_pos + 1;

    // Update folder's parent, depth, and position
    db.prepare(`
      UPDATE folders
      SET parent_folder_id = ?, depth = ?, position = ?
      WHERE id = ?
    `).run(newParentId, newDepth, newPosition, folderId);

    // Update all descendant depths
    updateDescendantDepths(folderId, newDepth);

    // Reindex old siblings
    if (oldParentId) {
      db.prepare(`
        UPDATE folders
        SET position = position - 1
        WHERE user_id = ? AND parent_folder_id = ? AND position > ?
      `).run(folder.user_id, oldParentId, oldPosition);
    } else {
      db.prepare(`
        UPDATE folders
        SET position = position - 1
        WHERE user_id = ? AND parent_folder_id IS NULL AND position > ?
      `).run(folder.user_id, oldPosition);
    }
  });

  transaction();
}

export function reorderFolder(folderId: number, newPosition: number): void {
  const transaction = db.transaction(() => {
    const folder = getFolderById(folderId);
    if (!folder) throw new Error('Folder not found');

    const oldPosition = folder.position;

    // Get total count of siblings
    const countResult = db.prepare(`
      SELECT COUNT(*) as count
      FROM folders
      WHERE user_id = ? AND ${folder.parent_folder_id ? 'parent_folder_id = ?' : 'parent_folder_id IS NULL'}
    `).get(folder.parent_folder_id ? [folder.user_id, folder.parent_folder_id] : [folder.user_id]) as { count: number };

    // Clamp position
    const clampedPosition = Math.max(0, Math.min(newPosition, countResult.count - 1));

    if (oldPosition === clampedPosition) return;

    if (oldPosition < clampedPosition) {
      // Moving down
      db.prepare(`
        UPDATE folders
        SET position = position - 1
        WHERE user_id = ? AND ${folder.parent_folder_id ? 'parent_folder_id = ?' : 'parent_folder_id IS NULL'}
          AND position > ? AND position <= ?
      `).run(folder.parent_folder_id ? [folder.user_id, folder.parent_folder_id, oldPosition, clampedPosition] : [folder.user_id, oldPosition, clampedPosition]);
    } else {
      // Moving up
      db.prepare(`
        UPDATE folders
        SET position = position + 1
        WHERE user_id = ? AND ${folder.parent_folder_id ? 'parent_folder_id = ?' : 'parent_folder_id IS NULL'}
          AND position >= ? AND position < ?
      `).run(folder.parent_folder_id ? [folder.user_id, folder.parent_folder_id, clampedPosition, oldPosition] : [folder.user_id, clampedPosition, oldPosition]);
    }

    // Update target folder position
    db.prepare(`
      UPDATE folders SET position = ? WHERE id = ?
    `).run(clampedPosition, folderId);
  });

  transaction();
}

// ============================================
// BACKUP DESTINATIONS
// ============================================

export function createBackupDestination(params: CreateBackupDestinationParams): number {
  const { name, type, config, enabled = 1 } = params;
  const result = db.prepare(`
    INSERT INTO backup_destinations (name, type, config, enabled)
    VALUES (?, ?, ?, ?)
  `).run(name, type, JSON.stringify(config), enabled);
  return result.lastInsertRowid as number;
}

export function getBackupDestinations(): BackupDestination[] {
  return db.prepare(`SELECT * FROM backup_destinations ORDER BY created_at DESC`).all() as BackupDestination[];
}

export function getBackupDestinationById(id: number): BackupDestination | undefined {
  return db.prepare(`SELECT * FROM backup_destinations WHERE id = ?`).get(id) as BackupDestination | undefined;
}

export function updateBackupDestination(id: number, params: Partial<CreateBackupDestinationParams>): void {
  const fields: string[] = [];
  const values: any[] = [];

  if (params.name !== undefined) {
    fields.push('name = ?');
    values.push(params.name);
  }
  if (params.type !== undefined) {
    fields.push('type = ?');
    values.push(params.type);
  }
  if (params.config !== undefined) {
    fields.push('config = ?');
    values.push(JSON.stringify(params.config));
  }
  if (params.enabled !== undefined) {
    fields.push('enabled = ?');
    values.push(params.enabled);
  }

  if (fields.length > 0) {
    values.push(id);
    db.prepare(`UPDATE backup_destinations SET ${fields.join(', ')} WHERE id = ?`).run(...values);
  }
}

export function deleteBackupDestination(id: number): void {
  db.prepare(`DELETE FROM backup_destinations WHERE id = ?`).run(id);
}

// ============================================
// BACKUP JOBS
// ============================================

export function createBackupJob(params: CreateBackupJobParams): number {
  const { destination_id, target_type, target_id = null, frequency, retention_days = 30, enabled = 1 } = params;
  const result = db.prepare(`
    INSERT INTO backup_jobs (destination_id, target_type, target_id, frequency, retention_days, enabled)
    VALUES (?, ?, ?, ?, ?, ?)
  `).run(destination_id, target_type, target_id, frequency, retention_days, enabled);
  return result.lastInsertRowid as number;
}

export function getBackupJobs(): BackupJob[] {
  return db.prepare(`SELECT * FROM backup_jobs ORDER BY created_at DESC`).all() as BackupJob[];
}

export function getBackupJobById(id: number): BackupJob | undefined {
  return db.prepare(`SELECT * FROM backup_jobs WHERE id = ?`).get(id) as BackupJob | undefined;
}

export function getBackupJobsByDestination(destinationId: number): BackupJob[] {
  return db.prepare(`SELECT * FROM backup_jobs WHERE destination_id = ? ORDER BY created_at DESC`).all(destinationId) as BackupJob[];
}

export function getEnabledBackupJobs(): BackupJob[] {
  return db.prepare(`SELECT * FROM backup_jobs WHERE enabled = 1`).all() as BackupJob[];
}

export function updateBackupJob(id: number, params: Partial<CreateBackupJobParams>): void {
  const fields: string[] = [];
  const values: any[] = [];

  if (params.destination_id !== undefined) {
    fields.push('destination_id = ?');
    values.push(params.destination_id);
  }
  if (params.target_type !== undefined) {
    fields.push('target_type = ?');
    values.push(params.target_type);
  }
  if (params.target_id !== undefined) {
    fields.push('target_id = ?');
    values.push(params.target_id);
  }
  if (params.frequency !== undefined) {
    fields.push('frequency = ?');
    values.push(params.frequency);
  }
  if (params.retention_days !== undefined) {
    fields.push('retention_days = ?');
    values.push(params.retention_days);
  }
  if (params.enabled !== undefined) {
    fields.push('enabled = ?');
    values.push(params.enabled);
  }

  if (fields.length > 0) {
    values.push(id);
    db.prepare(`UPDATE backup_jobs SET ${fields.join(', ')} WHERE id = ?`).run(...values);
  }
}

export function updateBackupJobRunTime(id: number, nextRunAt: string | null = null): void {
  const now = new Date().toISOString();
  db.prepare(`
    UPDATE backup_jobs
    SET last_run_at = ?, next_run_at = ?
    WHERE id = ?
  `).run(now, nextRunAt, id);
}

export function deleteBackupJob(id: number): void {
  db.prepare(`DELETE FROM backup_jobs WHERE id = ?`).run(id);
}

// ============================================
// BACKUPS (History)
// ============================================

export function createBackup(params: {
  job_id: number | null;
  destination_id: number;
  target_type: 'site' | 'database';
  target_id: number;
  backup_path: string;
  size_bytes?: number;
  status?: BackupStatus;
  error_message?: string | null;
}): number {
  const {
    job_id,
    destination_id,
    target_type,
    target_id,
    backup_path,
    size_bytes = 0,
    status = 'in_progress',
    error_message = null
  } = params;

  const result = db.prepare(`
    INSERT INTO backups (job_id, destination_id, target_type, target_id, backup_path, size_bytes, status, error_message)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?)
  `).run(job_id, destination_id, target_type, target_id, backup_path, size_bytes, status, error_message);
  return result.lastInsertRowid as number;
}

export function getBackups(limit: number = 100): Backup[] {
  return db.prepare(`SELECT * FROM backups ORDER BY created_at DESC LIMIT ?`).all(limit) as Backup[];
}

export function getBackupById(id: number): Backup | undefined {
  return db.prepare(`SELECT * FROM backups WHERE id = ?`).get(id) as Backup | undefined;
}

export function getBackupsByJob(jobId: number, limit: number = 50): Backup[] {
  return db.prepare(`
    SELECT * FROM backups WHERE job_id = ? ORDER BY created_at DESC LIMIT ?
  `).all(jobId, limit) as Backup[];
}

export function getBackupsByTarget(targetType: 'site' | 'database', targetId: number, limit: number = 50): Backup[] {
  return db.prepare(`
    SELECT * FROM backups
    WHERE target_type = ? AND target_id = ?
    ORDER BY created_at DESC LIMIT ?
  `).all(targetType, targetId, limit) as Backup[];
}

export function updateBackupStatus(
  id: number,
  status: BackupStatus,
  errorMessage?: string | null,
  sizeBytes?: number,
  backupPath?: string | null
): void {
  const fields = ['status = ?'];
  const values: any[] = [status];

  if (errorMessage !== undefined) {
    fields.push('error_message = ?');
    values.push(errorMessage);
  }
  if (sizeBytes !== undefined) {
    fields.push('size_bytes = ?');
    values.push(sizeBytes);
  }
  if (backupPath !== undefined) {
    fields.push('backup_path = ?');
    values.push(backupPath);
  }

  values.push(id);
  db.prepare(`UPDATE backups SET ${fields.join(', ')} WHERE id = ?`).run(...values);
}

export function deleteBackup(id: number): void {
  db.prepare(`DELETE FROM backups WHERE id = ?`).run(id);
}

export function clearBackupHistory(): number {
  const result = db.prepare(`DELETE FROM backups`).run();
  return result.changes;
}

export function deleteOldBackups(destinationId: number, retentionDays: number): number {
  const cutoffDate = new Date();
  cutoffDate.setDate(cutoffDate.getDate() - retentionDays);

  const result = db.prepare(`
    DELETE FROM backups
    WHERE destination_id = ? AND created_at < ? AND status = 'success'
  `).run(destinationId, cutoffDate.toISOString());

  return result.changes;
}

// =======================
// DNS Management Functions
// =======================

// Cloudflare Config
export function getCloudflareConfig(): CloudflareConfig | null {
  return db.prepare('SELECT * FROM cloudflare_config WHERE id = 1').get() as CloudflareConfig | null;
}

export function updateCloudflareConfig(apiToken?: string, accountId?: string, enabled?: number): void {
  const fields: string[] = ['updated_at = CURRENT_TIMESTAMP'];
  const values: any[] = [];

  if (apiToken !== undefined) {
    fields.push('api_token = ?');
    values.push(apiToken);
  }
  if (accountId !== undefined) {
    fields.push('account_id = ?');
    values.push(accountId);
  }
  if (enabled !== undefined) {
    fields.push('enabled = ?');
    values.push(enabled);
  }

  values.push(1);
  db.prepare(`UPDATE cloudflare_config SET ${fields.join(', ')} WHERE id = ?`).run(...values);
}

// DNS Zones
export function getDNSZones(): DNSZone[] {
  return db.prepare('SELECT * FROM dns_zones ORDER BY domain').all() as DNSZone[];
}

export function getDNSZoneById(id: number): DNSZone | null {
  return db.prepare('SELECT * FROM dns_zones WHERE id = ?').get(id) as DNSZone | null;
}

export function getDNSZoneByDomain(domain: string): DNSZone | null {
  return db.prepare('SELECT * FROM dns_zones WHERE domain = ?').get(domain) as DNSZone | null;
}

export function createDNSZone(params: CreateDNSZoneParams): number {
  const result = db.prepare(`
    INSERT INTO dns_zones (domain, zone_id, account_id, enabled)
    VALUES (?, ?, ?, ?)
  `).run(
    params.domain,
    params.zone_id,
    params.account_id || null,
    params.enabled !== undefined ? params.enabled : 1
  );
  return result.lastInsertRowid as number;
}

export function updateDNSZone(id: number, updates: Partial<CreateDNSZoneParams>): void {
  const fields: string[] = [];
  const values: any[] = [];

  if (updates.domain) {
    fields.push('domain = ?');
    values.push(updates.domain);
  }
  if (updates.zone_id) {
    fields.push('zone_id = ?');
    values.push(updates.zone_id);
  }
  if (updates.account_id !== undefined) {
    fields.push('account_id = ?');
    values.push(updates.account_id);
  }
  if (updates.enabled !== undefined) {
    fields.push('enabled = ?');
    values.push(updates.enabled);
  }

  if (fields.length === 0) return;

  values.push(id);
  db.prepare(`UPDATE dns_zones SET ${fields.join(', ')} WHERE id = ?`).run(...values);
}

export function updateDNSZoneSyncTime(id: number): void {
  db.prepare('UPDATE dns_zones SET last_synced_at = CURRENT_TIMESTAMP WHERE id = ?').run(id);
}

export function deleteDNSZone(id: number): void {
  db.prepare('DELETE FROM dns_zones WHERE id = ?').run(id);
}

// DNS Records
export function getDNSRecords(zoneId?: number): DNSRecord[] {
  if (zoneId) {
    return db.prepare('SELECT * FROM dns_records WHERE zone_id = ? ORDER BY type, name').all(zoneId) as DNSRecord[];
  }
  return db.prepare('SELECT * FROM dns_records ORDER BY zone_id, type, name').all() as DNSRecord[];
}

export function getDNSRecordById(id: number): DNSRecord | null {
  return db.prepare('SELECT * FROM dns_records WHERE id = ?').get(id) as DNSRecord | null;
}

export function createDNSRecord(params: CreateDNSRecordParams): number {
  const result = db.prepare(`
    INSERT INTO dns_records (zone_id, cloudflare_record_id, type, name, content, ttl, priority, proxied)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?)
  `).run(
    params.zone_id,
    params.cloudflare_record_id || null,
    params.type,
    params.name,
    params.content,
    params.ttl || 1,
    params.priority || null,
    params.proxied || 0
  );
  return result.lastInsertRowid as number;
}

export function updateDNSRecord(id: number, updates: Partial<CreateDNSRecordParams>): void {
  const fields: string[] = ['updated_at = CURRENT_TIMESTAMP'];
  const values: any[] = [];

  if (updates.cloudflare_record_id !== undefined) {
    fields.push('cloudflare_record_id = ?');
    values.push(updates.cloudflare_record_id);
  }
  if (updates.type) {
    fields.push('type = ?');
    values.push(updates.type);
  }
  if (updates.name) {
    fields.push('name = ?');
    values.push(updates.name);
  }
  if (updates.content) {
    fields.push('content = ?');
    values.push(updates.content);
  }
  if (updates.ttl !== undefined) {
    fields.push('ttl = ?');
    values.push(updates.ttl);
  }
  if (updates.priority !== undefined) {
    fields.push('priority = ?');
    values.push(updates.priority);
  }
  if (updates.proxied !== undefined) {
    fields.push('proxied = ?');
    values.push(updates.proxied);
  }

  values.push(id);
  db.prepare(`UPDATE dns_records SET ${fields.join(', ')} WHERE id = ?`).run(...values);
}

export function deleteDNSRecord(id: number): void {
  db.prepare('DELETE FROM dns_records WHERE id = ?').run(id);
}

export function clearDNSRecords(zoneId: number): void {
  db.prepare('DELETE FROM dns_records WHERE zone_id = ?').run(zoneId);
}

// Initialize database on module load
initializeDatabase();

export default db;
