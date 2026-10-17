package httpapi

import (
	"afterglow.local/tv/internal/guide"
	"errors"
	"net/http"
	"strconv"
	"time"
)

func (a *API) guideRoutes(mux *http.ServeMux) {
	s := guide.New(a.db)
	handleError := func(w http.ResponseWriter, e error) {
		var v *guide.Error
		if errors.As(e, &v) {
			code := 400
			if v.Code == "NOT_FOUND" {
				code = 404
			}
			problem(w, code, v.Message)
		} else {
			a.fail(w, e)
		}
	}
	for _, kind := range []string{"window", "now", "soon"} {
		path := "/api/v1/guide"
		if kind == "now" {
			path += "/on-now"
		}
		if kind == "soon" {
			path += "/starting-soon"
		}
		mux.HandleFunc("GET "+path, a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
			q := r.URL.Query()
			start, end := int64(0), int64(0)
			if kind == "window" {
				x, e := time.Parse(time.RFC3339, q.Get("start"))
				y, err := time.Parse(time.RFC3339, q.Get("end"))
				if e != nil || err != nil {
					problem(w, 400, "Use UTC or offset ISO timestamps for start and end")
					return
				}
				start = x.UnixMilli()
				end = y.UnixMilli()
			}
			offset, _ := strconv.Atoi(q.Get("offset"))
			limit, _ := strconv.Atoi(q.Get("limit"))
			v, e := s.Window(r.Context(), u.User.ID, guide.Query{Start: start, End: end, Group: q.Get("group"), Favorites: q.Get("favorites") == "true", Offset: offset, Limit: limit, Kind: kind})
			if e != nil {
				handleError(w, e)
				return
			}
			reply(w, 200, v)
		}))
	}
	mux.HandleFunc("GET /api/v1/guide/programs/{id}", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
		v, e := s.Detail(r.Context(), u.User.ID, r.PathValue("id"))
		if e != nil {
			handleError(w, e)
			return
		}
		reply(w, 200, v)
	}))
	mux.HandleFunc("GET /api/v1/users/me/favorites", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
		v, e := s.Favorites(r.Context(), u.User.ID)
		if e != nil {
			handleError(w, e)
			return
		}
		reply(w, 200, map[string]any{"channelIds": v})
	}))
	for _, method := range []string{"POST", "DELETE"} {
		mux.HandleFunc(method+" /api/v1/users/me/favorites/{id}", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
			if e := s.Favorite(r.Context(), u.User.ID, r.PathValue("id"), r.Method == "POST"); e != nil {
				handleError(w, e)
				return
			}
			reply(w, 200, map[string]bool{"favorite": r.Method == "POST"})
		}))
	}
	mux.HandleFunc("GET /api/v1/users/me/last-channel", a.auth(func(w http.ResponseWriter, r *http.Request, u Session) {
		id, e := s.Last(r.Context(), u.User.ID)
		if e != nil {
			handleError(w, e)
			return
		}
		reply(w, 200, map[string]string{"channelId": id})
	}))
}
