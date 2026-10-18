package playback

import (
	"afterglow.local/tv/internal/interstitials"
	"afterglow.local/tv/internal/plex"
	"afterglow.local/tv/internal/scheduler"
	"context"
	"net/http"
	"os"
)

type metadataSource struct{}

func (metadataSource) Close(context.Context) error { return nil }
func (metadataSource) Fetch(context.Context, string, string) (*http.Response, error) {
	return nil, failure("NOT_FOUND", "This presentation contains no video")
}
func (s *Service) resolve(ctx context.Context, p *scheduler.Entry, caps plex.Capabilities, id string, fallback bool) (plex.PlaybackResolution, error) {
	if p.Kind != "INTERSTITIAL" {
		return s.plex.ResolvePlayback(ctx, p.MediaID, caps, id)
	}
	a, e := s.schedule.Presentation.Asset(ctx, p.AssetID)
	if e != nil {
		return plex.PlaybackResolution{}, e
	}
	var server string
	if e = s.db.QueryRowContext(ctx, "SELECT plex_server_id FROM plex_media_items WHERE id=?", p.MediaID).Scan(&server); e != nil {
		return plex.PlaybackResolution{}, e
	}
	metadata := plex.PlaybackResolution{Source: metadataSource{}, Duration: p.Duration, ServerID: server, Mode: "METADATA_PRESENTATION", Mime: "application/x-afterglow-presentation"}
	if fallback || a.Source == "GENERATED_METADATA" {
		return metadata, nil
	}
	if a.Source == "UPLOAD" {
		path, e := s.schedule.Presentation.Path(a.ID)
		if e != nil {
			return metadata, nil
		}
		if _, e = os.Stat(path); e != nil {
			s.log.Warn("Presentation unavailable; keeping scheduled station break", "asset", a.ID)
			_, e = s.db.ExecContext(ctx, "UPDATE interstitial_assets SET available=0 WHERE id=?", a.ID)
			if e != nil {
				return plex.PlaybackResolution{}, e
			}
			return metadata, nil
		}
		return plex.PlaybackResolution{Source: interstitials.LocalSource{Path: path}, Duration: p.Duration, ServerID: server, Mode: "LOCAL_INTERSTITIAL", Mime: "video/mp4"}, nil
	}
	var result plex.PlaybackResolution
	if a.Source == "PLEX_EXTRA" {
		var key string
		if e = s.db.QueryRowContext(ctx, "SELECT plex_extra_key FROM interstitial_assets WHERE id=?", a.ID).Scan(&key); e != nil {
			return metadata, nil
		}
		adapter, ok := s.plex.(interface {
			ResolveExtra(context.Context, string, string, plex.Capabilities, string) (plex.PlaybackResolution, error)
		})
		if !ok {
			return metadata, nil
		}
		result, e = adapter.ResolveExtra(ctx, a.MediaID, key, caps, id)
	} else {
		result, e = s.plex.ResolvePlayback(ctx, a.MediaID, caps, id)
	}
	if e != nil {
		s.log.Warn("Optional Plex presentation unavailable", "asset", a.ID)
		if plex.ErrorCode(e) == "NOT_FOUND" {
			if _, err := s.db.ExecContext(ctx, "UPDATE interstitial_assets SET available=0 WHERE id=?", a.ID); err != nil {
				return plex.PlaybackResolution{}, err
			}
		}
		return metadata, nil
	}
	return result, nil
}

// Advance swaps the current source while preserving the viewer session identity.
func (s *Service) Advance(ctx context.Context, user, auth, id string, fallback bool) (Tune, error) {
	v, e := s.owned(user, id)
	if e != nil {
		return Tune{}, e
	}
	v.transition.Lock()
	defer v.transition.Unlock()
	v.mu.Lock()
	r := TuneRequest{ChannelID: v.tune.Channel.ID, Mode: "LIVE", ClientID: v.client, Capabilities: v.caps, ReplaceSession: id, continuation: true, presentationFallback: fallback}
	r.Capabilities.VersionID = ""
	r.Capabilities.AudioStreamID = ""
	r.Capabilities.SubtitleStreamID = ""
	v.mu.Unlock()
	return s.Tune(ctx, user, auth, r)
}
