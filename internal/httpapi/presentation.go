package httpapi

import (
	"afterglow.local/tv/internal/interstitials"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

func (a *API) presentationError(w http.ResponseWriter, e error) {
	var v *interstitials.Error
	if errors.As(e, &v) {
		problem(w, 422, v.Message)
		return
	}
	a.log.Error("Presentation request failed")
	problem(w, 500, "Presentation operation failed")
}
func (a *API) presentationRoutes(mux *http.ServeMux) {
	s := a.scheduler.Presentation
	audit := func(r *http.Request, u Session, action, target string) {
		if _, e := a.db.ExecContext(r.Context(), "INSERT INTO audit_log(user_id,action,created_at,target_id,summary) VALUES(?,?,unixepoch(),?,?)", u.User.ID, action, target, "Broadcast presentation updated"); e != nil {
			a.log.Error("Presentation audit write failed")
		}
	}
	mux.HandleFunc("POST /api/v1/interstitials/trailers", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var body struct {
			MediaID string `json:"mediaId"`
		}
		if !decodeChannel(w, r, &body) {
			return
		}
		if a.plex == nil {
			problem(w, 503, "Plex unavailable")
			return
		}
		extras, e := a.plex.Trailers(r.Context(), body.MediaID)
		if e != nil {
			a.plexError(w, e)
			return
		}
		count := 0
		for _, x := range extras {
			var exists bool
			if e = a.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM interstitial_assets WHERE media_id=? AND plex_extra_key=?)", body.MediaID, x.Key).Scan(&exists); e != nil {
				a.presentationError(w, e)
				return
			}
			if exists {
				if _, e = a.db.ExecContext(r.Context(), "UPDATE interstitial_assets SET available=1,duration_ms=?,updated_at=unixepoch() WHERE media_id=? AND plex_extra_key=?", x.Duration, body.MediaID, x.Key); e != nil {
					a.presentationError(w, e)
					return
				}
				continue
			}
			_, e = a.db.ExecContext(r.Context(), `INSERT INTO interstitial_assets(id,name,kind,source,media_id,duration_ms,created_by,created_at,updated_at,plex_extra_key) VALUES(?,?,'TRAILER','PLEX_EXTRA',?,?,?,unixepoch(),unixepoch(),?)`, interstitials.ID(), x.Title, body.MediaID, x.Duration, u.User.ID, x.Key)
			if e != nil {
				a.presentationError(w, e)
				return
			}
			count++
		}
		audit(r, u, "PLEX_TRAILERS_DISCOVERED", body.MediaID)
		reply(w, 200, map[string]int{"added": count, "available": len(extras)})
	}))

	mux.HandleFunc("GET /api/v1/interstitials/assets", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		v, e := s.Assets(r.Context())
		if e != nil {
			a.presentationError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("POST /api/v1/interstitials/assets", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var v interstitials.Asset
		if !decodeChannel(w, r, &v) {
			return
		}
		v.ID = ""
		v, e := s.SaveAsset(r.Context(), v, u.User.ID)
		if e != nil {
			a.presentationError(w, e)
			return
		}
		audit(r, u, "PRESENTATION_ASSET_CREATED", v.ID)
		reply(w, 201, v)
	}))
	mux.HandleFunc("PATCH /api/v1/interstitials/assets/{id}", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var v interstitials.Asset
		if !decodeChannel(w, r, &v) {
			return
		}
		v.ID = r.PathValue("id")
		v, e := s.SaveAsset(r.Context(), v, u.User.ID)
		if e != nil {
			a.presentationError(w, e)
			return
		}
		audit(r, u, "PRESENTATION_ASSET_UPDATED", v.ID)
		reply(w, 200, v)
	}))
	mux.HandleFunc("DELETE /api/v1/interstitials/assets/{id}", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		if e := s.DeleteAsset(r.Context(), r.PathValue("id")); e != nil {
			a.presentationError(w, e)
			return
		}
		audit(r, u, "PRESENTATION_ASSET_DELETED", r.PathValue("id"))
		reply(w, 200, map[string]bool{"deleted": true})
	}))
	mux.HandleFunc("POST /api/v1/interstitials/upload", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		maxMB := int64(256)
		if v, e := strconv.ParseInt(os.Getenv("PRESENTATION_UPLOAD_MB"), 10, 64); e == nil && v >= 1 && v <= 1024 {
			maxMB = v
		}
		controller := http.NewResponseController(w)
		controller.SetReadDeadline(time.Now().Add(5 * time.Minute))
		controller.SetWriteDeadline(time.Now().Add(6 * time.Minute))
		r.Body = http.MaxBytesReader(w, r.Body, maxMB*1024*1024+1048576)
		if e := r.ParseMultipartForm(8 << 20); e != nil {
			problem(w, 422, "Upload exceeded size limit or was interrupted")
			return
		}
		defer r.MultipartForm.RemoveAll()
		f, h, e := r.FormFile("file")
		if e != nil {
			problem(w, 422, "Choose an MP4 clip")
			return
		}
		defer f.Close()
		v, e := s.Upload(r.Context(), r.FormValue("name"), r.FormValue("kind"), h.Filename, u.User.ID, f, maxMB*1024*1024)
		if e != nil {
			a.presentationError(w, e)
			return
		}
		audit(r, u, "PRESENTATION_UPLOAD", v.ID)
		reply(w, 201, v)
	}))
	mux.HandleFunc("GET /api/v1/interstitials/assets/{id}/preview", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		v, e := s.Asset(r.Context(), r.PathValue("id"))
		if e != nil || v.Source != "UPLOAD" {
			problem(w, 404, "Local preview unavailable")
			return
		}
		p, e := s.Path(v.ID)
		if e != nil {
			a.presentationError(w, e)
			return
		}
		up, e := (interstitials.LocalSource{Path: p}).Fetch(r.Context(), "main", r.Header.Get("Range"))
		if e != nil {
			problem(w, 404, "Clip file unavailable")
			return
		}
		defer up.Body.Close()
		for k, vs := range up.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(up.StatusCode)
		io.Copy(w, up.Body)
	}))
	mux.HandleFunc("GET /api/v1/interstitials/rules", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		v, e := s.Rules(r.Context())
		if e != nil {
			a.presentationError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	for _, method := range []string{"POST", "PATCH"} {
		path := "/api/v1/interstitials/rules"
		if method == "PATCH" {
			path += "/{id}"
		}
		mux.HandleFunc(method+" "+path, a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
			var v interstitials.Rule
			if !decodeChannel(w, r, &v) {
				return
			}
			v.ID = r.PathValue("id")
			v, e := s.SaveRule(r.Context(), v)
			if e != nil {
				a.presentationError(w, e)
				return
			}
			audit(r, u, "PRESENTATION_RULE_UPDATED", v.ID)
			reply(w, 200, v)
		}))
	}
	mux.HandleFunc("DELETE /api/v1/interstitials/rules/{id}", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		if _, e := a.db.ExecContext(r.Context(), "DELETE FROM interstitial_rules WHERE id=?", r.PathValue("id")); e != nil {
			a.presentationError(w, e)
			return
		}
		audit(r, u, "PRESENTATION_RULE_DELETED", r.PathValue("id"))
		reply(w, 200, map[string]bool{"deleted": true})
	}))
	mux.HandleFunc("GET /api/v1/channels/{id}/presentation", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		v, e := s.Branding(r.Context(), r.PathValue("id"))
		if e != nil {
			a.presentationError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("PATCH /api/v1/channels/{id}/presentation", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var v interstitials.Branding
		if !decodeChannel(w, r, &v) {
			return
		}
		v.ChannelID = r.PathValue("id")
		if e := s.SaveBranding(r.Context(), v); e != nil {
			a.presentationError(w, e)
			return
		}
		audit(r, u, "CHANNEL_PRESENTATION_UPDATED", v.ChannelID)
		reply(w, 200, v)
	}))
	mux.HandleFunc("PUT /api/v1/channels/{id}/presentation/rules/{rule}", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var v struct {
			Enabled bool `json:"enabled"`
		}
		if !decodeChannel(w, r, &v) {
			return
		}
		_, e := a.db.ExecContext(r.Context(), `INSERT INTO channel_interstitial_rules VALUES(?,?,?) ON CONFLICT(channel_id,rule_id) DO UPDATE SET enabled=excluded.enabled`, r.PathValue("id"), r.PathValue("rule"), v.Enabled)
		if e != nil {
			a.presentationError(w, e)
			return
		}
		audit(r, u, "PRESENTATION_RULE_OVERRIDE", r.PathValue("id"))
		reply(w, 200, v)
	}))
}
