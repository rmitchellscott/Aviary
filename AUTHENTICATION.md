# Authentication Guide

Aviary supports multiple authentication methods that can be configured via environment variables. This guide covers all authentication options from simple single-user setups to enterprise-grade multi-user deployments with OIDC and proxy authentication.

## Authentication Modes

### Single-User Mode (Default)
Traditional environment variable-based authentication for simple deployments, this mode assumes a single reMarkable user.

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
Database-backed authentication with user management

#### Enabling Multi-User Mode
Set `MULTI_USER=true` and configure database settings. `AUTH_USERNAME` and `AUTH_PASSWORD` are required to be set for the first run, they will be used to create the initial admin user:
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

## Advanced Authentication

> [!IMPORTANT]  
> OIDC and Proxy Authentication require multi-user mode to be enabled (`MULTI_USER=true`).

### OIDC Authentication

OIDC allows users to authenticate using external identity providers like Keycloak, Authentik, Okta, etc.

#### Environment Variables

```bash
# Enable multi-user mode first
MULTI_USER=true

# OIDC Configuration
OIDC_ISSUER=https://auth.example.com/realms/aviary
OIDC_CLIENT_ID=aviary-client
OIDC_CLIENT_SECRET=your-oidc-client-secret
OIDC_REDIRECT_URL=https://aviary.example.com/api/auth/oidc/callback
OIDC_SCOPES=openid,profile,email
OIDC_AUTO_CREATE_USERS=true
OIDC_SUCCESS_REDIRECT_URL=https://aviary.example.com/
OIDC_POST_LOGOUT_REDIRECT_URL=https://aviary.example.com/
```

#### Configuration Details

- **OIDC_ISSUER**: The URL of your OIDC provider's issuer endpoint
- **OIDC_CLIENT_ID**: The client ID registered with your OIDC provider
- **OIDC_CLIENT_SECRET**: The client secret for your registered application
- **OIDC_REDIRECT_URL**: The callback URL where users are redirected after authentication (must match provider configuration)
- **OIDC_SCOPES**: Comma-separated list of OAuth2 scopes to request (defaults to "openid,profile,email")
- **OIDC_AUTO_CREATE_USERS**: Whether to automatically create user accounts for new OIDC users (true/false)
- **OIDC_SUCCESS_REDIRECT_URL**: Where to redirect users after successful login (optional, defaults to "/")
- **OIDC_POST_LOGOUT_REDIRECT_URL**: Where to redirect users after logout (optional)

### Proxy Authentication

Proxy authentication allows Aviary to trust authentication headers set by a reverse proxy like Traefik, nginx, or Apache. 

> [!IMPORTANT]  
> Users must be created manually in Aviary before they can authenticate via proxy.

#### Environment Variables

```bash
# Enable multi-user mode first
MULTI_USER=true

# Proxy Authentication Configuration
PROXY_AUTH_HEADER=X-Forwarded-User
```

#### Security Considerations

Proxy authentication assumes that your reverse proxy has already authenticated the user and is setting trusted headers. Ensure that:

1. Direct access to Aviary is blocked (only accessible through the proxy)
2. The proxy properly validates users before setting headers
3. Headers cannot be spoofed by external clients
4. Use HTTPS to prevent header manipulation

## User Management

### OIDC User Management
When OIDC is enabled:
- If `OIDC_AUTO_CREATE_USERS=true`, new users are automatically created on first login
- Users are identified by OIDC subject ID first, then by username, then by email for migration
- Existing users without OIDC subjects are automatically linked on first OIDC login
- User information is automatically updated from OIDC claims on each login
- Admin privileges are managed through Aviary's native user management UI

### Proxy Authentication User Management
When proxy authentication is enabled:
- Users must be created manually through Aviary's admin interface
- The proxy header must match the username field in Aviary exactly
- Admin privileges are managed through Aviary's native user management UI
- User accounts can be activated/deactivated through the admin interface

## Combined Authentication

In multi-user mode, you can enable multiple authentication methods simultaneously:

- **OIDC + Traditional Login**: Users can choose between SSO and username/password
- **Proxy Auth**: Takes precedence over other methods when enabled
- **API Keys**: Always available for programmatic access

## Migration from Single-User Mode

When enabling multi-user mode:

1. Set `MULTI_USER=true` in your environment
2. Set `AUTH_USERNAME` and `AUTH_PASSWORD` for the initial run
3. The env-based user becomes the administrator
4. Existing rmapi configuration is migrated to the admin user, if present
5. Existing PDF files are moved to the admin user's directory, if present
6. Environment-based API key is migrated to the admin user's account, if present

## API Access

API access continues to work with:
- **API Keys**: Generated through the web interface or migrated from environment variables
- **JWT Tokens**: Obtained through any authentication method

Example API usage:

```bash
# Using API key
curl -H "Authorization: Bearer your-api-key" https://aviary.example.com/api/status

# Using API key in header
curl -H "X-API-Key: your-api-key" https://aviary.example.com/api/status

# Using JWT cookie (after web login)
curl --cookie-jar cookies.txt https://aviary.example.com/api/status
```

## Troubleshooting

### OIDC Issues

1. **"OIDC not configured" error**
   - Ensure `MULTI_USER=true` is set
   - Verify `OIDC_ISSUER`, `OIDC_CLIENT_ID`, and `OIDC_CLIENT_SECRET` are set
   - Check that the issuer URL is accessible from your server

2. **"Invalid redirect URI" error**
   - Check that `OIDC_REDIRECT_URL` matches exactly what's configured in your OIDC provider
   - Ensure the URL is publicly accessible

3. **"User not found" error with auto-creation disabled**
   - Set `OIDC_AUTO_CREATE_USERS=true` to automatically create users
   - Or manually create the user account first

4. **Users cannot login on mobile**
   - Some mobile browsers (iOS) require HTTPS for OIDC
   - Enable HTTPS
   - Choose another authentication method

### Proxy Auth Issues

1. **"Proxy authentication header missing" error**
   - Ensure `MULTI_USER=true` is set
   - Verify your reverse proxy is setting the configured header
   - Check that the header name matches `PROXY_AUTH_HEADER`
   - Ensure direct access to Aviary is blocked to prevent header spoofing

2. **"User not found in database" error**
   - Create the user account manually in Aviary's admin interface first
   - Ensure the username in Aviary matches exactly what the proxy sends

3. **Admin privileges not working**
   - Use Aviary's native user management UI to promote users to admin
   - Check the Users admin panel to manage roles

### General Authentication Issues

1. **Authentication loops or redirects**
   - Check that cookies are being set correctly (HTTPS vs HTTP)
   - Verify `ALLOW_INSECURE=true` is set for HTTP environments
   - Ensure there are no conflicting authentication methods

2. **Users can't access after authentication**
   - Check that the user account is active
   - Verify JWT_SECRET is set and consistent across restarts
   - Ensure database is accessible
