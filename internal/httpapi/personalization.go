package httpapi

import (
	"afterglow.local/tv/internal/personalization"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

func (a *API) personalizationRoutes(mux *http.ServeMux) {
	s := personalization.New(a.db)
	for _, path := range []string{"for-you", "whats-good"} {
		mux.HandleFunc("GET /api/v1/personalization/"+path, a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
			v, e := s.Recommend(r.Context(), u.User.ID, time.Now(), r.URL.Query().Get("debug") == "true")
			if e != nil {
				a.fail(w, e)
				return
			}
			reply(w, 200, v)
		}))
	}
	mux.HandleFunc("GET /api/v1/personalization/affinities", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
		v, e := s.Affinities(r.Context(), u.User.ID)
		if e != nil {
			a.fail(w, e)
			return
		}
		reply(w, 200, map[string]any{"items": v, "rankingVersion": personalization.RankingVersion})
	}))
	mux.HandleFunc("POST /api/v1/personalization/rebuild", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
		id, e := s.Enqueue(r.Context(), u.User.ID)
		if e != nil {
			a.fail(w, e)
			return
		}
		reply(w, 202, map[string]string{"jobId": id})
	}))
	mux.HandleFunc("GET /api/v1/users/me/history", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		v, e := s.History(r.Context(), u.User.ID, offset)
		if e != nil {
			problem(w, 400, "Unable to load viewing history")
			return
		}
		reply(w, 200, map[string]any{"items": v, "retentionDays": personalization.HistoryRetentionDays})
	}))
	mux.HandleFunc("DELETE /api/v1/users/me/history", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
		if e := s.ClearHistory(r.Context(), u.User.ID); e != nil {
			a.fail(w, e)
			return
		}
		reply(w, 200, map[string]bool{"ok": true})
	}))
	mux.HandleFunc("GET /api/v1/watch/channel-number/{number}", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
		var id string
		e := a.db.QueryRowContext(r.Context(), "SELECT id FROM channels WHERE channel_number=? AND enabled=1 AND visibility='VISIBLE'", r.PathValue("number")).Scan(&id)
		if e != nil {
			problem(w, 404, "Channel unavailable")
			return
		}
		reply(w, 200, map[string]string{"channelId": id})
	}))
	result := func(w http.ResponseWriter, err error) {
		if errors.Is(err, personalization.ErrInvalid) {
			problem(w, 400, err.Error())
		} else if err != nil {
			a.fail(w, err)
		} else {
			reply(w, 200, map[string]bool{"ok": true})
		}
	}
	mux.HandleFunc("GET /api/v1/users/me/channels", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
		v, e := s.Channels(r.Context(), u.User.ID)
		if e != nil {
			a.fail(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("PATCH /api/v1/users/me/channels/order", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
		var v struct {
			Mode       string   `json:"mode"`
			ChannelIDs []string `json:"channelIds"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&v); e != nil {
			problem(w, 400, "Invalid channel order")
			return
		}
		result(w, s.Order(r.Context(), u.User.ID, v.Mode, v.ChannelIDs))
	}))
	for _, method := range []string{"POST", "DELETE"} {
		mux.HandleFunc(method+" /api/v1/users/me/channels/{id}/hide", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
			result(w, s.Hide(r.Context(), u.User.ID, r.PathValue("id"), r.Method == "POST"))
		}))
	}
	mux.HandleFunc("POST /api/v1/users/me/channels/reset", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) { result(w, s.Reset(r.Context(), u.User.ID)) }))
}
