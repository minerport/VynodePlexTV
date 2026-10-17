package plex

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const itemColumns = `m.id,m.plex_server_id,m.plex_library_id,l.title,m.plex_rating_key,m.parent_rating_key,m.grandparent_rating_key,m.media_type,m.title,m.sort_title,m.original_title,m.summary,m.year,m.content_rating,m.rating,m.audience_rating,m.duration_ms,m.added_at,m.updated_at_source,m.originally_available_at,m.studio,m.tagline,m.index_number,m.parent_index_number,m.guid,m.thumb_path,m.art_path,m.parent_thumb_path,m.grandparent_thumb_path,m.available,m.updated_at`

func scanItem(row scanner) (MediaItem, error) {
	var m MediaItem
	err := row.Scan(&m.ID, &m.ServerID, &m.LibraryID, &m.LibraryTitle, &m.RatingKey, &m.ParentKey, &m.GrandparentKey, &m.Type, &m.Title, &m.SortTitle, &m.OriginalTitle, &m.Summary, &m.Year, &m.ContentRating, &m.Rating, &m.AudienceRating, &m.DurationMS, &m.AddedAt, &m.SourceUpdatedAt, &m.OriginallyAvailableAt, &m.Studio, &m.Tagline, &m.Index, &m.ParentIndex, &m.GUID, &m.Thumb, &m.Art, &m.ParentThumb, &m.GrandparentThumb, &m.Available, &m.UpdatedAt)
	return m, err
}

type MediaQuery struct {
	ServerID, LibraryID, Type, Title, Availability, ParentKey string
	Offset, Limit                                             int
	TopLevel                                                  bool
}
type MediaResult struct {
	Items  []MediaItem `json:"items"`
	Total  int         `json:"total"`
	Offset int         `json:"offset"`
	Limit  int         `json:"limit"`
}

func (s *Service) Media(ctx context.Context, q MediaQuery) (MediaResult, error) {
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 40
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	where := []string{"1=1"}
	if q.TopLevel {
		where = append(where, "m.media_type IN ('movie','show')")
	}
	args := []any{}
	for _, f := range []struct{ column, value string }{{"m.plex_server_id", q.ServerID}, {"m.plex_library_id", q.LibraryID}, {"m.media_type", q.Type}, {"m.parent_rating_key", q.ParentKey}} {
		if f.value != "" {
			where = append(where, f.column+"=?")
			args = append(args, f.value)
		}
	}
	if q.Availability == "true" || q.Availability == "false" {
		where = append(where, "m.available=?")
		args = append(args, q.Availability == "true")
	}
	if q.Title != "" {
		title := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(q.Title) + "%"
		where = append(where, `(m.title LIKE ? ESCAPE '\' OR EXISTS (SELECT 1 FROM plex_media_items p WHERE p.plex_server_id=m.plex_server_id AND p.plex_rating_key IN (m.parent_rating_key,m.grandparent_rating_key) AND p.title LIKE ? ESCAPE '\'))`)
		args = append(args, title, title)
	}
	clause := strings.Join(where, " AND ")
	out := MediaResult{Items: []MediaItem{}, Offset: q.Offset, Limit: q.Limit}
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM plex_media_items m WHERE "+clause, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	args = append(args, q.Limit, q.Offset)
	order := "m.title COLLATE NOCASE,m.id"
	if q.ParentKey != "" {
		order = "COALESCE(m.index_number,2147483647)," + order
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+itemColumns+" FROM plex_media_items m JOIN plex_libraries l ON l.id=m.plex_library_id WHERE "+clause+" ORDER BY "+order+" LIMIT ? OFFSET ?", args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		m, e := scanItem(rows)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, m)
	}
	return out, rows.Err()
}
func (s *Service) Item(ctx context.Context, id string) (MediaItem, error) {
	m, err := scanItem(s.db.QueryRowContext(ctx, "SELECT "+itemColumns+" FROM plex_media_items m JOIN plex_libraries l ON l.id=m.plex_library_id WHERE m.id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return m, failure("NOT_FOUND", "Media item not found")
	}
	if err != nil {
		return m, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT e.kind,e.source_key,e.name,r.relationship,r.character_name FROM metadata_entities e JOIN media_item_entities r ON r.entity_id=e.id WHERE r.media_item_id=? ORDER BY r.relationship,e.name`, id)
	if err != nil {
		return m, err
	}
	for rows.Next() {
		var t Tag
		if err = rows.Scan(&t.Kind, &t.SourceKey, &t.Name, &t.Relationship, &t.Character); err != nil {
			rows.Close()
			return m, err
		}
		m.Tags = append(m.Tags, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return m, err
	}
	rows, err = s.db.QueryContext(ctx, "SELECT provider,external_id FROM media_external_ids WHERE media_item_id=?", id)
	if err != nil {
		return m, err
	}
	for rows.Next() {
		var x ExternalID
		if err = rows.Scan(&x.Provider, &x.Value); err != nil {
			rows.Close()
			return m, err
		}
		m.ExternalIDs = append(m.ExternalIDs, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return m, err
	}
	rows, err = s.db.QueryContext(ctx, "SELECT id,plex_media_id,plex_part_id,container,video_codec,audio_codec,video_resolution,width,height,bitrate,duration_ms,size,optimized FROM plex_media_versions WHERE media_item_id=?", id)
	if err != nil {
		return m, err
	}
	versionIDs := []string{}
	for rows.Next() {
		var v MediaVersion
		var vid string
		if err = rows.Scan(&vid, &v.MediaID, &v.PartID, &v.Container, &v.VideoCodec, &v.AudioCodec, &v.Resolution, &v.Width, &v.Height, &v.Bitrate, &v.DurationMS, &v.Size, &v.Optimized); err != nil {
			rows.Close()
			return m, err
		}
		versionIDs = append(versionIDs, vid)
		m.Versions = append(m.Versions, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return m, err
	}
	for i, vid := range versionIDs {
		rows, err = s.db.QueryContext(ctx, "SELECT plex_stream_id,stream_type,codec,language,language_code,title,is_default,forced,hearing_impaired,channels,width,height,profile FROM plex_media_streams WHERE version_id=?", vid)
		if err != nil {
			return m, err
		}
		for rows.Next() {
			var v Stream
			if err = rows.Scan(&v.ID, &v.Type, &v.Codec, &v.Language, &v.LanguageCode, &v.Title, &v.Default, &v.Forced, &v.HearingImpaired, &v.Channels, &v.Width, &v.Height, &v.Profile); err != nil {
				rows.Close()
				return m, err
			}
			m.Versions[i].Streams = append(m.Versions[i].Streams, v)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return m, err
		}
	}
	return m, nil
}

// upsertBatch commits a page at a time. Existing item IDs survive all metadata refreshes.
func (s *Service) upsertBatch(ctx context.Context, jobID string, l Library, items []MediaItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, m := range items {
		if m.RatingKey == "" || m.Title == "" {
			return failure("RESPONSE", "Plex returned an item without its identity or title")
		}
		var id, otherLibrary string
		err = tx.QueryRowContext(ctx, "SELECT id,plex_library_id FROM plex_media_items WHERE plex_server_id=? AND plex_rating_key=?", l.ServerID, m.RatingKey).Scan(&id, &otherLibrary)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if id == "" {
			id = newID()
		}
		if otherLibrary != "" && otherLibrary != l.ID {
			return failure("RESPONSE", "Plex returned an item belonging to another library; reconciliation stopped")
		}
		// A duplicate observation indicates shifting/ignored pagination, never a deletion.
		if _, err = tx.ExecContext(ctx, "INSERT INTO sync_seen(job_id,media_item_id) SELECT ?,id FROM plex_media_items WHERE id=?", jobID, id); err != nil {
			return failure("RESPONSE", "Plex pagination repeated an item; inventory reconciliation was skipped")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO plex_media_items(id,plex_server_id,plex_library_id,plex_rating_key,parent_rating_key,grandparent_rating_key,media_type,title,sort_title,original_title,summary,year,content_rating,rating,audience_rating,duration_ms,added_at,updated_at_source,originally_available_at,studio,tagline,index_number,parent_index_number,guid,thumb_path,art_path,parent_thumb_path,grandparent_thumb_path,available,created_at,updated_at)
  VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,?,?) ON CONFLICT(plex_server_id,plex_rating_key) DO UPDATE SET parent_rating_key=excluded.parent_rating_key,grandparent_rating_key=excluded.grandparent_rating_key,media_type=excluded.media_type,title=excluded.title,sort_title=excluded.sort_title,original_title=excluded.original_title,summary=excluded.summary,year=excluded.year,content_rating=excluded.content_rating,rating=excluded.rating,audience_rating=excluded.audience_rating,duration_ms=excluded.duration_ms,added_at=excluded.added_at,updated_at_source=excluded.updated_at_source,originally_available_at=excluded.originally_available_at,studio=excluded.studio,tagline=excluded.tagline,index_number=excluded.index_number,parent_index_number=excluded.parent_index_number,guid=excluded.guid,thumb_path=excluded.thumb_path,art_path=excluded.art_path,parent_thumb_path=excluded.parent_thumb_path,grandparent_thumb_path=excluded.grandparent_thumb_path,available=1,updated_at=excluded.updated_at`, id, l.ServerID, l.ID, m.RatingKey, m.ParentKey, m.GrandparentKey, m.Type, m.Title, m.SortTitle, m.OriginalTitle, m.Summary, m.Year, m.ContentRating, m.Rating, m.AudienceRating, m.DurationMS, m.AddedAt, m.SourceUpdatedAt, m.OriginallyAvailableAt, m.Studio, m.Tagline, m.Index, m.ParentIndex, m.GUID, m.Thumb, m.Art, m.ParentThumb, m.GrandparentThumb, now(), now())
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO sync_seen VALUES(?,?)", jobID, id); err != nil {
			return err
		}
		for _, table := range []string{"media_item_entities", "media_external_ids", "plex_media_versions"} {
			if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE media_item_id=?", id); err != nil {
				return err
			}
		}
		for _, t := range m.Tags {
			if _, err = tx.ExecContext(ctx, `INSERT INTO metadata_entities(plex_server_id,kind,source_key,name) VALUES(?,?,?,?) ON CONFLICT(plex_server_id,kind,source_key) DO UPDATE SET name=excluded.name`, l.ServerID, t.Kind, t.SourceKey, t.Name); err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO media_item_entities SELECT ?,id,?,? FROM metadata_entities WHERE plex_server_id=? AND kind=? AND source_key=?`, id, t.Relationship, t.Character, l.ServerID, t.Kind, t.SourceKey)
			if err != nil {
				return err
			}
		}
		for _, x := range m.ExternalIDs {
			if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO media_external_ids VALUES(?,?,?)", id, x.Provider, x.Value); err != nil {
				return err
			}
		}
		for vi, v := range m.Versions {
			vid := newID()
			mediaID := v.MediaID
			if mediaID == "" {
				mediaID = fmt.Sprintf("position:%d", vi)
			}
			partID := v.PartID
			if partID == "" {
				partID = fmt.Sprintf("position:%d", vi)
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO plex_media_versions VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, vid, id, mediaID, partID, v.Container, v.VideoCodec, v.AudioCodec, v.Resolution, v.Width, v.Height, v.Bitrate, v.DurationMS, v.File, v.Size, v.Optimized)
			if err != nil {
				return err
			}
			for _, stream := range v.Streams {
				_, err = tx.ExecContext(ctx, `INSERT INTO plex_media_streams(version_id,plex_stream_id,stream_type,codec,language,language_code,title,is_default,forced,hearing_impaired,channels,width,height,profile) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, vid, stream.ID, stream.Type, stream.Codec, stream.Language, stream.LanguageCode, stream.Title, stream.Default, stream.Forced, stream.HearingImpaired, stream.Channels, stream.Width, stream.Height, stream.Profile)
				if err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}
