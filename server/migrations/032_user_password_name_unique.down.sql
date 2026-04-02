DROP INDEX IF EXISTS user_name_lower_unique;
ALTER TABLE "user" DROP COLUMN IF EXISTS password_hash;
