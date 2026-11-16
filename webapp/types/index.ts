// User types
export type UserRole = 'super_admin' | 'admin' | 'user';

export interface User {
  id: number;
  username: string;
  password_hash: string;
  is_admin: number; // SQLite uses 0/1 for boolean (keep for backward compatibility)
  role: UserRole;
  is_super_admin: number; // SQLite uses 0/1 for boolean
  managed_by: number | null;
  created_at: string;
}

export interface UserSession {
  userId: number;
  username: string;
  isAdmin: boolean; // Keep for backward compatibility
  role: UserRole;
}

// Site types
export interface Site {
  id: number;
  domain: string;
  user_id: number;
  username: string;
  container_id: string | null;
  template_type: 'static' | 'php' | 'node';
  code_path: string;
  status: string;
  folder_id: number | null;
  created_at: string;
}

export interface CreateSiteParams {
  domain: string;
  user_id: number;
  template_type: 'static' | 'php' | 'node';
  container_id?: string;
  code_path?: string;
  status?: string;
  folder_id?: number | null;
}

// Database types
export interface Database {
  id: number;
  name: string;
  type: 'postgres' | 'sqlite';
  container_id: string;
  postgres_port: number;
  db_path?: string;
  created_at: string;
}

export interface CreateDatabaseParams {
  name: string;
  type: 'postgres' | 'sqlite';
  container_id: string;
  postgres_port: number;
  db_path?: string;
}

// Permission types
export interface DatabasePermission {
  id: number;
  user_id: number;
  database_id: number;
  created_at: string;
}

// Folder types
export interface Folder {
  id: number;
  user_id: number;
  name: string;
  parent_folder_id: number | null;
  depth: number;
  position: number;
  created_at: string;
}

// A group: a card that holds boxes (see go-app/internal/store/groups.go).
export interface BoxGroup {
  id: number;
  name: string;
  owner_id: number;
  owner_username?: string;
  linked: boolean;
  color?: string;
  icon?: string;
  members: string[]; // container names
}

export interface FolderNode extends Folder {
  children: FolderNode[];
  containers: any[]; // Will be ContainerInfo[] but defined later
}

export interface FolderContainer {
  id: number;
  folder_id: number;
  container_id: string;
  position: number;
  created_at: string;
}

// Container types (from Docker)
export interface ContainerInfo {
  id: string;
  name: string;
  status: string;
  state: string;
  uptime: string;
  image: string;
  ports: string;
  labels?: { [key: string]: string };
  owner_username?: string;
  style?: { color?: string; icon?: string }; // card colour and icon the owner picked (lib/box-styles.ts)
}

export interface ContainerStats {
  cpu: number;
  memory: {
    used: number;
    total: number;
    percentage: number;
  };
}

// Backup types
export type BackupDestinationType = 'local' | 'sftp' | 's3' | 'gdrive' | 'backblaze';
export type BackupTargetType = 'site' | 'database' | 'all-sites' | 'all-databases';
export type BackupStatus = 'success' | 'failed' | 'in_progress';

export interface BackupDestination {
  id: number;
  name: string;
  type: BackupDestinationType;
  config: string; // JSON string with connection details
  enabled: number; // SQLite boolean
  created_at: string;
}

export interface BackupDestinationConfig {
  // Local
  path?: string;
  // SFTP
  host?: string;
  port?: number;
  username?: string;
  password?: string;
  privateKey?: string;
  remotePath?: string;
  // S3/Backblaze
  endpoint?: string;
  region?: string;
  bucket?: string;
  accessKeyId?: string;
  secretAccessKey?: string;
  // Google Drive
  clientId?: string;
  clientSecret?: string;
  refreshToken?: string;
  folderId?: string;
}

export interface BackupJob {
  id: number;
  destination_id: number;
  target_type: BackupTargetType;
  target_id: number | null;
  frequency: string; // cron expression or simple: "daily", "every-3-days", etc.
  retention_days: number;
  enabled: number; // SQLite boolean
  last_run_at: string | null;
  next_run_at: string | null;
  created_at: string;
}

export interface Backup {
  id: number;
  job_id: number | null;
  destination_id: number;
  target_type: 'site' | 'database';
  target_id: number;
  backup_path: string;
  size_bytes: number;
  status: BackupStatus;
  error_message: string | null;
  created_at: string;
}

export interface CreateBackupDestinationParams {
  name: string;
  type: BackupDestinationType;
  config: BackupDestinationConfig;
  enabled?: number;
}

export interface CreateBackupJobParams {
  destination_id: number;
  target_type: BackupTargetType;
  target_id?: number | null;
  frequency: string;
  retention_days?: number;
  enabled?: number;
}

// DNS types
export type DNSRecordType = 'A' | 'AAAA' | 'CNAME' | 'MX' | 'TXT' | 'NS' | 'SRV' | 'CAA';

export interface CloudflareConfig {
  id: number;
  api_token: string | null;
  account_id: string | null;
  enabled: number;
  created_at: string;
  updated_at: string;
}

export interface DNSZone {
  id: number;
  domain: string;
  zone_id: string;
  account_id: string | null;
  enabled: number;
  last_synced_at: string | null;
  created_at: string;
}

export interface DNSRecord {
  id: number;
  zone_id: number;
  cloudflare_record_id: string | null;
  type: DNSRecordType;
  name: string;
  content: string;
  ttl: number;
  priority: number | null;
  proxied: number;
  created_at: string;
  updated_at: string;
}

export interface CreateDNSZoneParams {
  domain: string;
  zone_id: string;
  account_id?: string | null;
  enabled?: number;
}

export interface CreateDNSRecordParams {
  zone_id: number;
  cloudflare_record_id?: string | null;
  type: DNSRecordType;
  name: string;
  content: string;
  ttl?: number;
  priority?: number | null;
  proxied?: number;
}
