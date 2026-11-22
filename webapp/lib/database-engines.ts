import type { Icon } from '@phosphor-icons/react';
import { Database, Lightning, FileText, Leaf, Stack } from '@phosphor-icons/react';

/**
 * The kinds of database a container can be. The ids are the agent's (go-app/internal/docker/engines.go); SQLite is not a server
 * but a file inside one of your sites, so it has no container of its own and needs a site to live in.
 */
export type DbType = 'postgres' | 'mysql' | 'mariadb' | 'mongodb' | 'redis' | 'sqlite';

export interface DbEngine {
  id: DbType;
  label: string;
  family: string;
  blurb: string; // what it is for, in plain words
  hasUser: boolean;
  Icon: Icon;
}

export const DB_ENGINES: DbEngine[] = [
  { id: 'postgres', label: 'PostgreSQL', family: 'Tables', hasUser: true, Icon: Database, blurb: 'Tables of rows and columns, like a spreadsheet with rules. A strong all-round choice for most apps.' },
  { id: 'mysql', label: 'MySQL', family: 'Tables', hasUser: true, Icon: Stack, blurb: 'The most widely used open-source database. What WordPress and most PHP sites expect.' },
  { id: 'mariadb', label: 'MariaDB', family: 'Tables', hasUser: true, Icon: Stack, blurb: 'A MySQL-compatible fork with the same tools. Fine wherever MySQL is asked for.' },
  { id: 'mongodb', label: 'MongoDB', family: 'Documents', hasUser: true, Icon: Leaf, blurb: 'Flexible "documents" instead of tables, like contact cards that can each have different fields.' },
  { id: 'redis', label: 'Redis', family: 'Key and value', hasUser: false, Icon: Lightning, blurb: 'Very fast storage held in memory. Used for caching, logins and queues.' },
  { id: 'sqlite', label: 'SQLite', family: 'A file', hasUser: false, Icon: FileText, blurb: 'A single file inside one of your sites. No separate server to run.' },
];

export const dbLabel = (t?: string) => DB_ENGINES.find((e) => e.id === (t || 'postgres'))?.label ?? 'Database';

/** The container types that are databases (the container's docklite.type label). SQLite has no container, so it is not here. */
export const DB_CONTAINER_TYPES = ['postgres', 'mysql', 'mariadb', 'mongodb', 'redis'];

export interface DatabaseRecord {
  id: number;
  name: string;
  type?: DbType;
  container_id?: string | null;
  postgres_port?: number;
  db_path?: string;
  created_at: string;
  size: number;
  sizeCategory?: string;
  username?: string;
}

export function formatBytes(bytes: number) {
  if (!bytes) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1);
  return `${Math.round((bytes / Math.pow(k, i)) * 100) / 100} ${sizes[i]}`;
}
