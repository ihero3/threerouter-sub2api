-- Migration: Create default consent records for existing users
-- This migration ensures all existing users have default consent records
-- Run this before deploying the new version that enforces consent checks

-- Insert default consent records for all existing users
-- Only inserts if the user doesn't already have a record for that consent type

INSERT INTO user_consents (user_id, consent_type, granted, granted_at, source, created_at, updated_at)
SELECT 
    u.id,
    ct.consent_type,
    true,
    NOW(),
    'migration_default',
    NOW(),
    NOW()
FROM users u
CROSS JOIN (
    VALUES 
        ('terms_of_service'),
        ('gdpr_data_processing'),
        ('detailed_logging'),
        ('cross_border_transfer'),
        ('marketing'),
        ('model_training')
) AS ct(consent_type)
WHERE NOT EXISTS (
    SELECT 1 
    FROM user_consents uc 
    WHERE uc.user_id = u.id 
    AND uc.consent_type = ct.consent_type
);
