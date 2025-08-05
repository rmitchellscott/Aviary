# Data Management

This guide covers backup, restore, and database migration operations for Aviary.

## Overview

Aviary provides comprehensive data management capabilities through its built-in export/import system. This system can handle:

- **Complete system backups**: Database + user files + configurations
- **Selective exports**: Specific users or data types
- **Database migrations**: SQLite ↔ PostgreSQL
- **Disaster recovery**: Full system restoration

This system is included for ease of use and to aide in cross-database migrations. You can also backup the system yourself outside of Aviary by:
- Backing up the database (SQLite or Postgres)
- Backing up the storage directory

## Backup Operations

### Creating Backups

**Admin Interface**:
1. Log in as an admin user
2. Navigate to Admin Panel → System Settings
3. Click "Backup & Restore"
4. Click "Create Backup"
5. Monitor job progress in "Recent Backups" section
6. Download completed backups when ready

### Backup Metadata

Each backup includes metadata with:
- Aviary version and git commit
- Export timestamp
- Database type (SQLite/PostgreSQL)
- Users included in export
- Total document count and file sizes
- Exported database tables

## Backup File Structure

Aviary backups are compressed tar.gz archives with a standardized internal structure. Understanding this structure can help with troubleshooting and manual data recovery if needed.

> [!WARNING]  
> These backups contain sensitive information, such as usernames, hashed passwords, email addresses, users' reMarkable folder structure cache, and the device pairing file needed to interact with their cloud accounts. They should be treated accordingly.

### Archive Structure

```
backup-filename.tar.gz
├── metadata.json                    # Export metadata and compatibility info
├── database/                        # Database exports (JSON format)
│   ├── users.json                   # User accounts, settings, and rmapi configs
│   ├── api_keys.json                # API keys and permissions
│   ├── user_sessions.json           # Active user sessions
│   ├── documents.json               # Document metadata and references
│   ├── system_settings.json         # System-wide configuration
│   ├── login_attempts.json          # Authentication logs
│   └── user_folders_cache.json      # Cached folder structures
└── filesystem/                      # User files and configurations
    └── documents/                   # User-uploaded documents
        └── {user-id}/               # Per-user document storage
            ├── document1.pdf
            ├── subfolder/
            └── document2.pdf
```

### File Descriptions

**metadata.json**
Contains export information and compatibility data:
```json
{
  "aviary_version": "1.0.0",
  "git_commit": "abc123def",
  "export_timestamp": "2024-01-15T10:30:00Z",
  "database_type": "sqlite",
  "users_exported": ["user-uuid-1", "user-uuid-2"],
  "total_documents": 150,
  "total_size_bytes": 524288000,
  "exported_tables": ["users", "api_keys", "documents", ...]
}
```

**Database Files (database/)**
- **users.json**: User accounts, passwords (hashed), settings, preferences, and rmapi configurations
- **api_keys.json**: API keys, expiration dates, and usage tracking
- **user_sessions.json**: Active login sessions and JWT tokens
- **documents.json**: Document metadata including filenames, upload dates, and file paths
- **system_settings.json**: System-wide configuration and admin settings
- **login_attempts.json**: Authentication attempt logs and security data
- **user_folders_cache.json**: Cached reMarkable folder structures for performance

**Filesystem Files (filesystem/)**
- **documents/{user-id}/**: Documents archived by each user

### Database Table Structure

Each JSON file in the database directory contains an array of records from the corresponding database table. The structure matches the internal database schema:

**Example users.json structure:**
```json
[
  {
    "id": "uuid-string",
    "username": "admin",
    "email": "admin@example.com",
    "password": "$hashed_password",
    "is_admin": true,
    "is_active": true,
    "rmapi_host": "remarkable.cloud",
    "default_rmdir": "/Books",
    "coverpage_setting": "current",
    "rmapi_config": "[auth]\ndevicetoken=...\nusertoken=...\n",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
]
```

> [!NOTE]  
> As of v1.6.0, the `rmapi_config` field in the users table contains the complete rmapi configuration as a text string. This eliminates the need for separate config files in the filesystem.

### Manual Archive Access

You can manually extract and examine backup contents:

```bash
# Extract backup archive
tar -xzf backup-filename.tar.gz

# View metadata
cat metadata.json | jq '.'

# Examine database exports
cat database/users.json | jq '.'

# List user documents
ls -la filesystem/documents/*/
```

## Restore Operations

### Restore Process

> [!IMPORTANT]  
> Restore operations are destructive. Always backup current data first.

> [!NOTE]  
> Uploaded restore files and extracted archives are stored in temporary system storage (`/tmp`) and are automatically cleaned up after restoration or when cancelled.

1. Navigate to Admin Panel → System Settings → Backup & Restore
2. Click "Upload Restore"
3. Select and upload your `.tar.gz` backup file
4. Wait for upload and validation to complete
5. Click "Restore" on the uploaded file
6. Confirm

### Restore Validation

The system validates backups during upload:
- Checks backup file integrity and format
- Validates metadata compatibility
- Warns about version differences
- Analyzes backup contents before restoration

## Migrations

### SQLite to PostgreSQL

To migrate from SQLite to PostgreSQL:

1. **Create backup** from admin interface while running on SQLite
2. Download backup file
3. **Set up PostgreSQL** database and configure connection
4. **Update environment variables**:
   ```bash
   DB_TYPE=postgres
   DB_HOST=your-postgres-host
   DB_PORT=5432
   DB_USER=aviary
   DB_PASSWORD=your-password
   DB_NAME=aviary
   ```
5. **Restart Aviary** (creates new PostgreSQL schema)
6. **Restore backup** through admin interface

### PostgreSQL to SQLite

To migrate from PostgreSQL to SQLite:

1. **Create backup** from admin interface while running on PostgreSQL
2. Download backup file
3. **Update environment variables**:
   ```bash
   DB_TYPE=sqlite
   # Remove PostgreSQL-specific variables
   ```
4. **Restart Aviary** (creates new SQLite database)
5. **Restore backup** through admin interface

## User Data Structure

### Multi-User Mode Data Layout

```
/data/
├── aviary.db                 # SQLite database (if using SQLite)
└── users/
    └── {user-id}/
        └── pdfs/             # User's uploaded documents
            ├── document1.pdf
            └── subfolder/
```

> [!NOTE]  
> As of v1.6.0, rmapi configurations are stored in the database (in the `rmapi_config` column of the users table) rather than in the filesystem.

### Single-User to Multi-User Migration

When enabling multi-user mode (`MULTI_USER=true`), Aviary automatically performs a single-user to multi-user migration. For detailed migration procedures and troubleshooting, see the [Migration Guide](docs/MIGRATION_GUIDE.md).

**Important**: Single-user to multi-user migration requires using the same storage backend. If you need to change storage backends, use the cross-storage-backend migration process below.

## Cross-Storage-Backend Migration

To migrate between different storage backends (e.g., filesystem to S3, or S3 to filesystem), use the backup and restore process. For comprehensive migration instructions including verification steps and troubleshooting, see the [Migration Guide](docs/MIGRATION_GUIDE.md).

### Filesystem to S3 Migration

1. **Create a backup** while running with filesystem storage:
   ```bash
   # Current environment with filesystem storage
   STORAGE_BACKEND=filesystem  # or unset (defaults to filesystem)
   DATA_DIR=/data
   ```
   
2. **Download the backup** from the admin interface

3. **Update environment variables** for S3 storage:
   ```bash
   STORAGE_BACKEND=s3
   S3_ENDPOINT=https://s3.amazonaws.com
   S3_REGION=us-east-1
   S3_BUCKET=my-aviary-bucket
   S3_ACCESS_KEY_ID=your-access-key
   S3_SECRET_ACCESS_KEY=your-secret-key
   ```

4. **Restart Aviary** with the new configuration

5. **Restore the backup** through the admin interface

### S3 to Filesystem Migration

1. **Create a backup** while running with S3 storage:
   ```bash
   # Current environment with S3 storage
   STORAGE_BACKEND=s3
   S3_BUCKET=my-aviary-bucket
   # ... other S3 config
   ```

2. **Download the backup** from the admin interface

3. **Update environment variables** for filesystem storage:
   ```bash
   STORAGE_BACKEND=filesystem  # or unset (defaults to filesystem)
   DATA_DIR=/data
   # Remove S3-specific variables
   ```

4. **Restart Aviary** with the new configuration

5. **Restore the backup** through the admin interface

### S3 Provider Migration

To migrate between different S3 providers (e.g., AWS S3 to MinIO):

1. **Create a backup** with the current S3 configuration
2. **Update S3 environment variables** for the new provider:
   ```bash
   STORAGE_BACKEND=s3
   S3_ENDPOINT=https://minio.example.com     # New endpoint
   S3_BUCKET=new-bucket-name                 # New bucket
   S3_FORCE_PATH_STYLE=true                  # Often required for MinIO
   # ... update credentials as needed
   ```
3. **Restart Aviary** with the new configuration
4. **Restore the backup** through the admin interface

### Combined Migrations

Combining multiple migrations is supported, but may require two migration steps. 

Due to the backup capabilities exposed in multi-user mode it is recommended to perform the single-user → multi-user migration first before changing storage backends. 

Combined migrations between database and storage providers in a single step is fully supported by the restore process in multi-user mode.

1. **Stage 1**: Single-user → Multi-user (same storage)
   ```bash
   # Current: Single-user + filesystem + SQLite (implicit)
   MULTI_USER=true
   AUTH_USERNAME=admin
   AUTH_PASSWORD=secure-password
   # Keep: STORAGE_BACKEND=filesystem, DB_TYPE=sqlite
   ```

2. **Restart and verify** single→multi migration works

3. **Stage 2**: Create backup and configure S3 + PostgreSQL
   ```bash
   MULTI_USER=true
   STORAGE_BACKEND=s3
   S3_BUCKET=my-bucket
   # ... S3 config
   DB_TYPE=postgres
   DB_HOST=postgres-host
   # ... PostgreSQL config
   ```

4. **Restart and restore** backup

### Important Notes

- **Single-user rmapi.conf**: In single-user mode, the `rmapi.conf` file is always stored in the filesystem at `/root/.config/rmapi/rmapi.conf`, regardless of storage backend. Ensure this is properly mounted as a volume for persistence.

- **Backup verification**: Always verify that backups complete successfully before changing storage backends. Check the backup metadata and file counts.

- **Temporary storage**: Backup files are temporarily stored in `/tmp` during the restore process and are automatically cleaned up.

## Stateless Container Deployment

### Local Storage Requirements

Regardless of `STORAGE_BACKEND` setting, these components are **always stored locally**:

| Component | Location | Volume Mount Required |
|-----------|----------|----------------------|
| SQLite database | `/data/aviary.db` | Yes |
| Single-user rmapi.conf | `/root/.config/rmapi/rmapi.conf` | Yes |
| Temporary files | `/tmp` | No (ephemeral) |

### Stateless Configuration

The Aviary container can run **completely stateless** (no volume mounts) only with:

```bash
MULTI_USER=true           # Multi-user mode
DB_TYPE=postgres          # External PostgreSQL database  
STORAGE_BACKEND=s3        # S3-compatible object storage
# + PostgreSQL and S3 connection details
```

In this configuration:
- **Database**: External PostgreSQL server
- **Documents**: S3-compatible object storage
- **User configs**: Stored in PostgreSQL database
- **No local persistence needed**

### Configurations Requiring Volume Mounts

| Configuration | Required Mounts | Reason |
|---------------|----------------|---------|
| Single-user mode | `/root/.config/rmapi/` | rmapi.conf always filesystem-based |
| SQLite database | `/data` | Database file stored locally |
| Filesystem storage | `/data` | Documents stored locally |
