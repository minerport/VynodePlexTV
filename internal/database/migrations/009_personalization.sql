-- Personal browsing never changes a shared channel or its broadcast timeline.
CREATE TABLE user_channel_preferences (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
 position INTEGER CHECK(position IS NULL OR position >= 0),
 hidden INTEGER NOT NULL DEFAULT 0 CHECK(hidden IN (0,1)),
 PRIMARY KEY(user_id,channel_id)
);
CREATE INDEX personal_channel_order ON user_channel_preferences(user_id,hidden,position);
ALTER TABLE viewer_preferences ADD COLUMN order_mode TEXT NOT NULL DEFAULT 'NUMBER' CHECK(order_mode IN ('NUMBER','PERSONAL'));

CREATE TABLE user_program_history (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 channel_id TEXT REFERENCES channels(id) ON DELETE SET NULL,
 program_id TEXT REFERENCES channel_programs(id) ON DELETE SET NULL,
 media_item_id TEXT NOT NULL REFERENCES plex_media_items(id),
 started_at INTEGER NOT NULL,
 ended_at INTEGER,
 watch_duration_ms INTEGER NOT NULL DEFAULT 0 CHECK(watch_duration_ms>=0),
 program_duration_ms INTEGER NOT NULL CHECK(program_duration_ms>0),
 completion_ratio REAL NOT NULL DEFAULT 0 CHECK(completion_ratio BETWEEN 0 AND 1),
 playback_origin TEXT NOT NULL,
 client_type TEXT NOT NULL,
 last_sample_at INTEGER NOT NULL,
 last_position_ms INTEGER NOT NULL,
 last_state TEXT NOT NULL
);
CREATE INDEX personal_history_time ON user_program_history(user_id,started_at);
CREATE INDEX personal_history_media ON user_program_history(user_id,media_item_id,ended_at);
CREATE INDEX personal_history_channel ON user_program_history(user_id,channel_id,started_at);
CREATE TABLE user_content_affinities (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('genre','show','actor','director','channel')),
 identity TEXT NOT NULL,
 name TEXT NOT NULL,
 score REAL NOT NULL,
 engaged_sessions INTEGER NOT NULL,
 PRIMARY KEY(user_id,kind,identity)
);
CREATE TABLE personalization_state (
 user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 ranking_version INTEGER NOT NULL DEFAULT 1,
 rebuilt_at INTEGER NOT NULL DEFAULT 0,
 dirty INTEGER NOT NULL DEFAULT 1 CHECK(dirty IN (0,1))
);
CREATE TABLE channel_activation_windows (
 channel_id TEXT PRIMARY KEY REFERENCES channels(id) ON DELETE CASCADE,
 start_date TEXT NOT NULL,
 end_date TEXT NOT NULL,
 timezone TEXT NOT NULL,
 start_ms INTEGER NOT NULL,
 end_ms INTEGER NOT NULL CHECK(end_ms>start_ms)
);
