package playback

import (
	"afterglow.local/tv/internal/plex"
	"context"
)

func (s *Service) Options(ctx context.Context, user, id string) ([]plex.MediaVersion, error) {
	v, e := s.owned(user, id)
	if e != nil {
		return nil, e
	}
	v.mu.Lock()
	media, kind := v.tune.Program.MediaID, v.tune.Program.Kind
	v.mu.Unlock()
	if kind == "INTERSTITIAL" {
		return []plex.MediaVersion{}, nil
	}
	adapter, ok := s.plex.(interface {
		PlaybackOptions(context.Context, string) ([]plex.MediaVersion, error)
	})
	if !ok {
		return nil, failure("UNSUPPORTED", "Playback options unavailable")
	}
	return adapter.PlaybackOptions(ctx, media)
}

// Reconfigure retains the viewer identity and program, including behind-live/catch-up position.
func (s *Service) Reconfigure(ctx context.Context, user, auth, id string, caps plex.Capabilities, position int64) (Tune, error) {
	v, e := s.owned(user, id)
	if e != nil {
		return Tune{}, e
	}
	v.transition.Lock()
	defer v.transition.Unlock()
	v.mu.Lock()
	old := v.tune
	client := v.client
	v.mu.Unlock()
	if old.Program.Kind == "INTERSTITIAL" {
		return Tune{}, failure("INVALID", "Change playback options during a feature program")
	}
	old.Position = position
	return s.Tune(ctx, user, auth, TuneRequest{ChannelID: old.Channel.ID, ProgramID: old.Program.ID, Mode: "FROM_BEGINNING", ClientID: client, Capabilities: caps, ReplaceSession: id, continuation: true, resume: &old})
}
