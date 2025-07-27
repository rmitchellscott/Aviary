-- Add CASCADE to foreign key constraints

-- Update backup_jobs constraint
ALTER TABLE backup_jobs DROP CONSTRAINT IF EXISTS fk_backup_jobs_admin_user;
ALTER TABLE backup_jobs ADD CONSTRAINT fk_backup_jobs_admin_user 
    FOREIGN KEY (admin_user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Update restore_uploads constraint (if table exists)
ALTER TABLE restore_uploads DROP CONSTRAINT IF EXISTS fk_restore_uploads_admin_user;
ALTER TABLE restore_uploads ADD CONSTRAINT fk_restore_uploads_admin_user 
    FOREIGN KEY (admin_user_id) REFERENCES users(id) ON DELETE CASCADE;
