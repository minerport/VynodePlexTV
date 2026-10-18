package plex

import (
	"context"
	"net/url"
)

type Extra struct {
	Key      string `json:"key"`
	Title    string `json:"title"`
	Duration int64  `json:"durationMs"`
}

// Extras are discovered only on explicit administrator request. Remote web scraping is never used.
func (s *Service) Trailers(ctx context.Context, id string) ([]Extra, error) {
	m, e := s.Item(ctx, id)
	if e != nil {
		return nil, e
	}
	p, _, e := s.provider(ctx, m.ServerID)
	if e != nil {
		return nil, e
	}
	c, ok := p.(*Client)
	if !ok {
		return nil, failure("UNSUPPORTED", "Plex extras provider unavailable")
	}
	root, e := c.container(ctx, "/library/metadata/"+url.PathEscape(m.RatingKey), url.Values{"includeExtras": {"1"}})
	if e != nil {
		return nil, e
	}
	out := []Extra{}
	for _, movie := range root.Videos {
		for _, x := range movie.Extras.Videos {
			if x.ExtraType == 1 && x.RatingKey != "" && x.Duration != nil && *x.Duration > 0 && *x.Duration <= 600000 {
				out = append(out, Extra{x.RatingKey, x.Title, *x.Duration})
			}
		}
	}
	return out, nil
}
func (s *Service) ResolveExtra(ctx context.Context, parent, key string, caps Capabilities, session string) (PlaybackResolution, error) {
	m, e := s.Item(ctx, parent)
	if e != nil {
		return PlaybackResolution{}, e
	}
	if !m.Available {
		return PlaybackResolution{}, failure("NOT_FOUND", "Trailer parent is unavailable")
	}
	extras, e := s.Trailers(ctx, parent)
	if e != nil {
		return PlaybackResolution{}, e
	}
	found := false
	for _, x := range extras {
		if x.Key == key {
			found = true
		}
	}
	if !found {
		return PlaybackResolution{}, failure("NOT_FOUND", "Plex trailer is no longer available")
	}
	m.RatingKey = key
	m.GUID = ""
	m.Type = "clip"
	return s.resolveMedia(ctx, m, caps, session)
}
