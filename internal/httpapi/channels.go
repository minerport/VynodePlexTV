package httpapi

import (
	"afterglow.local/tv/internal/channels"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func (a *API) channelAdmin(next func(http.ResponseWriter, *http.Request, Session)) http.HandlerFunc {
	return a.auth(func(w http.ResponseWriter, r *http.Request, s Session) {
		if s.User.Role != "OWNER" && s.User.Role != "ADMIN" {
			problem(w, 403, "Administrator access required")
			return
		}
		next(w, r, s)
	})
}
func (a *API) channelError(w http.ResponseWriter, err error) {
	var e *channels.Error
	if errors.As(err, &e) {
		status := 422
		if e.Code == "NOT_FOUND" {
			status = 404
		}
		if e.Code == "CONFLICT" {
			status = 409
		}
		reply(w, status, e)
		return
	}
	a.log.Error("Channel operation failed")
	problem(w, 500, "Channel operation could not be completed")
}
func decodeChannel(w http.ResponseWriter, r *http.Request, target any) bool {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		problem(w, 415, "Content-Type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(target) != nil {
		problem(w, 400, "Invalid channel request body")
		return false
	}
	if d.Decode(&struct{}{}) != io.EOF {
		problem(w, 400, "Expected one JSON object")
		return false
	}
	return true
}
func pageParams(r *http.Request) (int, int) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return offset, limit
}
func (a *API) channelRoutes(mux *http.ServeMux) {
	s := channels.New(a.db, a.cfg.Dir)
	mux.HandleFunc("GET /api/v1/channel-metadata/selected", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		v, e := s.Lookup(r.Context(), r.URL.Query().Get("field"), r.URL.Query().Get("value"))
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("GET /api/v1/channels", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		offset, limit := pageParams(r)
		if limit < 1 || limit > 100 {
			limit = 50
		}
		items, e := s.ListGroup(r.Context(), offset, limit+1, r.URL.Query().Get("group"))
		if e != nil {
			a.channelError(w, e)
			return
		}
		more := len(items) > limit
		if more {
			items = items[:limit]
		}
		next, e := s.NextNumber(r.Context())
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, map[string]any{"items": items, "more": more, "nextNumber": next})
	}))
	mux.HandleFunc("POST /api/v1/channels", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var d channels.Definition
		if !decodeChannel(w, r, &d) {
			return
		}
		c, e := s.Save(r.Context(), "", d, u.User.ID)
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 201, c)
	}))
	mux.HandleFunc("GET /api/v1/channels/{id}", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		c, e := s.Get(r.Context(), r.PathValue("id"))
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, c)
	}))
	mux.HandleFunc("PATCH /api/v1/channels/{id}", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var d channels.Definition
		if !decodeChannel(w, r, &d) {
			return
		}
		c, e := s.Save(r.Context(), r.PathValue("id"), d, u.User.ID)
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, c)
	}))
	mux.HandleFunc("DELETE /api/v1/channels/{id}", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		if e := s.Delete(r.Context(), r.PathValue("id"), u.User.ID); e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, map[string]bool{"deleted": true})
	}))
	mux.HandleFunc("PATCH /api/v1/channels/{id}/enabled", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		if !decodeChannel(w, r, &body) {
			return
		}
		if body.Enabled == nil {
			problem(w, 400, "Enabled state required")
			return
		}
		if e := s.Enable(r.Context(), r.PathValue("id"), *body.Enabled, u.User.ID); e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	}))
	mux.HandleFunc("POST /api/v1/channels/{id}/duplicate", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		c, e := s.Duplicate(r.Context(), r.PathValue("id"), u.User.ID)
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 201, c)
	}))
	mux.HandleFunc("GET /api/v1/channels/{id}/media", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		offset, limit := pageParams(r)
		v, e := s.ResolveChannelMedia(r.Context(), r.PathValue("id"), r.URL.Query().Get("includeUnavailable") == "true", offset, limit)
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("GET /api/v1/channels/{id}/media/count", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		v, e := s.CountResolvedMedia(r.Context(), r.PathValue("id"))
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, map[string]int{"count": v})
	}))
	mux.HandleFunc("POST /api/v1/channel-preview", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		var body channels.PreviewRequest
		if !decodeChannel(w, r, &body) {
			return
		}
		v, e := s.Preview(r.Context(), body)
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("POST /api/v1/channel-rules/preview", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		var body struct {
			Rule               channels.Rule `json:"rule"`
			IncludeUnavailable bool          `json:"includeUnavailable"`
			Offset             int           `json:"offset"`
			Limit              int           `json:"limit"`
		}
		if !decodeChannel(w, r, &body) {
			return
		}
		v, e := s.Preview(r.Context(), channels.PreviewRequest{Source: channels.Source{Type: "RULE", Rule: &body.Rule}, IncludeUnavailable: body.IncludeUnavailable, Offset: body.Offset, Limit: body.Limit})
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("POST /api/v1/channel-rules/validate", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		var body channels.Rule
		if !decodeChannel(w, r, &body) {
			return
		}
		if _, e := (channels.RuleCompiler{DB: a.db}).Compile(r.Context(), body); e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, map[string]bool{"valid": true})
	}))
	mux.HandleFunc("GET /api/v1/channel-rules/fields", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) { reply(w, 200, channels.Fields()) }))
	mux.HandleFunc("GET /api/v1/channel-metadata/{field}", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		offset, limit := pageParams(r)
		v, e := s.Metadata(r.Context(), r.PathValue("field"), r.URL.Query().Get("q"), r.URL.Query().Get("parent"), offset, limit)
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("GET /api/v1/channel-groups", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		v, e := s.Groups(r.Context())
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("POST /api/v1/channel-groups", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var body channels.Group
		if !decodeChannel(w, r, &body) {
			return
		}
		v, e := s.SaveGroup(r.Context(), "", body, u.User.ID)
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 201, v)
	}))
	mux.HandleFunc("PATCH /api/v1/channel-groups/{id}", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var body channels.Group
		if !decodeChannel(w, r, &body) {
			return
		}
		v, e := s.SaveGroup(r.Context(), r.PathValue("id"), body, u.User.ID)
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("DELETE /api/v1/channel-groups/{id}", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		var body struct {
			MoveTo  *string `json:"moveTo"`
			Ungroup bool    `json:"ungroup"`
		}
		if !decodeChannel(w, r, &body) {
			return
		}
		if e := s.DeleteGroup(r.Context(), r.PathValue("id"), body.MoveTo, body.Ungroup, u.User.ID); e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 200, map[string]bool{"deleted": true})
	}))
	mux.HandleFunc("POST /api/v1/channel-artwork", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, u Session) {
		r.Body = http.MaxBytesReader(w, r.Body, channels.MaxUpload)
		data, e := io.ReadAll(r.Body)
		if e != nil {
			problem(w, 413, "Image must be no larger than 8 MB")
			return
		}
		v, e := s.Upload(r.Context(), data, u.User.ID)
		if e != nil {
			a.channelError(w, e)
			return
		}
		reply(w, 201, v)
	}))
	imageReply := func(w http.ResponseWriter, v channels.Artwork) {
		w.Header().Set("Content-Type", v.ContentType)
		w.Header().Set("Cache-Control", "private, max-age=60")
		w.WriteHeader(200)
		_, _ = w.Write(v.Data)
	}
	mux.HandleFunc("GET /api/v1/channel-artwork/{id}", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		v, e := s.UploadArtwork(r.Context(), r.PathValue("id"))
		if e != nil {
			a.channelError(w, e)
			return
		}
		imageReply(w, v)
	}))
	mux.HandleFunc("GET /api/v1/channels/{id}/artwork/{kind}", a.channelAdmin(func(w http.ResponseWriter, r *http.Request, _ Session) {
		v, e := s.ChannelArtwork(r.Context(), r.PathValue("id"), r.PathValue("kind"))
		if e != nil {
			a.channelError(w, e)
			return
		}
		if v.MediaID != "" && a.plex != nil {
			art, e := a.plex.Artwork(r.Context(), v.MediaID, v.Kind)
			if e == nil {
				v.Data = art.Data
				v.ContentType = art.ContentType
			}
		}
		imageReply(w, v)
	}))
}
