CREATE TABLE interstitial_assets (
 id TEXT PRIMARY KEY, name TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN('BUMPER','IDENT','TRAILER','COMING_UP_NEXT','CUSTOM_CLIP','FILLER')),
 source TEXT NOT NULL CHECK(source IN('UPLOAD','PLEX_MEDIA','PLEX_EXTRA','GENERATED_METADATA')),
 media_id TEXT REFERENCES plex_media_items(id), duration_ms INTEGER NOT NULL CHECK(duration_ms BETWEEN 1 AND 600000),
 enabled INTEGER NOT NULL DEFAULT 1, available INTEGER NOT NULL DEFAULT 1,
 container TEXT NOT NULL DEFAULT '', video_codec TEXT NOT NULL DEFAULT '', audio_codec TEXT NOT NULL DEFAULT '',
 width INTEGER NOT NULL DEFAULT 0,height INTEGER NOT NULL DEFAULT 0,size_bytes INTEGER NOT NULL DEFAULT 0,
 active_from TEXT NOT NULL DEFAULT '',active_until TEXT NOT NULL DEFAULT '',
 created_by TEXT REFERENCES users(id),created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL,
 CHECK((source IN('PLEX_MEDIA','PLEX_EXTRA') AND media_id IS NOT NULL) OR (source IN('UPLOAD','GENERATED_METADATA') AND media_id IS NULL))
);
CREATE TABLE interstitial_tags(asset_id TEXT NOT NULL REFERENCES interstitial_assets(id) ON DELETE CASCADE,tag TEXT NOT NULL,PRIMARY KEY(asset_id,tag));
CREATE INDEX interstitial_tags_reverse ON interstitial_tags(tag,asset_id);
CREATE TABLE interstitial_rules (
 id TEXT PRIMARY KEY,name TEXT NOT NULL,channel_id TEXT REFERENCES channels(id) ON DELETE CASCADE,
 enabled INTEGER NOT NULL DEFAULT 1,kind TEXT NOT NULL,tag TEXT NOT NULL DEFAULT '',
 placement TEXT NOT NULL CHECK(placement IN('BEFORE','BETWEEN','HOURLY')),
 media_type TEXT NOT NULL DEFAULT '' CHECK(media_type IN('','movie','episode')),
 every_n INTEGER NOT NULL DEFAULT 1 CHECK(every_n BETWEEN 1 AND 1000),
 repeat_minutes INTEGER NOT NULL DEFAULT 120 CHECK(repeat_minutes BETWEEN 0 AND 10080),
 max_duration_ms INTEGER NOT NULL DEFAULT 600000 CHECK(max_duration_ms BETWEEN 1 AND 600000),
 priority INTEGER NOT NULL DEFAULT 0,created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL
);
CREATE TABLE channel_presentation (
 channel_id TEXT PRIMARY KEY REFERENCES channels(id) ON DELETE CASCADE,
 enabled INTEGER NOT NULL DEFAULT 0,inherit_defaults INTEGER NOT NULL DEFAULT 1,
 bug_enabled INTEGER NOT NULL DEFAULT 0,bug_corner TEXT NOT NULL DEFAULT 'top-right' CHECK(bug_corner IN('top-left','top-right','bottom-left','bottom-right')),
 bug_opacity REAL NOT NULL DEFAULT 0.35 CHECK(bug_opacity BETWEEN 0.1 AND 0.7),bug_size INTEGER NOT NULL DEFAULT 72 CHECK(bug_size BETWEEN 32 AND 128),
 compact_logo_id TEXT REFERENCES channel_uploads(id),next_up_seconds INTEGER NOT NULL DEFAULT 45 CHECK(next_up_seconds BETWEEN 0 AND 120)
);
CREATE TABLE channel_interstitial_rules (
 channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,rule_id TEXT NOT NULL REFERENCES interstitial_rules(id) ON DELETE CASCADE,
 enabled INTEGER NOT NULL,PRIMARY KEY(channel_id,rule_id)
);
CREATE TABLE presentation_entries (
 id TEXT PRIMARY KEY,channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
 following_program_id TEXT NOT NULL REFERENCES channel_programs(id) ON DELETE CASCADE,
 asset_id TEXT NOT NULL REFERENCES interstitial_assets(id) ON DELETE RESTRICT,
 rule_id TEXT, start_time_utc INTEGER NOT NULL,end_time_utc INTEGER NOT NULL,duration_ms INTEGER NOT NULL CHECK(duration_ms>0),
 position INTEGER NOT NULL,reason TEXT NOT NULL, CHECK(end_time_utc=start_time_utc+duration_ms),UNIQUE(channel_id,start_time_utc)
);
CREATE INDEX presentation_end ON presentation_entries(channel_id,end_time_utc);
CREATE INDEX presentation_following ON presentation_entries(following_program_id);
CREATE TABLE presentation_history(channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,asset_id TEXT NOT NULL REFERENCES interstitial_assets(id),last_start_ms INTEGER NOT NULL,PRIMARY KEY(channel_id,asset_id));
ALTER TABLE viewing_sessions ADD COLUMN timeline_entry_id TEXT NOT NULL DEFAULT '';
CREATE VIEW broadcast_timeline AS
 SELECT id,channel_id,id AS following_program_id,media_item_id,'' AS asset_id,'PROGRAM' AS entry_type,start_time_utc,end_time_utc,duration_ms,sequence_number,policy_reason FROM channel_programs
 UNION ALL
 SELECT x.id,x.channel_id,x.following_program_id,p.media_item_id,x.asset_id,'INTERSTITIAL',x.start_time_utc,x.end_time_utc,x.duration_ms,p.sequence_number,x.reason FROM presentation_entries x JOIN channel_programs p ON p.id=x.following_program_id;
ALTER TABLE viewing_sessions ADD COLUMN stream_session_id TEXT NOT NULL DEFAULT '';
ALTER TABLE interstitial_assets ADD COLUMN plex_extra_key TEXT NOT NULL DEFAULT '';
