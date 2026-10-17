ALTER TABLE channel_schedule_settings ADD COLUMN recency_weighted INTEGER NOT NULL DEFAULT 0 CHECK(recency_weighted IN(0,1));
ALTER TABLE channel_schedule_settings ADD COLUMN avoid_same_show INTEGER NOT NULL DEFAULT 0 CHECK(avoid_same_show IN(0,1));
ALTER TABLE channel_schedule_settings ADD COLUMN min_other_shows INTEGER NOT NULL DEFAULT 0 CHECK(min_other_shows BETWEEN 0 AND 10);
