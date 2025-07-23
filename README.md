<p align="left">
  <picture>
    <source
      srcset="assets/logo-dark.svg"
      media="(prefers-color-scheme: dark)"
    >
    <img
      src="assets/logo-light.svg"
      alt="Aviary Logo"
      width="200"
    >
  </picture>
</p>

A webhook-driven document uploader that automatically downloads and sends PDFs to your reMarkable tablet through a web interface or API. Compatible with both reMarkable Cloud and self-hosted [rmfakecloud](https://github.com/ddvk/rmfakecloud) via [rmapi](https://github.com/ddvk/rmapi), with features like automatic PDF conversion, compression, and organized file management.

## Why the Name?

**Aviary** is a place where birds are kept, chosen to evoke sending documents into the clouds using avian couriers.

## Features
**Frontend**
- Send from URL
- Send from local filesystem
- Toggle for compression for supported filetypes
- Destination directory selector with cache for instant loadtimes
- Light and dark themes, with optional system theme detection
- Single-user auth via environment variables OR multi-user mode with database
- User settings management (RMAPI host, API keys, default directories)
- Admin interface for user management and system configuration
- Internationalization: Support for 15 languages with auto-detected browser locale:
  - English (en)
  - Spanish (es)
  - German (de)
  - French (fr)
  - Dutch (nl)
  - Italian (it)
  - Portuguese (pt)
  - Norwegian (no)
  - Swedish (sv)
  - Danish (da)
  - Finnish (fi)
  - Polish (pl)
  - Japanese (ja)
  - Korean (ko)
  - Chinese Simplified (zh-CN)

**Backend**
- Webhook endpoint (`/api/webhook`) for SMS or HTTP integrations (e.g. Twilio)
- Multi-user support with SQLite or PostgreSQL database
- Per-user API key management with expiration and usage tracking
- User authentication with password reset via SMTP
- Per-user folder caching and document management
- Admin tools for user management and database backup/restore
- Automatic PDF download with a realistic browser User-Agent
- Automatic conversion of PNG and JPEG images to PDF
- Optional Ghostscript compression
- Two upload modes:
  - **Simple**: upload the raw PDF
  - **Managed**: rename by date, upload, then append the year locally & clean up files older than 7 days

## Screenshot

  <picture>
    <source
      srcset="assets/screenshot-dark.webp"
      media="(prefers-color-scheme: dark)"
    >
    <img
      src="assets/screenshot-light.webp"
      alt="Aviary UI Screenshot"
    >
  </picture>

## Environment Variable Configuration

### Core Configuration
| Variable                 | Required? | Default | Description |
|--------------------------|-----------|---------|-------------|
| DISABLE_UI               | No        | false   | Set `true` to disable the UI routes and run in API-only mode |
| PDF_DIR                  | No        | /app/pdfs| Directory to archive PDFs into |
| RMAPI_HOST               | No        |         | Self-hosted endpoint to use for rmapi (single-user mode only) |
| RMAPI_COVERPAGE          | No        |         | Set to `first` to add `--coverpage=1` flag to rmapi put commands |
| RM_TARGET_DIR            | No        | /       | Target folder on reMarkable device (single-user mode only) |
| GS_COMPAT                | No        | 1.7     | Ghostscript compatibility level |
| GS_SETTINGS              | No        | /ebook  | Ghostscript PDFSETTINGS preset |
| SNIFF_TIMEOUT            | No        | 5s      | Timeout for sniffing the MIME type |
| DOWNLOAD_TIMEOUT         | No        | 1m      | Timeout for Download requests |
| FOLDER_CACHE_INTERVAL    | No        | 1h      | How often to refresh the folder listing cache. `0` disables caching |
| DRY_RUN                  | No        | false   | Set to `true` to log rmapi commands without running them |

### Multi-User Mode Configuration
| Variable                 | Required? | Default | Description |
|--------------------------|-----------|---------|-------------|
| MULTI_USER               | No        | false   | Set to `true` to enable multi-user mode with database |
| ADMIN_EMAIL              | No        | username@localhost | Admin user email (used when creating initial admin from AUTH_USERNAME) |
| DATA_DIR                 | No        | /data   | Directory for database and user data storage |

### Database Configuration (Multi-User Mode)
| Variable                 | Required? | Default | Description |
|--------------------------|-----------|---------|-------------|
| DB_TYPE                  | No        | sqlite  | Database type: `sqlite` or `postgres` |
| DB_HOST                  | No        | localhost | PostgreSQL host (postgres only) |
| DB_PORT                  | No        | 5432    | PostgreSQL port (postgres only) |
| DB_USER                  | No        | aviary  | PostgreSQL username (postgres only) |
| DB_PASSWORD              | No        |         | PostgreSQL password (postgres only) |
| DB_NAME                  | No        | aviary  | PostgreSQL database name (postgres only) |
| DB_SSLMODE               | No        | disable | PostgreSQL SSL mode (postgres only) |

### SMTP Configuration (Multi-User Mode)
| Variable                 | Required? | Default | Description |
|--------------------------|-----------|---------|-------------|
| SMTP_HOST                | No        |         | SMTP server hostname for password resets |
| SMTP_PORT                | No        | 587     | SMTP server port |
| SMTP_USERNAME            | No        |         | SMTP username |
| SMTP_PASSWORD            | No        |         | SMTP password |
| SMTP_FROM                | No        |         | From email address for system emails |
| SMTP_TLS                 | No        | true    | Whether to use TLS for SMTP connection |

For more rmapi-specific configuration, see [their documentation](https://github.com/ddvk/rmapi?tab=readme-ov-file#environment-variables).

### Authentication Configuration
| Variable                 | Required? | Default | Description |
|--------------------------|-----------|---------|-------------|
| AUTH_USERNAME            | No        |         | Username for web UI login (single-user mode) or initial admin user (multi-user mode) |
| AUTH_PASSWORD            | No        |         | Password for web UI login (single-user mode) or initial admin user (multi-user mode) |
| API_KEY                  | No        |         | Secret key for API access via Authorization header (single-user mode only) |
| JWT_SECRET               | No        | auto-generated | Custom JWT signing secret (auto-generated if not provided.) If not set, restarting the container will log out all users. |
| ALLOW_INSECURE           | No        |  false  | Set to `true` to allow non-secure cookies (not recommended) |

## Authentication

Aviary supports two authentication modes:

### Single-User Mode (Default)
Traditional environment variable-based authentication for simple deployments:

#### Web UI Authentication
Set both `AUTH_USERNAME` and `AUTH_PASSWORD` to enable login-protected web interface:
```bash
AUTH_USERNAME=myuser
AUTH_PASSWORD=mypassword
```

#### API Key Authentication
Set `API_KEY` to protect programmatic access to API endpoints:
```bash
API_KEY=your-secret-api-key-here
```

Use the API key in requests with either header:
- `Authorization: Bearer your-api-key`
- `X-API-Key: your-api-key`

#### Flexible Authentication
- **No auth**: If neither UI nor API auth is configured, all endpoints are open
- **UI only**: Set `AUTH_USERNAME` + `AUTH_PASSWORD` to protect web interface only
- **API only**: Set `API_KEY` to protect API endpoints only
- **Both**: Set all three to enable both authentication methods
- **API endpoints accept either**: Valid API key OR valid web login session

### Multi-User Mode
Database-backed authentication with user management:

#### Enabling Multi-User Mode
Set `MULTI_USER=true` and configure database settings. If `AUTH_USERNAME` and `AUTH_PASSWORD` are set, they will be used to create the initial admin user:
```bash
MULTI_USER=true
AUTH_USERNAME=admin
AUTH_PASSWORD=secure-admin-password
ADMIN_EMAIL=admin@example.com
```

#### Features
- **User Registration**: Admin can create/manage user accounts
- **Per-User API Keys**: Each user can generate multiple API keys with expiration
- **Per-User Settings**: Individual RMAPI_HOST, default directories, and cover page preferences
- **Password Reset**: Email-based password reset via SMTP
- **Admin Interface**: User management, system settings, database & storage backup/restore
- **Database Support**: SQLite (default) or PostgreSQL for production
- **Per-User Data**: Separate document storage and folder cache per user

## Translations

All translations were generated by AI and may contain errors or cultural inaccuracies. I welcome contributions from native speakers to improve translation quality!

**Contributing translations:**
- To correct existing translations or add support for additional languages, please [open a GitHub issue](https://github.com/rmitchellscott/Aviary/issues) or submit a pull request
- Translation files are located in `/locales/` directory
- Each language follows the same JSON structure as the English template

## Webhook POST parameters

The webhook endpoint supports two modes of operation:

### URL-based uploads (Form data)
| Parameter                | Required? | Example | Description |
|--------------------------|-----------|---------|-------------|
| Body                     | Yes       | https://pdfobject.com/pdf/sample.pdf | URL to PDF to download
| prefix                   | No        | Reports     | Folder and file-name prefix, only used if `manage` is also `true` |
| compress                 | No        | true/false  | Run Ghostscript compression |
| manage                   | No        | true/false  | Enable managed handling (renaming and cleanup) |
| archive                  | No        | true/false  | Download to PDF_DIR instead of /tmp |
| rm_dir                   | No        | Books       | Override default reMarkable upload directory |
| retention_days           | No        | 30          | Optional integer (in days) for cleanup if manage=true. Defaults to 7. |

### Document content uploads (JSON)
| Parameter                | Required? | Example | Description |
|--------------------------|-----------|---------|-------------|
| body                     | Yes       | base64-encoded content | Base64 encoded document content |
| contentType              | No        | application/pdf | MIME type of the document |
| filename                 | No        | document.pdf | Original filename |
| isContent                | Yes       | true | Must be set to true for content uploads |
| prefix                   | No        | Reports | Folder and file-name prefix |
| compress                 | No        | true/false | Run Ghostscript compression |
| manage                   | No        | true/false | Enable managed handling |
| archive                  | No        | true/false | Save to PDF_DIR instead of /tmp |
| rm_dir                   | No        | Books | Override default reMarkable upload directory |
| retention_days           | No        | 30 | Optional integer (in days) for cleanup |

**Supported content types:** PDF, JPEG, PNG, EPUB

### Example cURL

#### URL-based uploads (Form data)
```shell
# Basic request
curl -X POST http://localhost:8000/api/webhook \
  -d "Body=https://pdfobject.com/pdf/sample.pdf" \
  -d "prefix=Reports" \
  -d "compress=true" \
  -d "manage=true" \
  -d "rm_dir=Books"

# With API key authentication
curl -X POST http://localhost:8000/api/webhook \
  -H "Authorization: Bearer your-api-key" \
  -d "Body=https://pdfobject.com/pdf/sample.pdf" \
  -d "compress=true"
```

#### Document content uploads (JSON)
```shell
# Upload base64-encoded document content
curl -X POST http://localhost:8000/api/webhook \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "body": "JVBERi0xLjQKJcOkw7zDtsO4DQo...",
    "contentType": "application/pdf",
    "filename": "document.pdf",
    "isContent": true,
    "compress": "true",
    "rm_dir": "Books"
  }'

# Upload image content
curl -X POST http://localhost:8000/api/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "body": "iVBORw0KGgoAAAANSUhEUgAAA...",
    "contentType": "image/png",
    "filename": "screenshot.png",
    "isContent": true,
    "compress": "false"
  }'
```
## Integrations

* [AWS SES](https://github.com/rmitchellscott/aviary-integration-ses) - Lambda to provide emailed PDFs/ePubs to Aviary.


# Examples
The following examples are provided as a way to get started. Some adjustments may be required before production use, particularly regarding secret management.

## Set Up

### Pairing with reMarkable Cloud

Before using Aviary, you need to pair it with your reMarkable Cloud account. There are two ways to do this:

#### Method 1: Command Line Pairing (Recommended for Docker)
Get your device and user token file (rmapi.conf) by running:
```bash
docker run -it -e RMAPI_HOST=remarkable.mydomain.com ghcr.io/rmitchellscott/aviary pair
```

For the standard reMarkable Cloud, omit the `RMAPI_HOST`:
```bash
docker run -it ghcr.io/rmitchellscott/aviary pair
```

This will prompt you for an 8-character one-time code that you can get from https://my.remarkable.com/device/desktop/connect

Save the output as `rmapi.conf` and mount it into the container at `/root/.config/rmapi/rmapi.conf`.

#### Method 2: Web Interface Pairing
Alternatively, you can start Aviary without an rmapi.conf file and pair through the web interface:

1. Start Aviary (it will show a warning but continue to run)
2. Open the web interface
3. Click the "Pair" button
4. Enter the 8-character code as prompted
5. The pairing will be completed automatically

**Note:** In single-user mode, both methods write to the same location (`/root/.config/rmapi/rmapi.conf`). In multi-user mode, each user has their own pairing status managed through their profile settings.

### Multi-User Mode Pairing

In multi-user mode, pairing is handled per-user:

1. Each user must pair individually through their profile settings
2. Users can access pairing through Settings → Profile → Pair with reMarkable
3. Each user can configure their own RMAPI_HOST if using self-hosted rmfakecloud
4. Admin users can see which users are paired through the admin interface
5. User pairing data is stored in the database, not in filesystem configs


## Docker
```shell
# Basic usage (with pre-created rmapi.conf)
docker run -d \
-p 8000:8000 \
-v ~/rmapi.conf:/root/.config/rmapi/rmapi.conf \
ghcr.io/rmitchellscott/aviary

# Basic usage (pair through web interface)
docker run -d \
-p 8000:8000 \
-v ~/.config/rmapi:/root/.config/rmapi \
ghcr.io/rmitchellscott/aviary

# With authentication (single-user mode)
docker run -d \
-p 8000:8000 \
-e AUTH_USERNAME=myuser \
-e AUTH_PASSWORD=mypassword \
-e API_KEY=your-secret-api-key \
-v ~/.config/rmapi:/root/.config/rmapi \
ghcr.io/rmitchellscott/aviary

# Multi-user mode with SQLite
docker run -d \
-p 8000:8000 \
-e MULTI_USER=true \
-e AUTH_USERNAME=admin \
-e AUTH_PASSWORD=secure-admin-password \
-e ADMIN_EMAIL=admin@example.com \
-v ./data:/data \
ghcr.io/rmitchellscott/aviary
```

## Docker Compose

### Single-User Mode
```yaml
services:
  aviary:
    image: ghcr.io/rmitchellscott/aviary
    ports:
      - "8000:8000"
    environment:
      RMAPI_HOST: "${RMAPI_HOST}"
      # Optional authentication (uncomment to enable):
      # AUTH_USERNAME: "${AUTH_USERNAME}"
      # AUTH_PASSWORD: "${AUTH_PASSWORD}"
      # API_KEY: "${API_KEY}"
    volumes:
      # Option 1: Mount existing rmapi.conf file
      # - type: bind
      #   source: ~/rmapi.conf
      #   target: /root/.config/rmapi/rmapi.conf
      # Option 2: Mount directory for web interface pairing
      - type: bind
        source: ~/.config/rmapi
        target: /root/.config/rmapi
    restart: unless-stopped
```

### Multi-User Mode with SQLite
```yaml
services:
  aviary:
    image: ghcr.io/rmitchellscott/aviary
    ports:
      - "8000:8000"
    environment:
      MULTI_USER: "true"
      AUTH_USERNAME: "${AUTH_USERNAME}"  # Initial admin user
      AUTH_PASSWORD: "${AUTH_PASSWORD}"
      ADMIN_EMAIL: "${ADMIN_EMAIL}"
      # Optional SMTP for password resets:
      # SMTP_HOST: "${SMTP_HOST}"
      # SMTP_PORT: "${SMTP_PORT}"
      # SMTP_USERNAME: "${SMTP_USERNAME}"
      # SMTP_PASSWORD: "${SMTP_PASSWORD}"
      # SMTP_FROM: "${SMTP_FROM}"
    volumes:
      - type: bind
        source: ./data
        target: /data  # Database and user data storage
    restart: unless-stopped
```

### Multi-User Mode with PostgreSQL
```yaml
services:
  aviary:
    image: ghcr.io/rmitchellscott/aviary
    ports:
      - "8000:8000"
    environment:
      MULTI_USER: "true"
      AUTH_USERNAME: "${AUTH_USERNAME}"
      AUTH_PASSWORD: "${AUTH_PASSWORD}"
      ADMIN_EMAIL: "${ADMIN_EMAIL}"
      DB_TYPE: "postgres"
      DB_HOST: "postgres"
      DB_PORT: "5432"
      DB_USER: "${DB_USER}"
      DB_PASSWORD: "${DB_PASSWORD}"
      DB_NAME: "${DB_NAME}"
      DB_SSLMODE: "disable"
    volumes:
      - type: bind
        source: ./data
        target: /data
    depends_on:
      - postgres
    restart: unless-stopped

  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: "${DB_NAME}"
      POSTGRES_USER: "${DB_USER}"
      POSTGRES_PASSWORD: "${DB_PASSWORD}"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  postgres_data:
```

## Building Locally
### Requirements

- [Ghostscript](https://www.ghostscript.com/) (`gs` CLI)
- [ImageMagick](https://imagemagick.org/)
- [npm]()
- [rmapi](https://github.com/ddvk/rmapi) (must be installed & in your `$PATH`)
- Access to your reMarkable credentials (`rmapi` setup)

Ensure the requirements are installed and available in your PATH.
```shell
go generate # Generate the Vite static front-end
go build -o aviary
```
