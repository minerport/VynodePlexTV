package httpapi

import (
	"afterglow.local/tv/internal/channels"
	"afterglow.local/tv/internal/personalization"
	"database/sql"
	"net/http"
)

func (a *API) smartTemplateRoutes(mux *http.ServeMux) {
	c := channels.New(a.db, a.cfg.Dir)
	p := personalization.New(a.db)
	mux.HandleFunc("GET /api/v1/smart-channel-templates", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		reply(w, 200, map[string]any{"items": personalization.Templates()})
	}))
	for _, action := range []string{"preview", "instantiate"} {
		mux.HandleFunc("POST /api/v1/smart-channel-templates/"+action, a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
			var v personalization.TemplateInput
			if !decodeChannel(w, r, &v) {
				return
			}
			source, e := personalization.TemplateSource(v)
			if e != nil {
				problem(w, 422, "Choose valid template options and dates")
				return
			}
			preview, e := c.Preview(r.Context(), channels.PreviewRequest{Source: source, Limit: 12})
			if e != nil {
				a.channelError(w, e)
				return
			}
			if action == "preview" {
				mode := "SMART_RANDOM"
				if source.Type == "SHOW" {
					mode = "SEQUENTIAL"
				}
				if v.ScheduleMode != "" {
					mode = v.ScheduleMode
				}
				reply(w, 200, map[string]any{"preview": preview, "source": source, "scheduleMode": mode, "presentation": "Inherits existing channel defaults"})
				return
			}
			if v.Number == "" {
				v.Number, e = c.NextNumber(r.Context())
				if e != nil {
					a.channelError(w, e)
					return
				}
			}
			group := v.GroupID
			if group == nil {
				inferred := ""
				if source.Type == "SHOW" || v.MediaType == "episode" {
					inferred = "tv"
				} else if v.MediaType == "movie" || v.Template == "weekend" {
					inferred = "movies"
				}
				if inferred != "" {
					group = &inferred
				}
			}
			if group != nil && *group == "" {
				group = nil
			}
			channel, e := c.Save(r.Context(), "", channels.Definition{Number: v.Number, Name: v.Name, ShortName: v.Name, GroupID: group, Description: "Created from an explicit shared smart-channel template.", Enabled: v.Template != "seasonal", Visibility: "VISIBLE", Logo: channels.Appearance{Source: "AUTO"}, Background: channels.Appearance{Source: "AUTO"}, Source: source}, u.User.ID)
			if e != nil {
				a.channelError(w, e)
				return
			}
			if v.ScheduleMode != "" || v.RepeatDays != nil || v.FutureDays != nil {
				settings, err := a.scheduler.Settings(r.Context(), channel.ID)
				if err == nil {
					if v.ScheduleMode != "" {
						settings.Mode = v.ScheduleMode
					}
					if v.RepeatDays != nil {
						settings.MovieDays = *v.RepeatDays
						settings.EpisodeDays = *v.RepeatDays
					}
					if v.FutureDays != nil {
						settings.FutureDays = *v.FutureDays
					}
					_, err = a.scheduler.SaveSettings(r.Context(), channel.ID, settings, u.User.ID)
				}
				if err != nil {
					problem(w, 500, "Channel created; scheduling preferences could not be saved. Edit the channel to retry.")
					return
				}
			}
			if v.Template == "seasonal" {
				if e = p.SaveWindow(r.Context(), channel.ID, v.StartDate, v.EndDate, v.Timezone, u.User.ID); e != nil {
					problem(w, 500, "Channel remains disabled because seasonal dates could not be saved.")
					return
				}
			}
			if v.Template == "seasonal" {
				channel.Enabled = true
				channel, e = c.Save(r.Context(), channel.ID, channel.Definition, u.User.ID)
				if e != nil {
					a.channelError(w, e)
					return
				}
			}
			if _, e = a.scheduler.Enqueue(r.Context(), channel.ID, "SCHEDULE_GENERATE", u.User.ID); e != nil {
				problem(w, 500, "Channel created; schedule job could not be queued. Use Scheduling Studio to generate it.")
				return
			}
			reply(w, 201, channel)
		}))
	}
	mux.HandleFunc("GET /api/v1/channels/{id}/activation", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var start, end, zone string
		e := a.db.QueryRowContext(r.Context(), "SELECT start_date,end_date,timezone FROM channel_activation_windows WHERE channel_id=?", r.PathValue("id")).Scan(&start, &end, &zone)
		if e != nil && e != sql.ErrNoRows {
			a.fail(w, e)
			return
		}
		reply(w, 200, map[string]any{"enabled": e == nil, "startDate": start, "endDate": end, "timezone": zone})
	}))
	mux.HandleFunc("DELETE /api/v1/channels/{id}/activation", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		if _, e := a.db.ExecContext(r.Context(), "DELETE FROM channel_activation_windows WHERE channel_id=?", r.PathValue("id")); e != nil {
			a.fail(w, e)
			return
		}
		if _, e := a.scheduler.Enqueue(r.Context(), r.PathValue("id"), "SCHEDULE_REPAIR", u.User.ID); e != nil {
			a.fail(w, e)
			return
		}
		reply(w, 200, map[string]bool{"ok": true})
	}))
	mux.HandleFunc("PATCH /api/v1/channels/{id}/activation", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var v struct {
			StartDate string `json:"startDate"`
			EndDate   string `json:"endDate"`
			Timezone  string `json:"timezone"`
		}
		if !decodeChannel(w, r, &v) {
			return
		}
		if e := p.SaveWindow(r.Context(), r.PathValue("id"), v.StartDate, v.EndDate, v.Timezone, u.User.ID); e != nil {
			problem(w, 422, "Unable to save activation window")
			return
		}
		if _, e := a.scheduler.Enqueue(r.Context(), r.PathValue("id"), "SCHEDULE_REPAIR", u.User.ID); e != nil {
			a.fail(w, e)
			return
		}
		reply(w, 200, map[string]bool{"ok": true})
	}))
}
