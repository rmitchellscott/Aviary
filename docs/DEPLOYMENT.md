# Deployment Guide

This guide covers different ways to deploy Aviary, from simple Docker containers to production-ready multi-user setups with PostgreSQL.

## Set Up

### Pairing with reMarkable Cloud

Before using Aviary, you need to pair it with your reMarkable Cloud account. There are two ways to do this:

#### Method 1: Web Interface Pairing
You can start Aviary without an rmapi.conf file and pair through the web interface:

1. Start Aviary (it will show a warning but continue to run)
2. Open the web interface
3. Click the "Pair" button
4. Enter the 8-character code as prompted
5. The pairing will be completed automatically

**Note:** In single-user mode, both methods write to the same location (`/root/.config/rmapi/rmapi.conf`). This file is always stored in the filesystem regardless of storage backend configuration, so you must mount `/root/.config/rmapi/` as a volume for persistence. In multi-user mode, each user has their own pairing data stored in the database.

#### Method 2: Command Line Pairing (Recommended for stateless setups)
Get your device and user token file (rmapi.conf) by running:
```bash
docker run -it -e RMAPI_HOST=https://remarkable.mydomain.com ghcr.io/rmitchellscott/aviary pair
```

For the official reMarkable Cloud, omit the `RMAPI_HOST`:
```bash
docker run -it ghcr.io/rmitchellscott/aviary pair
```

This will prompt you for an 8-character one-time code.

Save the output as `rmapi.conf` and mount it into the container at `/root/.config/rmapi/rmapi.conf`.

### Multi-User Mode Pairing

In multi-user mode, pairing is handled per-user:

1. Each user must pair individually through their profile settings
2. Users can access pairing through the main page, or Settings → Profile → Pair
3. Each user can configure their own RMAPI_HOST if using self-hosted rmfakecloud
4. Admin users can see which users are paired through the admin interface
5. User pairing data is stored in the database

## Docker

### Basic Usage (Single-User Mode)

```shell
# Basic usage (with pre-created rmapi.conf)
docker run -d \
-p 8000:8000 \
-v ~/rmapi.conf:/root/.config/rmapi/rmapi.conf \
ghcr.io/rmitchellscott/aviary
```

```shell
# Basic usage (pair through web interface)
docker run -d \
-p 8000:8000 \
-v ~/.config/rmapi:/root/.config/rmapi \
ghcr.io/rmitchellscott/aviary
```

### With Authentication (Single-User Mode)

```shell
docker run -d \
-p 8000:8000 \
-e AUTH_USERNAME=myuser \
-e AUTH_PASSWORD=mypassword \
-e API_KEY=your-secret-api-key \
-v ~/.config/rmapi:/root/.config/rmapi \
ghcr.io/rmitchellscott/aviary
```

### Multi-User Mode with SQLite

```shell
# Option 1: Pre-create admin user
docker run -d \
-p 8000:8000 \
-e MULTI_USER=true \
-e AUTH_USERNAME=admin \
-e AUTH_PASSWORD=secure-admin-password \
-e ADMIN_EMAIL=admin@example.com \
-v ./data:/data \
ghcr.io/rmitchellscott/aviary

# Option 2: Let first user become admin
docker run -d \
-p 8000:8000 \
-e MULTI_USER=true \
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
      AUTH_USERNAME: "${AUTH_USERNAME}"  # Optional: pre-create admin user
      AUTH_PASSWORD: "${AUTH_PASSWORD}"  # Optional: pre-create admin user
      ADMIN_EMAIL: "${ADMIN_EMAIL}"      # Optional: admin email
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
      AUTH_USERNAME: "${AUTH_USERNAME}"  # Optional: pre-create admin user
      AUTH_PASSWORD: "${AUTH_PASSWORD}"  # Optional: pre-create admin user
      ADMIN_EMAIL: "${ADMIN_EMAIL}"      # Optional: admin email
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

### Build Steps

Ensure the requirements are installed and available in your PATH.
```shell
go generate # Generate the Vite static front-end
go build -o aviary
```
