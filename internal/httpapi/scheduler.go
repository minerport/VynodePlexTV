package httpapi

import (
	"afterglow.local/tv/internal/scheduler"
	"errors"
	"net/http"
	"time"
)

func (a *API) Scheduler() *scheduler.Service { return a.scheduler }
func (a *API) scheduleError(w http.ResponseWriter, e error) {
	var v *scheduler.Error
	if errors.As(e, &v) {
		code := 422
		if v.Code == "NOT_FOUND" {
			code = 404
		}
		if v.Code == "CONFLICT" {
			code = 409
		}
		reply(w, code, v)
		return
	}
	a.log.Error("Schedule request failed")
	problem(w, 500, scheduler.SafeError(e))
}
func (a *API) scheduleRoutes(mux *http.ServeMux) {
	s := a.scheduler
	mux.HandleFunc("GET /api/v1/scheduler", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		offset, limit := pageParams(r)
		ids, e := s.ChannelIDs(r.Context(), offset, limit)
		if e != nil {
			a.scheduleError(w, e)
			return
		}
		out := []scheduler.Status{}
		for _, id := range ids {
			v, e := s.Status(r.Context(), id)
			if e != nil {
				a.scheduleError(w, e)
				return
			}
			out = append(out, v)
		}
		reply(w, 200, out)
	}))
	mux.HandleFunc("GET /api/v1/scheduler/jobs", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		v, e := s.Jobs(r.Context(), r.URL.Query().Get("channel"))
		if e != nil {
			a.scheduleError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("GET /api/v1/channels/{id}/schedule/settings", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		v, e := s.Settings(r.Context(), r.PathValue("id"))
		if e != nil {
			a.scheduleError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("PATCH /api/v1/channels/{id}/schedule/settings", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var v scheduler.Settings
		if !decodeChannel(w, r, &v) {
			return
		}
		v, e := s.SaveSettings(r.Context(), r.PathValue("id"), v, u.User.ID)
		if e != nil {
			a.scheduleError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("GET /api/v1/channels/{id}/schedule/status", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		v, e := s.Status(r.Context(), r.PathValue("id"))
		if e != nil {
			a.scheduleError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	for action, kind := range map[string]string{"generate": "SCHEDULE_GENERATE", "repair": "SCHEDULE_REPAIR", "regenerate": "SCHEDULE_REGENERATE"} {
		mux.HandleFunc("POST /api/v1/channels/{id}/schedule/"+action, a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
			id, e := s.Enqueue(r.Context(), r.PathValue("id"), kind, u.User.ID)
			if e != nil {
				a.scheduleError(w, e)
				return
			}
			reply(w, 202, map[string]string{"jobId": id})
		}))
	}
	read := func(fn func(http.ResponseWriter, *http.Request)) {
		mux.HandleFunc("GET /api/v1/channels/{id}/schedule", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
			if e := s.Readable(r.Context(), r.PathValue("id"), u.User.Role != "USER"); e != nil {
				a.scheduleError(w, e)
				return
			}
			fn(w, r)
		}))
	}
	read(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		v, e := s.Settings(r.Context(), id)
		if e != nil {
			a.scheduleError(w, e)
			return
		}
		var start, end time.Time
		if day := r.URL.Query().Get("day"); day != "" {
			start, end, e = scheduler.DayRange(day, v.Timezone)
		} else {
			start, e = time.Parse(time.RFC3339Nano, r.URL.Query().Get("start"))
			if e == nil {
				end, e = time.Parse(time.RFC3339Nano, r.URL.Query().Get("end"))
			}
		}
		if e != nil {
			problem(w, 422, "Use day=YYYY-MM-DD or UTC RFC3339 start and end")
			return
		}
		offset, limit := pageParams(r)
		out, e := s.Programs(r.Context(), id, start.UnixMilli(), end.UnixMilli(), offset, limit)
		if e != nil {
			a.scheduleError(w, e)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("GET /api/v1/channels/{id}/schedule/at", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
		id := r.PathValue("id")
		if e := s.Readable(r.Context(), id, u.User.Role != "USER"); e != nil {
			a.scheduleError(w, e)
			return
		}
		at := time.Now()
		var e error
		if q := r.URL.Query().Get("time"); q != "" {
			at, e = time.Parse(time.RFC3339Nano, q)
		}
		if e != nil {
			problem(w, 422, "Use a RFC3339 timestamp")
			return
		}
		current, e := s.GetProgramAt(r.Context(), id, at)
		if e != nil {
			a.scheduleError(w, e)
			return
		}
		next, e := s.Next(r.Context(), id, at)
		if e != nil {
			a.scheduleError(w, e)
			return
		}
		prev, e := s.Previous(r.Context(), id, at)
		if e != nil {
			a.scheduleError(w, e)
			return
		}
		reply(w, 200, map[string]any{"current": current, "next": next, "previous": prev})
	}))
}
