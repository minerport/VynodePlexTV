-- Upgrade only the original six-hour default; preserve custom refresh intervals.
UPDATE plex_settings SET sync_interval_seconds=900 WHERE sync_interval_seconds=21600;
