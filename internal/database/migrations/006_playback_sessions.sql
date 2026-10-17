CREATE TABLE viewing_sessions (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
 program_id TEXT REFERENCES channel_programs(id) ON DELETE SET NULL,
 media_item_id TEXT NOT NULL REFERENCES plex_media_items(id),
 plex_server_id TEXT NOT NULL REFERENCES plex_servers(id),
 auth_session_hash TEXT NOT NULL,
 started_at INTEGER NOT NULL,
 last_seen_at INTEGER NOT NULL,
 playback_mode TEXT NOT NULL CHECK(playback_mode IN ('LIVE','BEHIND_LIVE','FROM_BEGINNING')),
 scheduled_live_offset_ms INTEGER NOT NULL CHECK(scheduled_live_offset_ms>=0),
 current_position_ms INTEGER NOT NULL CHECK(current_position_ms>=0),
 state TEXT NOT NULL CHECK(state IN ('STARTING','PLAYING','PAUSED','STOPPED','ERROR')),
 client_identifier TEXT NOT NULL,
 client_type TEXT NOT NULL,
 delivery_mode TEXT NOT NULL,
 error_summary TEXT NOT NULL DEFAULT ''
);
CREATE INDEX viewing_sessions_active ON viewing_sessions(state,last_seen_at);
CREATE INDEX viewing_sessions_user_client ON viewing_sessions(user_id,client_identifier);
