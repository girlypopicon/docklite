import fs from 'fs/promises';
import path from 'path';

const SITES_BASE_PATH = process.env.DOCKLITE_SITES_DIR || '/var/www/sites';

/**
 * Ensures a user's home directory exists in /var/www/sites/{username}
 * Creates the directory if it doesn't exist with proper permissions
 */
export async function ensureUserFolder(username: string): Promise<string> {
  const userPath = path.join(SITES_BASE_PATH, username);

  try {
    // Check if directory exists
    await fs.access(userPath);
  } catch (error: any) {
    if (error.code === 'ENOENT') {
      // Directory doesn't exist, create it
      await fs.mkdir(userPath, { recursive: true, mode: 0o755 });
      console.log(`✓ Created user folder: ${userPath}`);
    } else {
      throw error;
    }
  }

  return userPath;
}

/**
 * Ensures all users in the database have their folders created
 * This is a repair/maintenance function
 */
export async function ensureAllUserFolders(users: { username: string }[]): Promise<void> {
  const results = await Promise.allSettled(
    users.map(user => ensureUserFolder(user.username))
  );

  const created = results.filter(r => r.status === 'fulfilled').length;
  const failed = results.filter(r => r.status === 'rejected').length;

  console.log(`User folder check complete: ${created} OK, ${failed} failed`);
}

/**
 * Gets the user's sites directory path
 */
export function getUserSitesPath(username: string): string {
  return path.join(SITES_BASE_PATH, username);
}
