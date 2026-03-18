-- Relay database initialisation script
-- Creates one database per service to support logical isolation.
-- Runs automatically via the Docker Compose postgres container on first start.

\c postgres

CREATE DATABASE relay_auth;
CREATE DATABASE relay_users;
CREATE DATABASE relay_workspaces;
CREATE DATABASE relay_channels;
CREATE DATABASE relay_messages;
CREATE DATABASE relay_files;
CREATE DATABASE relay_notifications;
CREATE DATABASE relay_search;

-- Grant the relay user full access to all databases
GRANT ALL PRIVILEGES ON DATABASE relay_auth        TO relay;
GRANT ALL PRIVILEGES ON DATABASE relay_users       TO relay;
GRANT ALL PRIVILEGES ON DATABASE relay_workspaces  TO relay;
GRANT ALL PRIVILEGES ON DATABASE relay_channels    TO relay;
GRANT ALL PRIVILEGES ON DATABASE relay_messages    TO relay;
GRANT ALL PRIVILEGES ON DATABASE relay_files       TO relay;
GRANT ALL PRIVILEGES ON DATABASE relay_notifications TO relay;
GRANT ALL PRIVILEGES ON DATABASE relay_search      TO relay;
