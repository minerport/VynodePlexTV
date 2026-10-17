CREATE TABLE plex_servers (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, base_url TEXT NOT NULL,
 machine_identifier TEXT NOT NULL UNIQUE, product TEXT NOT NULL DEFAULT '', version TEXT NOT NULL DEFAULT '',
 platform TEXT NOT NULL DEFAULT '', platform_version TEXT NOT NULL DEFAULT '', friendly_name TEXT NOT NULL DEFAULT '',
 owned INTEGER, secure_connections INTEGER, token_encrypted BLOB NOT NULL, token_key_version INTEGER NOT NULL,
 last_health_check_at INTEGER, last_success_at INTEGER, last_error TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'CONNECTED', disabled INTEGER NOT NULL DEFAULT 0,
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE TABLE plex_libraries (
 id TEXT PRIMARY KEY, plex_server_id TEXT NOT NULL REFERENCES plex_servers(id),
 plex_section_key TEXT NOT NULL, title TEXT NOT NULL, type TEXT NOT NULL,
 agent TEXT NOT NULL DEFAULT '', scanner TEXT NOT NULL DEFAULT '', language TEXT NOT NULL DEFAULT '', uuid TEXT NOT NULL DEFAULT '',
 selected INTEGER NOT NULL DEFAULT 0, present INTEGER NOT NULL DEFAULT 1,
 last_synced_at INTEGER, last_full_sync_at INTEGER, source_revision TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
 UNIQUE(plex_server_id,plex_section_key)
);
CREATE TABLE plex_media_items (
 id TEXT PRIMARY KEY, plex_server_id TEXT NOT NULL REFERENCES plex_servers(id), plex_library_id TEXT NOT NULL REFERENCES plex_libraries(id),
 plex_rating_key TEXT NOT NULL, parent_rating_key TEXT NOT NULL DEFAULT '', grandparent_rating_key TEXT NOT NULL DEFAULT '',
 media_type TEXT NOT NULL, title TEXT NOT NULL, sort_title TEXT NOT NULL DEFAULT '', original_title TEXT NOT NULL DEFAULT '',
 summary TEXT NOT NULL DEFAULT '', year INTEGER, content_rating TEXT NOT NULL DEFAULT '', rating REAL, audience_rating REAL,
 duration_ms INTEGER, added_at INTEGER, updated_at_source INTEGER, originally_available_at TEXT NOT NULL DEFAULT '',
 studio TEXT NOT NULL DEFAULT '', tagline TEXT NOT NULL DEFAULT '', index_number INTEGER, parent_index_number INTEGER,
 guid TEXT NOT NULL DEFAULT '', thumb_path TEXT NOT NULL DEFAULT '', art_path TEXT NOT NULL DEFAULT '',
 parent_thumb_path TEXT NOT NULL DEFAULT '', grandparent_thumb_path TEXT NOT NULL DEFAULT '',
 available INTEGER NOT NULL DEFAULT 1, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
 UNIQUE(plex_server_id,plex_rating_key)
);
CREATE INDEX media_library ON plex_media_items(plex_library_id,available,media_type);
CREATE INDEX media_type ON plex_media_items(media_type);
CREATE INDEX media_title ON plex_media_items(title COLLATE NOCASE);
CREATE INDEX media_sort ON plex_media_items(sort_title);
CREATE INDEX media_year ON plex_media_items(year);
CREATE INDEX media_added ON plex_media_items(added_at);
CREATE INDEX media_available ON plex_media_items(available);
CREATE INDEX media_parent ON plex_media_items(plex_server_id,parent_rating_key,index_number);
CREATE INDEX media_show ON plex_media_items(plex_server_id,grandparent_rating_key,parent_index_number,index_number);
CREATE TABLE metadata_entities (
 id INTEGER PRIMARY KEY, plex_server_id TEXT NOT NULL REFERENCES plex_servers(id),
 kind TEXT NOT NULL CHECK(kind IN ('genre','person','collection','label')),
 source_key TEXT NOT NULL, name TEXT NOT NULL,
 UNIQUE(plex_server_id,kind,source_key)
);
CREATE TABLE media_item_entities (
 media_item_id TEXT NOT NULL REFERENCES plex_media_items(id), entity_id INTEGER NOT NULL REFERENCES metadata_entities(id),
 relationship TEXT NOT NULL, character_name TEXT NOT NULL DEFAULT '',
 PRIMARY KEY(media_item_id,entity_id,relationship)
);
CREATE INDEX entity_media ON media_item_entities(entity_id,relationship,media_item_id);
CREATE TABLE media_external_ids (
 media_item_id TEXT NOT NULL REFERENCES plex_media_items(id), provider TEXT NOT NULL, external_id TEXT NOT NULL,
 PRIMARY KEY(media_item_id,provider,external_id)
);
CREATE INDEX external_id_media ON media_external_ids(provider,external_id);
CREATE TABLE plex_media_versions (
 id TEXT PRIMARY KEY, media_item_id TEXT NOT NULL REFERENCES plex_media_items(id), plex_media_id TEXT NOT NULL, plex_part_id TEXT NOT NULL,
 container TEXT NOT NULL DEFAULT '', video_codec TEXT NOT NULL DEFAULT '', audio_codec TEXT NOT NULL DEFAULT '',
 video_resolution TEXT NOT NULL DEFAULT '', width INTEGER, height INTEGER, bitrate INTEGER, duration_ms INTEGER,
 file_path TEXT NOT NULL DEFAULT '', size INTEGER, optimized INTEGER,
 UNIQUE(media_item_id,plex_media_id,plex_part_id)
);
CREATE TABLE plex_media_streams (
 id INTEGER PRIMARY KEY, version_id TEXT NOT NULL REFERENCES plex_media_versions(id) ON DELETE CASCADE,
 plex_stream_id TEXT NOT NULL, stream_type INTEGER, codec TEXT NOT NULL DEFAULT '', language TEXT NOT NULL DEFAULT '',
 language_code TEXT NOT NULL DEFAULT '', title TEXT NOT NULL DEFAULT '', is_default INTEGER, forced INTEGER,
 hearing_impaired INTEGER, channels INTEGER, width INTEGER, height INTEGER, profile TEXT NOT NULL DEFAULT ''
);
CREATE INDEX streams_version ON plex_media_streams(version_id);
CREATE TABLE jobs (
 id TEXT PRIMARY KEY, type TEXT NOT NULL, server_id TEXT REFERENCES plex_servers(id), library_id TEXT REFERENCES plex_libraries(id),
 mode TEXT NOT NULL DEFAULT 'incremental', status TEXT NOT NULL CHECK(status IN ('QUEUED','RUNNING','SUCCEEDED','FAILED','CANCELLED')),
 created_at INTEGER NOT NULL, started_at INTEGER, completed_at INTEGER,
 processed INTEGER NOT NULL DEFAULT 0, total INTEGER, current_item TEXT NOT NULL DEFAULT '',
 error TEXT NOT NULL DEFAULT '', cancel_requested INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX jobs_queue ON jobs(status,created_at);
CREATE UNIQUE INDEX one_library_job ON jobs(library_id) WHERE type='PLEX_LIBRARY_SYNC' AND status IN ('QUEUED','RUNNING');
CREATE UNIQUE INDEX one_server_job ON jobs(server_id,type) WHERE library_id IS NULL AND status IN ('QUEUED','RUNNING');
CREATE UNIQUE INDEX one_maintenance_job ON jobs(type) WHERE type='PLEX_ARTWORK_MAINTENANCE' AND status IN ('QUEUED','RUNNING');
CREATE TABLE sync_seen (
 job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE, media_item_id TEXT NOT NULL REFERENCES plex_media_items(id),
 PRIMARY KEY(job_id,media_item_id)
);
CREATE TABLE plex_settings (
 id INTEGER PRIMARY KEY CHECK(id=1), health_interval_seconds INTEGER NOT NULL DEFAULT 300,
 sync_interval_seconds INTEGER NOT NULL DEFAULT 21600, full_sync_interval_seconds INTEGER NOT NULL DEFAULT 86400,
 artwork_cache_mb INTEGER NOT NULL DEFAULT 512
);
INSERT INTO plex_settings(id) VALUES(1);
CREATE TABLE artwork_cache (
 cache_key TEXT PRIMARY KEY, content_type TEXT NOT NULL, etag TEXT NOT NULL DEFAULT '', last_modified TEXT NOT NULL DEFAULT '',
 cached_at INTEGER NOT NULL, last_access_at INTEGER NOT NULL, size INTEGER NOT NULL
);
