CREATE TABLE channel_groups (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, slug TEXT NOT NULL UNIQUE COLLATE NOCASE,
 description TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL DEFAULT 0,
 system INTEGER NOT NULL DEFAULT 0 CHECK(system IN(0,1)), created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE TABLE channel_uploads (
 id TEXT PRIMARY KEY, content_type TEXT NOT NULL, size INTEGER NOT NULL, width INTEGER NOT NULL, height INTEGER NOT NULL,
 created_by TEXT REFERENCES users(id), created_at INTEGER NOT NULL
);
CREATE TABLE channels (
 id TEXT PRIMARY KEY, channel_number TEXT NOT NULL UNIQUE COLLATE NOCASE, name TEXT NOT NULL, short_name TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '',
 channel_type TEXT NOT NULL CHECK(channel_type IN('STATIC','SHOW','COLLECTION','RULE')),
 group_id TEXT REFERENCES channel_groups(id) ON DELETE RESTRICT,
 enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN(0,1)), visibility TEXT NOT NULL DEFAULT 'VISIBLE' CHECK(visibility IN('VISIBLE','HIDDEN')),
 sort_order INTEGER NOT NULL DEFAULT 0,
 logo_source TEXT NOT NULL DEFAULT 'AUTO' CHECK(logo_source IN('AUTO','PLEX','UPLOAD')),
 logo_upload_id TEXT REFERENCES channel_uploads(id),
 background_source TEXT NOT NULL DEFAULT 'AUTO' CHECK(background_source IN('AUTO','PLEX','UPLOAD')),
 background_upload_id TEXT REFERENCES channel_uploads(id),
 default_artwork_media_id TEXT REFERENCES plex_media_items(id),
 created_by TEXT REFERENCES users(id), created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE INDEX channels_group ON channels(group_id,sort_order);
CREATE TABLE channel_sources (
 channel_id TEXT PRIMARY KEY REFERENCES channels(id) ON DELETE CASCADE,
 source_type TEXT NOT NULL CHECK(source_type IN('STATIC','SHOW','COLLECTION','RULE')),
 config_json TEXT NOT NULL CHECK(json_valid(config_json)), created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE TABLE channel_static_items (
 channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
 media_item_id TEXT NOT NULL REFERENCES plex_media_items(id) ON DELETE RESTRICT,
 position INTEGER NOT NULL CHECK(position>=0), created_at INTEGER NOT NULL,
 PRIMARY KEY(channel_id,media_item_id), UNIQUE(channel_id,position)
);
ALTER TABLE audit_log ADD COLUMN target_id TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_log ADD COLUMN summary TEXT NOT NULL DEFAULT '';
CREATE INDEX media_studio ON plex_media_items(studio);
CREATE INDEX media_rating ON plex_media_items(rating);
CREATE INDEX media_content_rating ON plex_media_items(content_rating);
CREATE INDEX metadata_name ON metadata_entities(kind,name COLLATE NOCASE);
INSERT INTO channel_groups(id,name,slug,sort_order,system,created_at,updated_at) VALUES
 ('movies','Movies','movies',10,1,unixepoch(),unixepoch()),('tv','TV','tv',20,1,unixepoch(),unixepoch()),('kids','Kids','kids',30,1,unixepoch(),unixepoch());
