-- Rollback 000001_init
DROP TABLE IF EXISTS workspace_domain_policies;
DROP TYPE  IF EXISTS domain_classification;
DROP TABLE IF EXISTS reactions;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS channel_members;
DROP TABLE IF EXISTS guest_channel_access;
DROP TABLE IF EXISTS channels;
DROP TABLE IF EXISTS workspace_members;
DROP TYPE  IF EXISTS member_role;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS users;
DROP FUNCTION IF EXISTS next_snowflake();
DROP SEQUENCE IF EXISTS snowflake_seq;
