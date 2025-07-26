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
| PDF_DIR                  | No        | /app/pdfs| Directory to archive PDFs into |
| RMAPI_HOST               | No        |         | Self-hosted endpoint to use for rmapi (single-user mode only) |
| RMAPI_COVERPAGE          | No        |         | Set to `first` to add `--coverpage=1` flag to rmapi put commands, used as the default in multi-user mode |
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

For more rmapi-specific configuration, see [their documentation](https://github.com/ddvk/rmapi?tab=readme-ov-file#environment-variables).

## Multi-User Mode Configuration

| Variable                 | Required? | Default | Description |
|--------------------------|-----------|---------|-------------|
| MULTI_USER               | No        | false   | Set to `true` to enable multi-user mode with database |
| ADMIN_EMAIL              | No        | username@localhost | Admin user email (used when creating initial admin from AUTH_USERNAME, if provided) |
| DATA_DIR                 | No        | /data   | Directory for database and user data storage |

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
| OIDC_SUCCESS_REDIRECT_URL | No       |         | URL to redirect to after successful OIDC authentication |
| OIDC_POST_LOGOUT_REDIRECT_URL | No   |         | URL to redirect to after OIDC logout |

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
AUTH_USERNAME=admin
AUTH_PASSWORD=secure-admin-password
ADMIN_EMAIL=admin@example.com
```

### Multi-User with PostgreSQL
```bash
MULTI_USER=true
AUTH_USERNAME=admin
AUTH_PASSWORD=secure-admin-password
ADMIN_EMAIL=admin@example.com

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
AUTH_USERNAME=admin
AUTH_PASSWORD=secure-admin-password
ADMIN_EMAIL=admin@example.com

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

## Configuration Validation

Aviary validates configuration on startup and will log warnings or errors for:
- Invalid timeout values
- Missing required fields when multi-user mode is enabled
- Invalid database connection parameters
- SMTP configuration issues

Check the startup logs to ensure your configuration is valid.
