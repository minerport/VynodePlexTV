CREATE TABLE users (
 id TEXT PRIMARY KEY,
 username TEXT NOT NULL COLLATE NOCASE UNIQUE,
 password_hash TEXT NOT NULL,
 role TEXT NOT NULL CHECK(role IN ('OWNER','ADMIN','USER')),
 created_at INTEGER NOT NULL
);
CREATE TABLE user_preferences (
 user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 preferences_json TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE sessions (
 token_hash TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 csrf_token TEXT NOT NULL,
 expires_at INTEGER NOT NULL
);
CREATE INDEX sessions_expiry ON sessions(expires_at);
CREATE TABLE app_settings (
 id INTEGER PRIMARY KEY CHECK(id=1),
 display_name TEXT NOT NULL DEFAULT 'Afterglow TV',
 timezone TEXT NOT NULL DEFAULT 'UTC'
);
INSERT INTO app_settings(id) VALUES(1);
CREATE TABLE audit_log (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 user_id TEXT REFERENCES users(id),
 action TEXT NOT NULL,
 created_at INTEGER NOT NULL
);
