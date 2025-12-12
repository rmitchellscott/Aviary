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

### Web Interface
- Send documents from URL or local filesystem
- Compression toggle for supported file types
- Destination directory selector
- Light/dark themes with system detection
- Internationalization: 15 languages with auto-detected browser locale

### Authentication & User Management
- **Single-user mode**: Simple environment variable authentication
- **Multi-user mode**: Database-backed user management with admin interface
  - Advanced auth: OIDC and proxy authentication support
  - Per-user API key management with expiration tracking
  - Password reset via e-mail

### Document Processing
- Webhook endpoint for HTTP integrations
- Automatic PDF download with realistic browser User-Agent
- Web article extraction (using Mozilla Readability algorithm)
- Markdown and HTML to PDF/EPUB conversion
- PNG/JPEG to PDF conversion
- Optional Ghostscript compression
- Configurable conflict resolution (abort/overwrite/content-only)
- Smart upload modes (simple or managed with retention via API)

### Data Management (Mulit-User Mode)
- SQLite (default) or PostgreSQL database support
- Complete backup/restore system with cross-database migration
- Per-user document storage and folder caching
- Admin tools for user management and system settings

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

## Quick Start

### Simple Docker Setup
```bash
# Basic usage (pair through web interface)
docker run -d -p 8000:8000 -v ~/.config/rmapi:/root/.config/rmapi ghcr.io/rmitchellscott/aviary
```

### Multi-User Setup
```bash
# Multi-user mode - first user becomes admin
docker run -d -p 8000:8000 \
  -e MULTI_USER=true \
  -e ALLOW_INSECURE=true \
  -v ./data:/data \
  ghcr.io/rmitchellscott/aviary

# Or pre-create admin user
docker run -d -p 8000:8000 \
  -e MULTI_USER=true \
  -e ALLOW_INSECURE=true \
  -e AUTH_USERNAME=admin \
  -e AUTH_PASSWORD=secure-password \
  -v ./data:/data \
  ghcr.io/rmitchellscott/aviary
```

Open http://localhost:8000 and pair with your reMarkable account.

## Documentation

- **[Configuration](docs/CONFIGURATION.md)** - Environment variables and settings
- **[Authentication](docs/AUTHENTICATION.md)** - User management, OIDC, and proxy auth
- **[Deployment](docs/DEPLOYMENT.md)** - Docker, Docker Compose, and production setup
- **[API Reference](docs/API.md)** - Webhook endpoints and integrations
- **[Data Management](docs/DATA_MANAGEMENT.md)** - Backup, restore, and database migration
- **[Translations](docs/TRANSLATIONS.md)** - Multi-language support and contributing

## Data Management

Aviary includes comprehensive backup and restore capabilities.

### Backup & Restore Features
- **Complete system backups**: Database + user files + configurations
- **Cross-database migration**: Migrate between SQLite and PostgreSQL  
- **Large backup support**: Background job processing for large datasets

### Migration & Data Safety
When migrating from single-user to multi-user mode, Aviary automatically:
- Creates initial admin user
- Migrates existing cloud pairing, archived PDF files, and API key
- Runs schema migrations for new features

See [Data Management](docs/DATA_MANAGEMENT.md) and [Migration from Single-User Mode](docs/AUTHENTICATION.md#Migration-from-Single-User-Mode) for more details.

## Integrations

* [AWS SES Integration](https://github.com/rmitchellscott/aviary-integration-ses) - Lambda to provide emailed PDFs/ePubs to Aviary

See [API Reference](docs/API.md) for webhook details and integration examples.

## Building from Source

See [Deployment Guide](docs/DEPLOYMENT.md) for local build instructions and requirements.
