package personalization

import (
	"afterglow.local/tv/internal/channels"
	"context"
	"encoding/json"
	"time"
)

type Template struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Field       string `json:"field"`
}

func Templates() []Template {
	return []Template{
		{"genre", "Genre Network", "Choose a synchronized genre. Shared Smart Random programming.", "genre"},
		{"actor", "Actor Network", "Movies and episodes featuring a selected person.", "actor"},
		{"director", "Director Network", "A selected director’s movies and episodes.", "director"},
		{"decade", "Decade Network", "An explicit ten-year release window.", "year"},
		{"comfort", "Comfort TV", "Choose a familiar show explicitly. No inferred taste required.", "show"},
		{"weekend", "Weekend Movies", "Movie programming with a genre you choose; available throughout the week.", "genre"},
		{"family", "Family Night", "G, PG, TV-Y, TV-Y7 or TV-G content from a chosen genre. Review metadata before use.", "genre"},
		{"seasonal", "Seasonal Network", "Choose an explicit holiday label and an inclusive local date window.", "label"},
	}
}

type TemplateInput struct {
	GroupID       *string `json:"groupId"`
	ScheduleMode  string  `json:"scheduleMode"`
	RepeatDays    *int    `json:"repeatDays"`
	FutureDays    *int    `json:"futureDays"`
	YearFrom      int     `json:"yearFrom"`
	YearTo        int     `json:"yearTo"`
	MinimumRating float64 `json:"minimumRating"`
	Template      string  `json:"template"`
	Selection     string  `json:"selection"`
	Name          string  `json:"name"`
	Number        string  `json:"number"`
	MediaType     string  `json:"mediaType"`
	Decade        int     `json:"decade"`
	StartDate     string  `json:"startDate"`
	EndDate       string  `json:"endDate"`
	Timezone      string  `json:"timezone"`
}

func WindowBounds(start, end, zone string) (int64, int64, error) {
	loc, e := time.LoadLocation(zone)
	if e != nil {
		return 0, 0, ErrInvalid
	}
	a, e := time.ParseInLocation("2006-01-02", start, loc)
	if e != nil {
		return 0, 0, ErrInvalid
	}
	b, e := time.ParseInLocation("2006-01-02", end, loc)
	if e != nil || a.UnixMilli() < 0 || b.Before(a) || b.Sub(a) > 366*24*time.Hour {
		return 0, 0, ErrInvalid
	}
	return a.UnixMilli(), b.AddDate(0, 0, 1).UnixMilli(), nil
}
func TemplateSource(v TemplateInput) (channels.Source, error) {
	if (v.ScheduleMode != "" && v.ScheduleMode != "SEQUENTIAL" && v.ScheduleMode != "RANDOM" && v.ScheduleMode != "SMART_RANDOM") || (v.RepeatDays != nil && (*v.RepeatDays < 0 || *v.RepeatDays > 365)) || (v.FutureDays != nil && (*v.FutureDays < 1 || *v.FutureDays > 30)) || v.MinimumRating < 0 || v.MinimumRating > 10 {
		return channels.Source{}, ErrInvalid
	}
	if (v.YearFrom != 0 && (v.YearFrom < 1880 || v.YearFrom > 2100)) || (v.YearTo != 0 && (v.YearTo < 1880 || v.YearTo > 2100)) || (v.YearFrom != 0 && v.YearTo != 0 && v.YearFrom > v.YearTo) {
		return channels.Source{}, ErrInvalid
	}
	condition := func(field, op string, value any) channels.Node {
		b, _ := json.Marshal(value)
		return channels.Node{Type: "condition", Field: field, Operator: op, Value: b}
	}
	nodes := []channels.Node{{Type: "condition", Field: "availability", Operator: "is_true"}}
	switch v.Template {
	case "genre", "actor", "director":
		if v.Selection == "" {
			return channels.Source{}, ErrInvalid
		}
		nodes = append(nodes, condition(v.Template, "is", v.Selection))
	case "comfort":
		if v.Selection == "" || (v.MediaType != "" && v.MediaType != "episode") {
			return channels.Source{}, ErrInvalid
		}
		return channels.Source{Type: "SHOW", Show: &channels.ShowConfig{ShowID: v.Selection, EpisodeScope: "ALL"}}, nil
	case "decade":
		if v.Decade < 1880 || v.Decade > 2090 || v.Decade%10 != 0 {
			return channels.Source{}, ErrInvalid
		}
		nodes = append(nodes, condition("year", "between", []int{v.Decade, v.Decade + 9}))
	case "weekend", "family":
		if v.Selection == "" {
			return channels.Source{}, ErrInvalid
		}
		nodes = append(nodes, condition("genre", "is", v.Selection))
		if v.Template == "weekend" {
			nodes = append(nodes, condition("media_type", "is", "movie"))
		} else {
			nodes = append(nodes, condition("content_rating", "in", []string{"G", "PG", "TV-Y", "TV-Y7", "TV-G"}))
		}
	case "seasonal":
		if v.Selection == "" {
			return channels.Source{}, ErrInvalid
		}
		if _, _, e := WindowBounds(v.StartDate, v.EndDate, v.Timezone); e != nil {
			return channels.Source{}, e
		}
		nodes = append(nodes, condition("label", "is", v.Selection))
	default:
		return channels.Source{}, ErrInvalid
	}
	if v.MediaType != "" {
		if v.MediaType != "movie" && v.MediaType != "episode" {
			return channels.Source{}, ErrInvalid
		}
		nodes = append(nodes, condition("media_type", "is", v.MediaType))
	}
	if v.YearFrom > 0 {
		nodes = append(nodes, condition("year", "greater_than_or_equal", v.YearFrom))
	}
	if v.YearTo > 0 {
		nodes = append(nodes, condition("year", "less_than_or_equal", v.YearTo))
	}
	if v.MinimumRating > 0 {
		nodes = append(nodes, condition("rating", "greater_than_or_equal", v.MinimumRating))
	}
	return channels.Source{Type: "RULE", Rule: &channels.Rule{Version: 1, Root: channels.Node{Type: "group", Operator: "AND", Children: nodes}}}, nil
}
func (s *Service) SaveWindow(ctx context.Context, id, start, end, zone, actor string) error {
	a, b, e := WindowBounds(start, end, zone)
	if e != nil {
		return e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, `INSERT INTO channel_activation_windows(channel_id,start_date,end_date,timezone,start_ms,end_ms) VALUES(?,?,?,?,?,?) ON CONFLICT(channel_id) DO UPDATE SET start_date=excluded.start_date,end_date=excluded.end_date,timezone=excluded.timezone,start_ms=excluded.start_ms,end_ms=excluded.end_ms`, id, start, end, zone, a, b)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO audit_log(user_id,action,created_at,target_id,summary) VALUES(?,'CHANNEL_ACTIVATION_UPDATED',unixepoch(),?,'Explicit local-date activation window saved; future repair required')`, actor, id)
	if e != nil {
		return e
	}
	return tx.Commit()
}
