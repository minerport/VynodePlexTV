CREATE TABLE channel_schedule_settings (
 channel_id TEXT PRIMARY KEY REFERENCES channels(id) ON DELETE CASCADE,
 mode TEXT NOT NULL CHECK(mode IN('SEQUENTIAL','RANDOM','SMART_RANDOM')),
 timezone TEXT NOT NULL, anchor_ms INTEGER NOT NULL, seed TEXT NOT NULL,
 movie_repeat_days INTEGER NOT NULL DEFAULT 14 CHECK(movie_repeat_days BETWEEN 0 AND 365),
 episode_repeat_days INTEGER NOT NULL DEFAULT 30 CHECK(episode_repeat_days BETWEEN 0 AND 365),
 past_days INTEGER NOT NULL DEFAULT 7 CHECK(past_days BETWEEN 0 AND 30),
 future_days INTEGER NOT NULL DEFAULT 14 CHECK(future_days BETWEEN 1 AND 30),
 include_unavailable INTEGER NOT NULL DEFAULT 0, enabled INTEGER NOT NULL DEFAULT 1,
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
INSERT INTO channel_schedule_settings(channel_id,mode,timezone,anchor_ms,seed,created_at,updated_at)
 SELECT id,CASE WHEN channel_type IN('STATIC','SHOW') THEN 'SEQUENTIAL' ELSE 'SMART_RANDOM' END,
 (SELECT timezone FROM app_settings WHERE id=1),(created_at-604800)*1000,lower(hex(randomblob(16))),unixepoch(),unixepoch() FROM channels;
CREATE TABLE channel_programs (
 id TEXT PRIMARY KEY, channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
 media_item_id TEXT NOT NULL REFERENCES plex_media_items(id) ON DELETE RESTRICT,
 source_membership_identity TEXT NOT NULL,
 start_time_utc INTEGER NOT NULL, end_time_utc INTEGER NOT NULL, duration_ms INTEGER NOT NULL CHECK(duration_ms>0),
 sequence_number INTEGER NOT NULL CHECK(sequence_number>=0), generation_id TEXT NOT NULL,
 schedule_revision INTEGER NOT NULL, policy_reason TEXT NOT NULL, created_at INTEGER NOT NULL,
 CHECK(end_time_utc=start_time_utc+duration_ms), UNIQUE(channel_id,start_time_utc), UNIQUE(channel_id,sequence_number)
);
CREATE INDEX program_end ON channel_programs(channel_id,end_time_utc);
CREATE INDEX program_history ON channel_programs(channel_id,media_item_id,start_time_utc);
CREATE TABLE scheduler_state (
 channel_id TEXT PRIMARY KEY REFERENCES channels(id) ON DELETE CASCADE,
 fingerprint TEXT NOT NULL DEFAULT '', config_fingerprint TEXT NOT NULL DEFAULT '', membership_json TEXT NOT NULL DEFAULT '[]',
 revision INTEGER NOT NULL DEFAULT 0, next_sequence INTEGER NOT NULL DEFAULT 0, covered_until INTEGER,
 status TEXT NOT NULL DEFAULT 'NEEDS_GENERATION', last_generated INTEGER, last_error TEXT NOT NULL DEFAULT ''
);
CREATE TABLE channel_schedule_history (
 channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
 media_item_id TEXT NOT NULL REFERENCES plex_media_items(id), last_start_ms INTEGER NOT NULL,
 PRIMARY KEY(channel_id,media_item_id)
);
ALTER TABLE jobs ADD COLUMN channel_id TEXT REFERENCES channels(id) ON DELETE CASCADE;
ALTER TABLE jobs ADD COLUMN actor_id TEXT REFERENCES users(id);
ALTER TABLE jobs ADD COLUMN repaired INTEGER NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN affected_start INTEGER;
ALTER TABLE jobs ADD COLUMN affected_end INTEGER;
CREATE UNIQUE INDEX one_channel_schedule_job ON jobs(channel_id) WHERE type LIKE 'SCHEDULE_%' AND status IN('QUEUED','RUNNING');
