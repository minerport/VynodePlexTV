package plex

import "context"

func (s *Service) PlaybackOptions(ctx context.Context, id string) ([]MediaVersion, error) {
	item, e := s.Item(ctx, id)
	if e != nil {
		return nil, e
	}
	p, _, e := s.provider(ctx, item.ServerID)
	if e != nil {
		return nil, e
	}
	live, e := p.Metadata(ctx, item.RatingKey)
	if e != nil {
		return nil, e
	}
	if live.Type != item.Type || item.GUID != "" && live.GUID != "" && item.GUID != live.GUID {
		return nil, failure("NOT_FOUND", "Program identity changed")
	}
	out := []MediaVersion{}
	for _, v := range live.Versions {
		if v.PartCount == 1 {
			out = append(out, v)
		}
	}
	return out, nil
}
