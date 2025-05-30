-- Drop tables and functions for user tool credentials

-- Drop triggers first
DROP TRIGGER IF EXISTS update_user_tool_credentials_updated_at_trigger ON user_tool_credentials;

-- Drop functions
DROP FUNCTION IF EXISTS update_user_tool_credentials_updated_at();
DROP FUNCTION IF EXISTS cleanup_expired_credentials();

-- Drop tables (credential_access_log first due to foreign key)
DROP TABLE IF EXISTS credential_access_log;
DROP TABLE IF EXISTS user_tool_credentials;