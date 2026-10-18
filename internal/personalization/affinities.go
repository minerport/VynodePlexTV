package personalization

import (
	"context"
	"time"
)

type Affinity struct {
	Kind     string  `json:"kind"`
	Identity string  `json:"identity"`
	Name     string  `json:"name"`
	Score    float64 `json:"score"`
	Sessions int     `json:"engagedSessions"`
}

func (s *Service) Affinities(ctx context.Context, user string) ([]Affinity, error) {
	out := []Affinity{}
	rows, err := s.db.QueryContext(ctx, `SELECT kind,identity,name,score,engaged_sessions FROM user_content_affinities WHERE user_id=? ORDER BY score DESC,kind,identity LIMIT 200`, user)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var a Affinity
		if err = rows.Scan(&a.Kind, &a.Identity, &a.Name, &a.Score, &a.Sessions); err != nil {
			return out, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Rebuild replaces derived state atomically. No personal facts are shared across users.
func (s *Service) Rebuild(ctx context.Context, user string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM user_program_history WHERE user_id=? AND started_at<?", user, now.AddDate(0, 0, -HistoryRetentionDays).UnixMilli()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM user_content_affinities WHERE user_id=?", user); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `WITH engaged AS (
 SELECT h.*,MIN(1.0,watch_duration_ms/1800000.0)*CASE WHEN started_at>=? THEN 1.0 WHEN started_at>=? THEN 0.5 ELSE 0.15 END weight FROM user_program_history h WHERE user_id=? AND watch_duration_ms>=?
 ), signals AS (
 SELECT h.id,'channel' kind,c.id identity,c.name,h.weight FROM engaged h JOIN channels c ON c.id=h.channel_id
 UNION ALL SELECT h.id,'show',sh.id,sh.title,h.weight FROM engaged h JOIN plex_media_items m ON m.id=h.media_item_id JOIN plex_media_items sh ON sh.plex_server_id=m.plex_server_id AND sh.plex_rating_key=m.grandparent_rating_key
 UNION ALL SELECT DISTINCT h.id,r.relationship,CAST(e.id AS TEXT),e.name,h.weight FROM engaged h JOIN plex_media_items m ON m.id=h.media_item_id LEFT JOIN plex_media_items sh ON sh.plex_server_id=m.plex_server_id AND sh.plex_rating_key=m.grandparent_rating_key JOIN media_item_entities r ON r.media_item_id=m.id OR r.media_item_id=sh.id JOIN metadata_entities e ON e.id=r.entity_id WHERE r.relationship IN ('genre','actor','director')
 ) INSERT INTO user_content_affinities(user_id,kind,identity,name,score,engaged_sessions) SELECT ?,kind,identity,name,MIN(1.0,SUM(weight)/5.0),COUNT(DISTINCT id) FROM signals WHERE (SELECT COUNT(*) FROM engaged)>=? GROUP BY kind,identity,name`, now.AddDate(0, 0, -7).UnixMilli(), now.AddDate(0, 0, -30).UnixMilli(), user, MeaningfulWatchMS, user, MinimumAffinitySessions)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO personalization_state(user_id,ranking_version,rebuilt_at,dirty) VALUES(?,?,?,0) ON CONFLICT(user_id) DO UPDATE SET ranking_version=excluded.ranking_version,rebuilt_at=excluded.rebuilt_at,dirty=0`, user, RankingVersion, now.UnixMilli())
	if err != nil {
		return err
	}
	return tx.Commit()
}
