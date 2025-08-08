# Configuration Reference

This guide covers all available environment variables for configuring Aviary.

## File-based Environment Variables

For security purposes, any environment variable can be read from a file instead of being set directly. To use this feature, append `_FILE` to any environment variable name and provide the path to a file containing the value.

**Examples:**
```bash
# Instead of setting the password directly:
SMTP_PASSWORD=mypassword

# You can store it in a file and reference it:
SMTP_PASSWORD_FILE=/run/secrets/smtp_password

# This works for any environment variable:
API_KEY_FILE=/run/secrets/api_key
JWT_SECRET_FILE=/run/secrets/jwt_secret
DB_PASSWORD_FILE=/run/secrets/db_password
AUTH_PASSWORD_FILE=/run/secrets/auth_password
```

This is particularly useful when using Docker secrets, Kubernetes secrets, or other secure credential management systems. The file contents will be read and whitespace will be trimmed automatically.

## Core Configuration

| Variable                 | Required? | Default | Description |
|--------------------------|-----------|---------|-------------|
| PORT                     | No        | 8000    | Port for the web server to listen on |
| GIN_MODE                 | No        | release | Gin web framework mode (`release`, `debug`, or `test`) |
| DISABLE_UI               | No        | false   | Set `true` to disable the UI routes and run in API-only mode |
| PDF_DIR                  | No        | /app/pdfs| Directory to archive PDFs into (filesystem storage only) |
| RMAPI_HOST               | No        |         | Self-hosted endpoint to use for rmapi (single-user mode only) |
| RMAPI_COVERPAGE          | No        |         | Set to `first` to add `--coverpage=1` flag to rmapi put commands, used as the default in multi-user mode |
| RMAPI_CONFLICT_RESOLUTION| No        | abort   | Default conflict resolution mode: `abort`, `overwrite`, or `content_only` |
| RMAPI_FOLDER_DEPTH_LIMIT | No        | 0       | Limit folder traversal depth (0 = no limit, used as the default in multi-user mode) |
| RMAPI_FOLDER_EXCLUSION_LIST | No     |         | Comma-separated list of folder names to exclude (e.g., `trash,templates,archive`) |
| RM_TARGET_DIR            | No        | /       | Target folder on reMarkable device (single-user mode only) |
| GS_COMPAT                | No        | 1.7     | Ghostscript compatibility level |
| GS_SETTINGS              | No        | /ebook  | Ghostscript PDFSETTINGS preset |
| SNIFF_TIMEOUT            | No        | 5s      | Timeout for sniffing the MIME type |
| DOWNLOAD_TIMEOUT         | No        | 1m      | Timeout for Download requests |
| FOLDER_CACHE_INTERVAL    | No        | 1h      | How often to refresh the folder listing cache. `0` disables caching |
| FOLDER_REFRESH_RATE      | No        | 0.2     | Rate of folder refreshes per second (e.g., "0.2" for one refresh every 5 seconds) |
| PAGE_RESOLUTION          | No        | 1404x1872 | Page resolution for PDF conversion (WIDTHxHEIGHT format), used as the default in multi-user mode |
| PAGE_DPI                 | No        | 226     | Page DPI for PDF conversion, used as the default in multi-user mode |
| DRY_RUN                  | No        | false   | Set to `true` to log rmapi commands without running them |
| MAX_UPLOAD_SIZE          | No        | 524288000 | Maximum file upload size in bytes (default: 500MB) |

For more rmapi-specific configuration, see [their documentation](https://github.com/ddvk/rmapi?tab=readme-ov-file#environment-variables).

## Multi-User Mode Configuration

| Variable                 | Required? | Default | Description |
|--------------------------|-----------|---------|-------------|
| MULTI_USER               | No        | false   | Set to `true` to enable multi-user mode with database |
| ADMIN_EMAIL              | No        | username@localhost | Admin user email (used when creating initial admin from AUTH_USERNAME, if provided) |
| DATA_DIR                 | No        | /data   | Directory for database and user data storage (filesystem backend only) |

## Storage Backend Configuration

| Variable                 | Required? | Default | Description |
|--------------------------|-----------|---------|-------------|
| STORAGE_BACKEND          | No        | filesystem | Storage backend type: `filesystem` or `s3` |
| S3_ENDPOINT              | No        |         | S3-compatible endpoint URL (optional for AWS S3, required for other S3-compatible services) |
| S3_REGION                | No        | us-east-1 | S3 region |
| S3_BUCKET                | No        |         | S3 bucket name (required for S3 backend) |
| S3_ACCESS_KEY_ID         | No        |         | S3 access key ID (required for S3 backend) |
| S3_SECRET_ACCESS_KEY     | No        |         | S3 secret access key (required for S3 backend) |
| S3_FORCE_PATH_STYLE      | No        | false   | Force path-style S3 URLs (required for some S3-compatible services) |

### Storage Backend Notes

- **Filesystem backend**: Default option. 
   - `DATA_DIR`: Multi-user mode, primary storage for user data, database, and archived documents. 
   - `PDF_DIR`: Single-user mode, directory for archived PDFs 
- **S3 backend**: Stores archived documents and backups in S3-compatible object storage
- **Single-user mode limitation**: In single-user mode, only archived documents use the storage backend. The `rmapi.conf` file is always stored in the filesystem at `/root/.config/rmapi/rmapi.conf` and must be mounted as a volume for persistence. `PDF_DIR` is ignored when using S3 storage backend
- **Migration constraint**: Single-user to multi-user migration requires using the same storage backend. For cross-backend migrations, see [Data Management](docs/DATA_MANAGEMENT.md)
- **Database storage**: SQLite databases are always stored in the `DATA_DIR` and require volume mounts. For stateless deployment, use PostgreSQL with S3 storage backend

## Database Configuration (Multi-User Mode)

| Variable                 | Required? | Default | Description |
|--------------------------|-----------|---------|-------------|
| DB_TYPE                  | No        | sqlite  | Database type: `sqlite` or `postgres` |
| DB_HOST                  | No        | localhost | PostgreSQL host (postgres only) |
| DB_PORT                  | No        | 5432    | PostgreSQL port (postgres only) |
| DB_USER                  | No        | aviary  | PostgreSQL username (postgres only) |
| DB_PASSWORD              | No        |         | PostgreSQL password (postgres only) |
| DB_NAME                  | No        | aviary  | PostgreSQL database name (postgres only) |
| DB_SSLMODE               | No        | disable | PostgreSQL SSL mode (postgres only) |

## SMTP Configuration (Multi-User Mode)

| Variable                 | Required? | Default | Description |
|--------------------------|-----------|---------|-------------|
| SMTP_HOST                | No        |         | SMTP server hostname for password resets |
| SMTP_PORT                | No        | 587     | SMTP server port |
| SMTP_USERNAME            | No        |         | SMTP username |
| SMTP_PASSWORD            | No        |         | SMTP password |
| SMTP_FROM                | No        |         | From email address for system emails |
| SMTP_TLS                 | No        | true    | Whether to use TLS for SMTP connection |
| SITE_URL                 | No        | http://localhost:8000 | Base URL for the site (used in email links) |
| DISABLE_WELCOME_EMAIL    | No        | false   | Set to `true` to disable sending welcome emails to new users |

## Authentication Configuration

| Variable                 | Required? | Default | Description |
|--------------------------|-----------|---------|-------------|
| AUTH_USERNAME            | No        |         | Username for web UI login (single-user mode) or initial admin user (multi-user mode, optional) |
| AUTH_PASSWORD            | No        |         | Password for web UI login (single-user mode) or initial admin user (multi-user mode, optional) |
| API_KEY                  | No        |         | Secret key for API access via Authorization header (single-user mode only) |
| JWT_SECRET               | No        | auto-generated | Custom JWT signing secret (auto-generated if not provided.) If not set, restarting the container will log out all users. |
| ALLOW_INSECURE           | No        |  false  | Set to `true` to allow non-secure cookies (not recommended) |
| PROXY_AUTH_HEADER        | No        |         | Header name for proxy-based authentication |
| SESSION_TIMEOUT          | No        | 24h     | Lifetime of login sessions (e.g., `24h`, `30d`) |

## OIDC Authentication Configuration

| Variable                 | Required? | Default | Description |
|--------------------------|-----------|---------|-------------|
| OIDC_ISSUER              | No        |         | OIDC issuer URL |
| OIDC_CLIENT_ID           | No        |         | OIDC client ID |
| OIDC_CLIENT_SECRET       | No        |         | OIDC client secret |
| OIDC_REDIRECT_URL        | No        |         | OIDC redirect URL |
| OIDC_SCOPES              | No        |         | OIDC scopes (space-separated) |
| OIDC_AUTO_CREATE_USERS   | No        | false   | Set to `true` to automatically create users from OIDC claims |
| OIDC_ADMIN_GROUP         | No        |         | OIDC group name for admin role assignment. Users in this group become admins. Role management via native UI is disabled |
| OIDC_SSO_ONLY            | No        | false   | Set to `true` to hide traditional login form and show only OIDC login button |
| OIDC_BUTTON_TEXT         | No        |         | Custom text to override the OIDC login button with |
| OIDC_SUCCESS_REDIRECT_URL | No       |         | URL to redirect to after successful OIDC authentication |
| OIDC_POST_LOGOUT_REDIRECT_URL | No   |         | URL to redirect to after OIDC logout |
| OIDC_DEBUG               | No        | false   | Log debug messages related to OIDC lookup, linking, and claims |

## Configuration Examples

### Minimal Single-User Setup
```bash
# Basic setup with no authentication
PORT=8000
```

### Single-User with Authentication
```bash
PORT=8000
AUTH_USERNAME=admin
AUTH_PASSWORD=secure-password
API_KEY=your-secret-api-key-here
```

### Multi-User with SQLite
```bash
MULTI_USER=true
```

### Multi-User with PostgreSQL
```bash
MULTI_USER=true

DB_TYPE=postgres
DB_HOST=postgres
DB_PORT=5432
DB_USER=aviary
DB_PASSWORD=secure-db-password
DB_NAME=aviary
DB_SSLMODE=disable
```

### Production Multi-User with SMTP
```bash
MULTI_USER=true

DB_TYPE=postgres
DB_HOST=postgres
DB_PORT=5432
DB_USER=aviary
DB_PASSWORD=secure-db-password
DB_NAME=aviary
DB_SSLMODE=require

SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=noreply@example.com
SMTP_PASSWORD=smtp-app-password
SMTP_FROM=noreply@example.com
SMTP_TLS=true

SITE_URL=https://aviary.example.com
JWT_SECRET=your-long-random-jwt-secret-here
```

### With AWS S3 Storage Backend
```bash
MULTI_USER=true

# AWS S3 storage configuration
STORAGE_BACKEND=s3
S3_REGION=us-west-2
S3_BUCKET=my-aviary-bucket
S3_ACCESS_KEY_ID=AKIA...
S3_SECRET_ACCESS_KEY=secret-key-here
# S3_ENDPOINT not needed for AWS S3 - uses default endpoints

DB_TYPE=postgres
DB_HOST=postgres
DB_PORT=5432
DB_USER=aviary
DB_PASSWORD=secure-db-password
DB_NAME=aviary
```

### With MinIO/Self-hosted S3
```bash
MULTI_USER=true

# MinIO configuration
STORAGE_BACKEND=s3
S3_ENDPOINT=https://minio.example.com
S3_REGION=us-east-1
S3_BUCKET=aviary
S3_ACCESS_KEY_ID=minioadmin
S3_SECRET_ACCESS_KEY=minioadmin
S3_FORCE_PATH_STYLE=true
```

### With File-based Secrets
```bash
MULTI_USER=true
AUTH_USERNAME=admin
AUTH_PASSWORD_FILE=/run/secrets/auth_password
ADMIN_EMAIL=admin@example.com

DB_TYPE=postgres
DB_HOST=postgres
DB_PORT=5432
DB_USER=aviary
DB_PASSWORD_FILE=/run/secrets/db_password
DB_NAME=aviary

SMTP_HOST=smtp.gmail.com
SMTP_PASSWORD_FILE=/run/secrets/smtp_password
SMTP_FROM=noreply@example.com

JWT_SECRET_FILE=/run/secrets/jwt_secret
API_KEY_FILE=/run/secrets/api_key
```

## Environment File (.env)

You can also use a `.env` file in your project directory:

```bash
# .env file example
MULTI_USER=true
AUTH_USERNAME=admin
AUTH_PASSWORD=secure-password
ADMIN_EMAIL=admin@localhost
```

**Note:** Environment variables set in your shell or Docker configuration will override values in the `.env` file.
