// Package guide reads persisted broadcasts and personal preferences, without Plex HTTP.
package guide

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	db  *sql.DB
	Now func() time.Time
}

func New(db *sql.DB) *Service { return &Service{db, time.Now} }

type Error struct{ Code, Message string }

func (e *Error) Error() string { return e.Message }
func missing() error           { return &Error{"NOT_FOUND", "Program or channel is no longer available"} }

type Channel struct {
	ID         string    `json:"id"`
	Number     string    `json:"number"`
	Name       string    `json:"name"`
	Logo       string    `json:"logo"`
	GroupID    string    `json:"groupId"`
	Group      string    `json:"group"`
	Favorite   bool      `json:"favorite"`
	Status     string    `json:"status"`
	RecentKind string    `json:"recentKind"`
	Programs   []Program `json:"programs"`
}
type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Program struct {
	ID         string  `json:"id"`
	ChannelID  string  `json:"channelId"`
	MediaID    string  `json:"mediaId"`
	Title      string  `json:"title"`
	ShowTitle  string  `json:"showTitle"`
	Type       string  `json:"type"`
	Start      int64   `json:"start"`
	End        int64   `json:"end"`
	Duration   int64   `json:"durationMs"`
	Season     *int    `json:"season"`
	Episode    *int    `json:"episode"`
	Available  bool    `json:"available"`
	Artwork    string  `json:"artwork"`
	State      string  `json:"state"`
	Progress   float64 `json:"progress"`
	CanRestart bool    `json:"canRestart"`
}
type Detail struct {
	Program
	Channel       Channel  `json:"channel"`
	Summary       string   `json:"summary"`
	Year          *int     `json:"year"`
	Rating        *float64 `json:"rating"`
	ContentRating string   `json:"contentRating"`
	AddedAt       *int64   `json:"addedAt"`
	Genres        []string `json:"genres"`
	Background    string   `json:"background"`
	ServerTime    int64    `json:"serverTime"`
	Timezone      string   `json:"timezone"`
}
type Query struct {
	Start, End    int64
	Group         string
	Favorites     bool
	Offset, Limit int
	Kind          string
}
type Window struct {
	Channels   []Channel `json:"channels"`
	Groups     []Group   `json:"groups"`
	ServerTime int64     `json:"serverTime"`
	Timezone   string    `json:"timezone"`
	More       bool      `json:"more"`
	Start      int64     `json:"start"`
	End        int64     `json:"end"`
	Warnings   []string  `json:"warnings"`
}

func classify(p *Program, now int64) {
	p.State = "FUTURE"
	if p.End <= now {
		p.State = "PAST"
	} else if p.Start <= now {
		p.State = "CURRENT"
		p.Progress = min(100, max(0, float64(now-p.Start)/float64(p.Duration)*100))
	}
	p.CanRestart = p.Available && p.Start <= now
	p.Artwork = "/api/v1/watch/channels/" + p.ChannelID + "/programs/" + p.ID + "/artwork"
}

const programColumns = `p.id,p.channel_id,p.media_item_id,m.title,COALESCE(sh.title,''),m.media_type,p.start_time_utc,p.end_time_utc,p.duration_ms,m.parent_index_number,m.index_number,m.available`
const programJoin = ` FROM channel_programs p JOIN plex_media_items m ON m.id=p.media_item_id LEFT JOIN plex_media_items sh ON sh.plex_server_id=m.plex_server_id AND sh.plex_rating_key=m.grandparent_rating_key `

type scanner interface{ Scan(...any) error }

func scanProgram(row scanner) (Program, error) {
	var p Program
	e := row.Scan(&p.ID, &p.ChannelID, &p.MediaID, &p.Title, &p.ShowTitle, &p.Type, &p.Start, &p.End, &p.Duration, &p.Season, &p.Episode, &p.Available)
	return p, e
}
func (s *Service) Window(ctx context.Context, user string, q Query) (Window, error) {
	now := s.Now().UnixMilli()
	o := Window{Channels: []Channel{}, Groups: []Group{}, Warnings: []string{}, ServerTime: now, Start: q.Start, End: q.End}
	if q.Kind == "now" {
		q.Start = now
		q.End = now + 32*86400000
	} else if q.Kind == "soon" {
		q.Start = now
		q.End = now + 1800000
	} else if q.Start < 0 || q.End <= q.Start || q.End-q.Start > 86400000 {
		return o, &Error{"INVALID", "Choose a guide window of at most 24 hours"}
	}
	if q.Offset < 0 || q.Offset > 100000 {
		return o, &Error{"INVALID", "Invalid channel page"}
	}
	if q.Limit < 1 || q.Limit > 100 {
		q.Limit = 50
	}
	o.Start = q.Start
	o.End = q.End
	if e := s.db.QueryRowContext(ctx, "SELECT timezone FROM app_settings WHERE id=1").Scan(&o.Timezone); e != nil {
		return o, e
	}
	rows, e := s.db.QueryContext(ctx, `SELECT id,name FROM channel_groups ORDER BY sort_order,name,id`)
	if e != nil {
		return o, e
	}
	for rows.Next() {
		var g Group
		if e = rows.Scan(&g.ID, &g.Name); e != nil {
			rows.Close()
			return o, e
		}
		o.Groups = append(o.Groups, g)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return o, e
	}
	filter := `c.enabled=1 AND c.visibility='VISIBLE' AND COALESCE(pref.hidden,0)=0 AND EXISTS(SELECT 1 FROM channel_programs WHERE channel_id=c.id)`
	args := []any{user, user, user}
	filter += ` AND (NOT EXISTS(SELECT 1 FROM channel_activation_windows aw WHERE aw.channel_id=c.id) OR EXISTS(SELECT 1 FROM channel_activation_windows aw WHERE aw.channel_id=c.id AND aw.start_ms<=? AND aw.end_ms>?) OR EXISTS(SELECT 1 FROM channel_programs p WHERE p.channel_id=c.id AND p.start_time_utc<=? AND p.end_time_utc>?))`
	args = append(args, now, now, now, now)
	if q.Group != "" {
		filter += " AND c.group_id=?"
		args = append(args, q.Group)
	}
	if q.Favorites {
		filter += " AND f.user_id IS NOT NULL"
	}
	args = append(args, q.Limit+1, q.Offset)
	rows, e = s.db.QueryContext(ctx, `SELECT c.id,c.channel_number,c.name,COALESCE(c.group_id,''),COALESCE(g.name,''),f.user_id IS NOT NULL,COALESCE(st.status,'NEEDS_GENERATION'),COALESCE(json_extract(src.config_json,'$.recent.kind'),'') FROM channels c LEFT JOIN channel_groups g ON g.id=c.group_id LEFT JOIN user_channel_favorites f ON f.channel_id=c.id AND f.user_id=? LEFT JOIN user_channel_preferences pref ON pref.channel_id=c.id AND pref.user_id=? LEFT JOIN viewer_preferences vp ON vp.user_id=? LEFT JOIN scheduler_state st ON st.channel_id=c.id LEFT JOIN channel_sources src ON src.channel_id=c.id WHERE `+filter+` ORDER BY CASE WHEN vp.order_mode='PERSONAL' THEN COALESCE(pref.position,2147483647) ELSE 0 END,CAST(c.channel_number AS REAL),c.channel_number,c.id LIMIT ? OFFSET ?`, args...)
	if e != nil {
		return o, e
	}
	for rows.Next() {
		var c Channel
		if e = rows.Scan(&c.ID, &c.Number, &c.Name, &c.GroupID, &c.Group, &c.Favorite, &c.Status, &c.RecentKind); e != nil {
			rows.Close()
			return o, e
		}
		c.Logo = "/api/v1/watch/channels/" + c.ID + "/artwork/logo"
		c.Programs = []Program{}
		o.Channels = append(o.Channels, c)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return o, e
	}
	o.More = len(o.Channels) > q.Limit
	if o.More {
		o.Channels = o.Channels[:q.Limit]
	}
	if len(o.Channels) == 0 {
		return o, nil
	}
	marks := []string{}
	args = []any{}
	positions := map[string]int{}
	for i, c := range o.Channels {
		marks = append(marks, "?")
		args = append(args, c.ID)
		positions[c.ID] = i
	}
	args = append(args, q.Start, q.End)
	sqlQuery := "SELECT " + programColumns + programJoin + " WHERE p.channel_id IN (" + strings.Join(marks, ",") + ") AND p.end_time_utc>? AND p.start_time_utc<?"
	if q.Kind == "soon" {
		sqlQuery += " AND p.start_time_utc>?"
		args = append(args, now)
	}
	if q.Kind == "now" {
		// Rank the indexed timeline once, then join metadata only for current/next.
		sqlQuery = "WITH chosen AS (SELECT id,ROW_NUMBER() OVER(PARTITION BY channel_id ORDER BY start_time_utc) AS rank FROM channel_programs WHERE channel_id IN (" + strings.Join(marks, ",") + ") AND end_time_utc>?) SELECT " + programColumns + programJoin + " JOIN chosen ON chosen.id=p.id WHERE chosen.rank<=2"
		args = append(args[:len(o.Channels)], now)
	}
	sqlQuery += " ORDER BY p.channel_id,p.start_time_utc LIMIT 10001"
	rows, e = s.db.QueryContext(ctx, sqlQuery, args...)
	if e != nil {
		return o, e
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		p, e := scanProgram(rows)
		if e != nil {
			return o, e
		}
		n++
		if n > 10000 {
			return o, &Error{"INVALID", "Guide window is too dense; choose fewer channels or a shorter interval"}
		}
		i := positions[p.ChannelID]
		list := o.Channels[i].Programs
		if p.Duration <= 0 || p.End <= p.Start || p.End-p.Start != p.Duration || len(list) > 0 && p.Start < list[len(list)-1].End {
			o.Warnings = append(o.Warnings, fmt.Sprintf("Invalid interval omitted on channel %s", o.Channels[i].Number))
			continue
		}
		classify(&p, now)
		o.Channels[i].Programs = append(list, p)
	}
	return o, rows.Err()
}
func (s *Service) Detail(ctx context.Context, user, id string) (Detail, error) {
	o := Detail{Genres: []string{}, ServerTime: s.Now().UnixMilli()}
	p, e := scanProgram(s.db.QueryRowContext(ctx, "SELECT "+programColumns+programJoin+` JOIN channels c ON c.id=p.channel_id WHERE p.id=? AND c.enabled=1 AND c.visibility='VISIBLE'`, id))
	if errors.Is(e, sql.ErrNoRows) {
		return o, missing()
	}
	if e != nil {
		return o, e
	}
	if p.Duration <= 0 || p.End <= p.Start || p.End-p.Start != p.Duration {
		return o, &Error{"INVALID", "Program timing is unavailable"}
	}
	classify(&p, o.ServerTime)
	o.Program = p
	e = s.db.QueryRowContext(ctx, `SELECT m.summary,m.year,m.rating,m.content_rating,m.added_at,c.channel_number,c.name,COALESCE(c.group_id,''),COALESCE(g.name,''),EXISTS(SELECT 1 FROM user_channel_favorites WHERE user_id=? AND channel_id=c.id),COALESCE(json_extract(src.config_json,'$.recent.kind'),''),(SELECT timezone FROM app_settings WHERE id=1) FROM plex_media_items m JOIN channels c ON c.id=? LEFT JOIN channel_groups g ON g.id=c.group_id LEFT JOIN channel_sources src ON src.channel_id=c.id WHERE m.id=?`, user, p.ChannelID, p.MediaID).Scan(&o.Summary, &o.Year, &o.Rating, &o.ContentRating, &o.AddedAt, &o.Channel.Number, &o.Channel.Name, &o.Channel.GroupID, &o.Channel.Group, &o.Channel.Favorite, &o.Channel.RecentKind, &o.Timezone)
	if e != nil {
		return o, e
	}
	o.Channel.ID = p.ChannelID
	o.Channel.Logo = "/api/v1/watch/channels/" + p.ChannelID + "/artwork/logo"
	o.Background = p.Artwork + "?kind=background"
	rows, e := s.db.QueryContext(ctx, `SELECT DISTINCT e.name FROM metadata_entities e JOIN media_item_entities r ON r.entity_id=e.id WHERE e.kind='genre' AND r.media_item_id IN (?,(SELECT sh.id FROM plex_media_items m JOIN plex_media_items sh ON sh.plex_server_id=m.plex_server_id AND sh.plex_rating_key=m.grandparent_rating_key WHERE m.id=?)) ORDER BY e.name`, p.MediaID, p.MediaID)
	if e != nil {
		return o, e
	}
	defer rows.Close()
	for rows.Next() {
		var g string
		if e = rows.Scan(&g); e != nil {
			return o, e
		}
		o.Genres = append(o.Genres, g)
	}
	return o, rows.Err()
}
func (s *Service) Favorites(ctx context.Context, user string) ([]string, error) {
	out := []string{}
	rows, e := s.db.QueryContext(ctx, `SELECT f.channel_id FROM user_channel_favorites f JOIN channels c ON c.id=f.channel_id WHERE f.user_id=? AND c.enabled=1 AND c.visibility='VISIBLE' ORDER BY CAST(c.channel_number AS REAL),c.channel_number,c.id`, user)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return out, e
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
func (s *Service) Favorite(ctx context.Context, user, id string, on bool) error {
	if !on {
		_, e := s.db.ExecContext(ctx, "DELETE FROM user_channel_favorites WHERE user_id=? AND channel_id=?", user, id)
		return e
	}
	r, e := s.db.ExecContext(ctx, `INSERT INTO user_channel_favorites(user_id,channel_id,created_at) SELECT ?,id,? FROM channels WHERE id=? AND enabled=1 AND visibility='VISIBLE' ON CONFLICT DO NOTHING`, user, s.Now().Unix(), id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		var count int
		e = s.db.QueryRowContext(ctx, "SELECT count(*) FROM user_channel_favorites WHERE user_id=? AND channel_id=?", user, id).Scan(&count)
		if e != nil {
			return e
		}
		if count == 0 {
			return missing()
		}
	}
	return nil
}
func (s *Service) Last(ctx context.Context, user string) (string, error) {
	var id string
	e := s.db.QueryRowContext(ctx, `SELECT c.id FROM viewer_preferences v JOIN channels c ON c.id=v.last_channel_id WHERE v.user_id=? AND c.enabled=1 AND c.visibility='VISIBLE'`, user).Scan(&id)
	if e == sql.ErrNoRows {
		return "", nil
	}
	return id, e
}
