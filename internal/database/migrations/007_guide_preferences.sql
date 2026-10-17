CREATE TABLE user_channel_favorites (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
 created_at INTEGER NOT NULL,
 PRIMARY KEY(user_id,channel_id)
);
CREATE TABLE viewer_preferences (
 user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 last_channel_id TEXT REFERENCES channels(id) ON DELETE SET NULL,
 updated_at INTEGER NOT NULL
);
ALTER TABLE viewing_sessions ADD COLUMN origin TEXT NOT NULL DEFAULT 'LIVE' CHECK(origin IN ('LIVE','CURRENT_FROM_BEGINNING','CATCH_UP'));
CREATE INDEX programs_window ON channel_programs(start_time_utc,end_time_utc,channel_id);
