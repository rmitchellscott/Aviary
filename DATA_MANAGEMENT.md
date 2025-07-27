# Data Management

This guide covers backup, restore, and database migration operations for Aviary.

## Overview

Aviary provides comprehensive data management capabilities through its built-in export/import system. This system can handle:

- **Complete system backups**: Database + user files + configurations
- **Selective exports**: Specific users or data types
- **Database migrations**: SQLite ↔ PostgreSQL
- **Disaster recovery**: Full system restoration

This system is included for easy of use and to aide in cross-database migrations. You can also backup the system yourself externally to Aviary by:
- Backing up the database (SQLite or Postgres)
- Backing up the storage directory

## Backup Operations

### Creating Backups

Backups are created through the Admin interface or via API calls. The system generates compressed `.tar.gz` archives containing:

- **Database data**: All tables exported as JSON files
- **User documents**: PDF files and other uploaded documents
- **User configurations**: rmapi configs and settings
- **Metadata**: Export information and compatibility data

### Background Job Processing

Aviary supports background backup processing:

- **Background jobs**: Backups are processed as background jobs
- **Job tracking**: Monitor backup progress through the admin interface
- **Download when ready**: Completed backups can be downloaded from the jobs panel

### Backup Options

When creating a backup via the Admin Panel, it will always be a full backup.

Via the API, you can choose what to include:

- **Include Database**: All user accounts, API keys, settings, and metadata
- **Include Files**: User-uploaded documents and PDFs
- **Include Configs**: rmapi configurations and user-specific settings
- **User Selection**: Export all users or specific users only

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
│   ├── users.json                   # User accounts and settings
│   ├── api_keys.json                # API keys and permissions
│   ├── user_sessions.json           # Active user sessions
│   ├── documents.json               # Document metadata and references
│   ├── system_settings.json         # System-wide configuration
│   ├── login_attempts.json          # Authentication logs
│   └── user_folders_cache.json      # Cached folder structures
└── filesystem/                      # User files and configurations
    ├── documents/                   # User-uploaded documents
    │   └── {user-id}/               # Per-user document storage
    │       ├── document1.pdf
    │       ├── subfolder/
    │       └── document2.pdf
    └── configs/                     # User configurations
        └── {user-id}/               # Per-user configurations
            └── rmapi.conf           # reMarkable API configuration
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
- **users.json**: User accounts, passwords (hashed), settings, and preferences
- **api_keys.json**: API keys, expiration dates, and usage tracking
- **user_sessions.json**: Active login sessions and JWT tokens
- **documents.json**: Document metadata including filenames, upload dates, and file paths
- **system_settings.json**: System-wide configuration and admin settings
- **login_attempts.json**: Authentication attempt logs and security data
- **user_folders_cache.json**: Cached reMarkable folder structures for performance

**Filesystem Files (filesystem/)**
- **documents/{user-id}/**: All PDF files and documents uploaded by each user
- **configs/{user-id}/**: User-specific configurations, primarily rmapi.conf files

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
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
]
```

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

# Check user configurations
ls -la filesystem/configs/*/
```

### Access Backup Interface

**Admin Interface**:
1. Log in as an admin user
2. Navigate to Admin Panel → System Settings
3. Click "Backup & Restore"
4. Click "Create Backup"
5. Monitor job progress in "Recent Jobs" section
6. Download completed backups when ready

**API Access - Background Job**:
```bash
curl -X POST http://localhost:8000/api/admin/backup-jobs \
  -H "Authorization: Bearer your-admin-api-key" \
  -d '{
    "includeDatabase": true,
    "includeFiles": true,
    "includeConfigs": true
  }'
```

## Restore Operations

### Enhanced Two-Step Restore Process

Aviary uses a two-step restore process:

1. **Upload Phase**: Upload and validate backup files
2. **Restore Phase**: Confirm and execute restoration

### Restore Workflow

**Step 1: Upload Backup File**
- Upload `.tar.gz` backup files through admin interface
- Files are validated and stored temporarily (24-hour expiration)
- Multiple files can be uploaded and managed
- File metadata and compatibility are checked

**Step 2: Confirm Restoration**
- Select uploaded file from pending uploads
- Configure restore options (overwrite settings, user selection)
- Confirm and execute restoration process
- Monitor progress through background job tracking

### Restore Options

- **Overwrite Files**: Replace existing user files
- **Overwrite Database**: Replace existing database content (⚠️ destructive)
- **User Selection**: Restore specific users only
- **Background Processing**: Large restores run as background jobs

### Restore Process

> [!IMPORTANT]  
> Restore operations are destructive. Always backup current data first.

**Admin Interface**:
1. Navigate to Admin Panel → System Settings → Backup & Restore
2. Click "Upload Restore"
3. Select and upload your `.tar.gz` backup file
4. Wait for upload and validation to complete
5. Click "Restore" on the uploaded file
6. Confirm
7. Monitor restoration progress in jobs panel

**API Access - Upload**:
```bash
curl -X POST http://localhost:8000/api/admin/restore/upload \
  -H "Authorization: Bearer your-admin-api-key" \
  -F "backup_file=@backup.tar.gz"
```

**API Access - Restore**:
```bash
curl -X POST http://localhost:8000/api/admin/restore \
  -H "Authorization: Bearer your-admin-api-key" \
  -d '{
    "upload_id": "uuid-from-upload",
    "overwrite_files": true,
    "overwrite_database": true,
    "user_ids": []
  }'
```

### Restore Validation

The system validates backups during upload:
- Checks backup file integrity and format
- Validates metadata compatibility
- Warns about version differences
- Confirms database type compatibility
- Analyzes backup contents before restoration

## Database Migration

### SQLite to PostgreSQL

To migrate from SQLite to PostgreSQL:

1. **Create full backup** from SQLite system
2. **Set up PostgreSQL** database and configure connection
3. **Update environment variables**:
   ```bash
   DB_TYPE=postgres
   DB_HOST=your-postgres-host
   DB_PORT=5432
   DB_USER=aviary
   DB_PASSWORD=your-password
   DB_NAME=aviary
   ```
4. **Restart Aviary** (creates new PostgreSQL schema)
5. **Restore backup** through admin interface

### PostgreSQL to SQLite

To migrate from PostgreSQL to SQLite:

1. **Create full backup** from PostgreSQL system
2. **Update environment variables**:
   ```bash
   DB_TYPE=sqlite
   # Remove PostgreSQL-specific variables
   ```
3. **Restart Aviary** (creates new SQLite database)
4. **Restore backup** through admin interface

## User Data Structure

### Multi-User Mode Data Layout

```
/data/
├── aviary.db                 # SQLite database (if using SQLite)
└── users/
    └── {user-id}/
        ├── pdfs/             # User's uploaded documents
        │   ├── document1.pdf
        │   └── subfolder/
        └── rmapi/            # User's rmapi configuration
            └── rmapi.conf
```

### Single-User to Multi-User Migration

When enabling multi-user mode (`MULTI_USER=true`), Aviary automatically migrates:

1. **Creates admin user** from `AUTH_USERNAME` and `AUTH_PASSWORD` if present, or from the first user to login
2. **Migrates rmapi config** from `/root/.config/rmapi/rmapi.conf`
3. **Moves archived files** from `PDF_DIR` to admin user directory
4. **Migrates API key** from `API_KEY` environment variable
5. **Sets user preferences** based on environment variables
