-- Multi-user database schema for Aviary
-- Supports both SQLite and PostgreSQL

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT (CASE 
        WHEN (SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users') > 0 
        THEN hex(randomblob(16)) 
        ELSE gen_random_uuid() 
    END),
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    is_admin BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP,
    
    -- User-specific settings
    rmapi_host VARCHAR(255),
    default_rmdir VARCHAR(255) DEFAULT '/',
    coverpage_setting VARCHAR(255),
    folder_refresh_percent INTEGER DEFAULT 0,
    page_resolution VARCHAR(255),
    page_dpi DECIMAL(10,2),
    
    -- Password reset
    reset_token VARCHAR(255),
    reset_token_expires TIMESTAMP,
    
    -- Account verification
    email_verified BOOLEAN DEFAULT FALSE,
    verification_token VARCHAR(255)
);

-- API keys table (per user)
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT (CASE 
        WHEN (SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='api_keys') > 0 
        THEN hex(randomblob(16)) 
        ELSE gen_random_uuid() 
    END),
    user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(16) NOT NULL, -- First 16 chars for display
    is_active BOOLEAN DEFAULT TRUE,
    last_used TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- User sessions table (for JWT token management)
CREATE TABLE user_sessions (
    id UUID PRIMARY KEY DEFAULT (CASE 
        WHEN (SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='user_sessions') > 0 
        THEN hex(randomblob(16)) 
        ELSE gen_random_uuid() 
    END),
    user_id UUID NOT NULL,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_used TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    user_agent TEXT,
    ip_address VARCHAR(45),
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- User folders cache table (per user)
CREATE TABLE user_folders_cache (
    id UUID PRIMARY KEY DEFAULT (CASE 
        WHEN (SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='user_folders_cache') > 0 
        THEN hex(randomblob(16)) 
        ELSE gen_random_uuid() 
    END),
    user_id UUID NOT NULL,
    folder_path VARCHAR(1000) NOT NULL,
    folder_data TEXT, -- JSON data for folder contents
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(user_id, folder_path)
);

-- User uploads/downloads log table
CREATE TABLE user_documents (
    id UUID PRIMARY KEY DEFAULT (CASE 
        WHEN (SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='user_documents') > 0 
        THEN hex(randomblob(16)) 
        ELSE gen_random_uuid() 
    END),
    user_id UUID NOT NULL,
    document_name VARCHAR(255) NOT NULL,
    local_path VARCHAR(1000),
    remote_path VARCHAR(1000),
    document_type VARCHAR(50), -- 'pdf', 'epub', etc.
    file_size BIGINT,
    upload_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) DEFAULT 'uploaded', -- 'uploaded', 'failed', 'deleted'
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- System settings table (for admin configuration)
CREATE TABLE system_settings (
    key VARCHAR(255) PRIMARY KEY,
    value TEXT,
    description TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    
    FOREIGN KEY (updated_by) REFERENCES users(id)
);

-- Login attempts table (for rate limiting)
CREATE TABLE login_attempts (
    id UUID PRIMARY KEY DEFAULT (CASE 
        WHEN (SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='login_attempts') > 0 
        THEN hex(randomblob(16)) 
        ELSE gen_random_uuid() 
    END),
    ip_address VARCHAR(45) NOT NULL,
    username VARCHAR(255),
    success BOOLEAN DEFAULT FALSE,
    attempted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    user_agent TEXT
);

-- Create indexes for performance
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_reset_token ON users(reset_token);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX idx_user_sessions_token_hash ON user_sessions(token_hash);
CREATE INDEX idx_user_folders_cache_user_id ON user_folders_cache(user_id);
CREATE INDEX idx_user_documents_user_id ON user_documents(user_id);
CREATE INDEX idx_login_attempts_ip ON login_attempts(ip_address);
CREATE INDEX idx_login_attempts_username ON login_attempts(username);
CREATE INDEX idx_login_attempts_time ON login_attempts(attempted_at);

-- Insert default system settings
INSERT INTO system_settings (key, value, description) VALUES
    ('smtp_enabled', 'false', 'Whether SMTP is configured for password resets'),
    ('smtp_host', '', 'SMTP server hostname'),
    ('smtp_port', '587', 'SMTP server port'),
    ('smtp_username', '', 'SMTP username'),
    ('smtp_password', '', 'SMTP password'),
    ('smtp_from', '', 'From email address for system emails'),
    ('smtp_tls', 'true', 'Whether to use TLS for SMTP'),
    ('registration_enabled', 'true', 'Whether new user registration is enabled'),
    ('max_api_keys_per_user', '10', 'Maximum API keys per user'),
    ('session_timeout_hours', '24', 'Session timeout in hours'),
    ('password_reset_timeout_hours', '24', 'Password reset token timeout in hours');