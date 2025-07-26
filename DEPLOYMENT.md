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

**Note:** In single-user mode, both methods write to the same location (`/root/.config/rmapi/rmapi.conf`). In multi-user mode, each user has their own pairing status managed through their profile settings.

#### Method 2: Command Line Pairing (Recommended for stateless setups)
Get your device and user token file (rmapi.conf) by running:
```bash
docker run -it -e RMAPI_HOST=https://remarkable.mydomain.com ghcr.io/rmitchellscott/aviary pair
```

For the standard reMarkable Cloud, omit the `RMAPI_HOST`:
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
5. User pairing status is stored in the database, with the user's rmapi.conf stored in a user-specific directory in the filesystem

## Docker

### Basic Usage

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

### Multi-User Mode with PostgreSQL and Docker Secrets
```yaml
services:
  aviary:
    image: ghcr.io/rmitchellscott/aviary
    ports:
      - "8000:8000"
    environment:
      MULTI_USER: "true"
      AUTH_USERNAME: "admin"
      AUTH_PASSWORD_FILE: "/run/secrets/auth_password"
      ADMIN_EMAIL: "admin@example.com"
      DB_TYPE: "postgres"
      DB_HOST: "postgres"
      DB_PORT: "5432"
      DB_USER: "aviary"
      DB_PASSWORD_FILE: "/run/secrets/db_password"
      DB_NAME: "aviary"
      DB_SSLMODE: "disable"
      JWT_SECRET_FILE: "/run/secrets/jwt_secret"
      SMTP_PASSWORD_FILE: "/run/secrets/smtp_password"
    secrets:
      - auth_password
      - db_password
      - jwt_secret
      - smtp_password
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
      POSTGRES_DB: "aviary"
      POSTGRES_USER: "aviary"
      POSTGRES_PASSWORD_FILE: "/run/secrets/db_password"
    secrets:
      - db_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

secrets:
  auth_password:
    file: ./secrets/auth_password.txt
  db_password:
    file: ./secrets/db_password.txt
  jwt_secret:
    file: ./secrets/jwt_secret.txt
  smtp_password:
    file: ./secrets/smtp_password.txt

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

## Production Considerations

### Security
- Always use HTTPS in production
- Use strong, randomly generated passwords and API keys
- Consider using Docker secrets for sensitive environment variables
- Regularly update to the latest version

### Performance
- For heavy usage, consider PostgreSQL over SQLite
- Monitor disk usage in the data directory
- Set up log rotation if needed

### Backup
- Regularly backup your data directory
- For PostgreSQL, use standard PostgreSQL backup tools
- See [Data Management](DATA_MANAGEMENT.md) for automated backup/restore options

### Monitoring
- Monitor container logs for errors
- Set up health checks on the `/api/status` endpoint
- Monitor disk space in the data directory
