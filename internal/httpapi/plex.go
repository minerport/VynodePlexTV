package httpapi

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"

	"afterglow.local/tv/internal/playback"
	"afterglow.local/tv/internal/plex"
)

// MountPlex adds only admin inventory operations. The provider never reaches HTTP handlers.
func (a *API) MountPlex(service *plex.Service) {
	a.plex = service
	a.playback = playback.New(a.db, a.scheduler, service, a.log)
}
func (a *API) Playback() *playback.Service { return a.playback }
func (a *API) admin(next func(http.ResponseWriter, *http.Request, Session)) http.HandlerFunc {
	return a.auth(func(w http.ResponseWriter, r *http.Request, s Session) {
		if s.User.Role != "OWNER" && s.User.Role != "ADMIN" {
			problem(w, 403, "Administrator access required")
			return
		}
		if a.plex == nil {
			problem(w, 503, "Plex service is unavailable")
			return
		}
		next(w, r, s)
	})
}
func (a *API) plexError(w http.ResponseWriter, err error) {
	status := 502
	switch plex.ErrorCode(err) {
	case "INVALID":
		status = 400
	case "CONFLICT":
		status = 409
	case "NOT_FOUND":
		status = 404
	case "INTERNAL":
		status = 500
	}
	// Deliberately never log errors that could contain an upstream URL/credential.
	a.log.Warn("Plex request failed", "code", plex.ErrorCode(err))
	problem(w, status, plex.SafeMessage(err))
}
func (a *API) plexRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/plex/link", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		v, err := a.plex.StartLink(r.Context(), s.User.ID)
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 201, v)
	}))
	mux.HandleFunc("POST /api/v1/plex/link/{id}/poll", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		v, err := a.plex.PollLink(r.Context(), r.PathValue("id"), s.User.ID)
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("DELETE /api/v1/plex/link/{id}", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		if err := a.plex.CancelLink(r.PathValue("id"), s.User.ID); err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, map[string]string{"status": "cancelled"})
	}))
	mux.HandleFunc("POST /api/v1/plex/link/{id}/connect", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		var value struct {
			ServerID string `json:"serverId"`
			URI      string `json:"uri"`
		}
		if !decode(w, r, &value) {
			return
		}
		v, err := a.plex.ConnectLinkedServer(r.Context(), r.PathValue("id"), s.User.ID, value.ServerID, value.URI)
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 201, v)
	}))
	mux.HandleFunc("GET /api/v1/plex/servers", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		v, err := a.plex.Servers(r.Context())
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("POST /api/v1/plex/connections/test", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		var c plex.Connection
		if !decode(w, r, &c) {
			return
		}
		v, err := a.plex.TestConnection(r.Context(), c)
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("POST /api/v1/plex/servers", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		var c plex.Connection
		if !decode(w, r, &c) {
			return
		}
		v, err := a.plex.SaveServer(r.Context(), "", c, s.User.ID)
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 201, v)
	}))
	mux.HandleFunc("GET /api/v1/plex/servers/{id}", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		v, err := a.plex.Server(r.Context(), r.PathValue("id"))
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("PATCH /api/v1/plex/servers/{id}", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		var c plex.Connection
		if !decode(w, r, &c) {
			return
		}
		v, err := a.plex.SaveServer(r.Context(), r.PathValue("id"), c, s.User.ID)
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("DELETE /api/v1/plex/servers/{id}", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		if err := a.plex.RemoveServer(r.Context(), r.PathValue("id"), s.User.ID); err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, map[string]string{"status": "disconnected"})
	}))
	for path, kind := range map[string]string{"test": "PLEX_HEALTH_CHECK", "discover": "PLEX_DISCOVERY"} {
		jobType := kind
		mux.HandleFunc("POST /api/v1/plex/servers/{id}/"+path, a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
			v, err := a.plex.Enqueue(r.Context(), jobType, r.PathValue("id"), "", "", s.User.ID)
			if err != nil {
				a.plexError(w, err)
				return
			}
			reply(w, 202, v)
		}))
	}
	mux.HandleFunc("GET /api/v1/plex/servers/{id}/libraries", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		v, err := a.plex.Libraries(r.Context(), r.PathValue("id"))
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("PATCH /api/v1/plex/libraries/{id}", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		var value struct {
			Selected *bool `json:"selected"`
		}
		if !decode(w, r, &value) {
			return
		}
		if value.Selected == nil {
			problem(w, 400, "Selection is required")
			return
		}
		if err := a.plex.SelectLibrary(r.Context(), r.PathValue("id"), *value.Selected, s.User.ID); err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, map[string]string{"status": "saved"})
	}))
	mux.HandleFunc("POST /api/v1/plex/libraries/{id}/sync", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		var value struct {
			Mode string `json:"mode"`
		}
		if !decode(w, r, &value) {
			return
		}
		v, err := a.plex.Enqueue(r.Context(), "PLEX_LIBRARY_SYNC", "", r.PathValue("id"), value.Mode, s.User.ID)
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 202, v)
	}))
	mux.HandleFunc("GET /api/v1/jobs", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		v, err := a.plex.Jobs(r.Context())
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("GET /api/v1/jobs/{id}", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		v, err := a.plex.Job(r.Context(), r.PathValue("id"))
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("POST /api/v1/jobs/{id}/cancel", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		if err := a.plex.Cancel(r.Context(), r.PathValue("id"), s.User.ID); err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, map[string]string{"status": "cancellation_requested"})
	}))
	mux.HandleFunc("GET /api/v1/media", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		q := r.URL.Query()
		offset, _ := strconv.Atoi(q.Get("offset"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		v, err := a.plex.Media(r.Context(), plex.MediaQuery{ServerID: q.Get("server"), LibraryID: q.Get("library"), Type: q.Get("type"), Title: q.Get("q"), Availability: q.Get("available"), ParentKey: q.Get("parent"), TopLevel: q.Get("topLevel") == "true", Offset: offset, Limit: limit})
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("GET /api/v1/media/{id}", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		v, err := a.plex.Item(r.Context(), r.PathValue("id"))
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("GET /api/v1/media/{id}/artwork/{kind}", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		v, err := a.plex.Artwork(r.Context(), r.PathValue("id"), r.PathValue("kind"))
		if err != nil {
			a.plexError(w, err)
			return
		}
		etag := fmt.Sprintf(`"%x"`, sha256.Sum256(v.Data))
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "private, max-age=300")
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(304)
			return
		}
		w.Header().Set("Content-Type", v.ContentType)
		w.WriteHeader(200)
		_, _ = w.Write(v.Data)
	}))
	mux.HandleFunc("GET /api/v1/plex/settings", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		v, err := a.plex.Settings(r.Context())
		if err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("PUT /api/v1/plex/settings", a.admin(func(w http.ResponseWriter, r *http.Request, s Session) {
		var v plex.Options
		if !decode(w, r, &v) {
			return
		}
		if err := a.plex.SaveSettings(r.Context(), v, s.User.ID); err != nil {
			a.plexError(w, err)
			return
		}
		reply(w, 200, v)
	}))
}
