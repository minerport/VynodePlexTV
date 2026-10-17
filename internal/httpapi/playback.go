package httpapi

import (
	"afterglow.local/tv/internal/channels"
	"afterglow.local/tv/internal/playback"
	"errors"
	"io"
	"net/http"
	"time"
)

func (a *API) playbackError(w http.ResponseWriter, e error) {
	status := 502
	var v *playback.Error
	if errors.As(e, &v) {
		switch v.Code {
		case "INVALID":
			status = 400
		case "NOT_FOUND", "EMPTY":
			status = 404
		case "BOUNDARY":
			status = 409
		case "BUSY":
			status = 429
		}
	}
	problem(w, status, playback.SafeError(e))
}
func (a *API) viewer(next func(http.ResponseWriter, *http.Request, Session)) http.HandlerFunc {
	return a.auth(func(w http.ResponseWriter, r *http.Request, s Session) {
		if a.playback == nil {
			problem(w, 503, "Playback service unavailable")
			return
		}
		next(w, r, s)
	})
}
func authHash(r *http.Request) string {
	c, e := r.Cookie("afterglow_session")
	if e != nil {
		return ""
	}
	return digest(c.Value)
}
func (a *API) playbackRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/on-now", a.viewer(func(w http.ResponseWriter, r *http.Request, s Session) {
		v, e := a.playback.OnNow(r.Context())
		if e != nil {
			a.playbackError(w, e)
			return
		}
		reply(w, 200, map[string]any{"items": v, "serverTime": time.Now().UnixMilli()})
	}))
	mux.HandleFunc("POST /api/v1/playback/tune", a.viewer(func(w http.ResponseWriter, r *http.Request, s Session) {
		var body playback.TuneRequest
		if !decodeChannel(w, r, &body) {
			return
		}
		v, e := a.playback.Tune(r.Context(), s.User.ID, authHash(r), body)
		if e != nil {
			a.playbackError(w, e)
			return
		}
		reply(w, 201, v)
	}))
	mux.HandleFunc("GET /api/v1/playback/active", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, s Session) {
		if a.playback == nil {
			reply(w, 200, []any{})
			return
		}
		v, e := a.playback.Active(r.Context())
		if e != nil {
			a.playbackError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("GET /api/v1/playback/{id}", a.viewer(func(w http.ResponseWriter, r *http.Request, s Session) {
		v, e := a.playback.Get(s.User.ID, r.PathValue("id"))
		if e != nil {
			a.playbackError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("POST /api/v1/playback/{id}/stop", a.viewer(func(w http.ResponseWriter, r *http.Request, s Session) {
		if e := a.playback.Stop(r.Context(), s.User.ID, r.PathValue("id")); e != nil {
			a.playbackError(w, e)
			return
		}
		reply(w, 200, map[string]bool{"stopped": true})
	}))
	for _, route := range []string{"heartbeat", "seek"} {
		mux.HandleFunc("POST /api/v1/playback/{id}/"+route, a.viewer(func(w http.ResponseWriter, r *http.Request, s Session) {
			var body playback.Heartbeat
			if !decodeChannel(w, r, &body) {
				return
			}
			v, e := a.playback.Heartbeat(r.Context(), s.User.ID, r.PathValue("id"), body)
			if e != nil {
				a.playbackError(w, e)
				return
			}
			reply(w, 200, v)
		}))
	}
	mux.HandleFunc("POST /api/v1/playback/{id}/go-live", a.viewer(func(w http.ResponseWriter, r *http.Request, s Session) {
		v, e := a.playback.GoLive(r.Context(), s.User.ID, authHash(r), r.PathValue("id"))
		if e != nil {
			a.playbackError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("GET /api/v1/playback/{id}/stream/{resource}", a.viewer(func(w http.ResponseWriter, r *http.Request, s Session) {
		up, e := a.playback.Fetch(r.Context(), s.User.ID, r.PathValue("id"), r.PathValue("resource"), r.Header.Get("Range"))
		if e != nil {
			a.playbackError(w, e)
			return
		}
		defer up.Body.Close()
		for _, k := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
			if v := up.Header.Get(k); v != "" {
				w.Header().Set(k, v)
			}
		}
		w.WriteHeader(up.StatusCode)
		controller := http.NewResponseController(w)
		buf := make([]byte, 64*1024)
		for {
			controller.SetWriteDeadline(time.Now().Add(30 * time.Second))
			n, e := up.Body.Read(buf)
			if n > 0 {
				if _, err := w.Write(buf[:n]); err != nil {
					return
				}
			}
			if e != nil {
				if e != io.EOF {
					a.log.Warn("Playback stream interrupted", "session", r.PathValue("id"))
				}
				return
			}
		}
	}))
	mux.HandleFunc("GET /api/v1/watch/channels/{id}/artwork/{kind}", a.viewer(func(w http.ResponseWriter, r *http.Request, s Session) {
		id := r.PathValue("id")
		if _, e := a.playback.Channel(r.Context(), id); e != nil {
			a.playbackError(w, e)
			return
		}
		v, e := channels.New(a.db, a.cfg.Dir).ChannelArtwork(r.Context(), id, r.PathValue("kind"))
		if e != nil {
			a.channelError(w, e)
			return
		}
		if v.MediaID != "" {
			art, err := a.plex.Artwork(r.Context(), v.MediaID, v.Kind)
			if err == nil {
				v.Data = art.Data
				v.ContentType = art.ContentType
			}
		}
		w.Header().Set("Content-Type", v.ContentType)
		w.Write(v.Data)
	}))
	mux.HandleFunc("GET /api/v1/watch/channels/{id}/programs/{program}/artwork", a.viewer(func(w http.ResponseWriter, r *http.Request, s Session) {
		id := r.PathValue("id")
		if _, e := a.playback.Channel(r.Context(), id); e != nil {
			a.playbackError(w, e)
			return
		}
		var media string
		if e := a.db.QueryRowContext(r.Context(), "SELECT media_item_id FROM channel_programs WHERE id=? AND channel_id=?", r.PathValue("program"), id).Scan(&media); e != nil {
			problem(w, 404, "Artwork unavailable")
			return
		}
		kind := "poster"
		if r.URL.Query().Get("kind") == "background" {
			kind = "background"
		}
		v, e := a.plex.Artwork(r.Context(), media, kind)
		if e != nil {
			a.plexError(w, e)
			return
		}
		w.Header().Set("Content-Type", v.ContentType)
		w.Write(v.Data)
	}))
}
