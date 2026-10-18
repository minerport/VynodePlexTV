// Package playback orchestrates persisted programs and client sessions. Plex protocol stays in its adapter.
package playback

import (
	"afterglow.local/tv/internal/interstitials"
	"afterglow.local/tv/internal/plex"
	"afterglow.local/tv/internal/scheduler"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
)

type Adapter interface {
	ResolvePlayback(context.Context, string, plex.Capabilities, string) (plex.PlaybackResolution, error)
	StopPlayback(context.Context, string, string, string) error
}
type Error struct {
	Code    string `json:"code"`
	Message string `json:"error"`
}

func (e *Error) Error() string           { return e.Message }
func failure(code, message string) error { return &Error{code, message} }
func SafeError(e error) string {
	var p *Error
	if errors.As(e, &p) {
		return p.Message
	}
	return plex.SafeMessage(e)
}
func uid() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic("secure random unavailable")
	}
	return hex.EncodeToString(b)
}

type Channel struct {
	Timezone   string `json:"timezone"`
	ID         string `json:"id"`
	Number     string `json:"number"`
	Name       string `json:"name"`
	Logo       string `json:"logo"`
	RecentKind string `json:"recentKind"`
}
type OnNow struct {
	Channel  Channel          `json:"channel"`
	Program  *scheduler.Entry `json:"program"`
	Next     *scheduler.Entry `json:"next"`
	Progress float64          `json:"progress"`
}
type TuneRequest struct {
	resume               *Tune
	continuation         bool
	presentationFallback bool
	ProgramID            string            `json:"programId,omitempty"`
	ChannelID            string            `json:"channelId"`
	Mode                 string            `json:"mode"`
	ClientID             string            `json:"clientId"`
	Capabilities         plex.Capabilities `json:"capabilities"`
	ReplaceSession       string            `json:"replaceSession,omitempty"`
}
type Tune struct {
	Version    *plex.MediaVersion     `json:"version,omitempty"`
	Branding   interstitials.Branding `json:"branding"`
	Origin     string                 `json:"origin"`
	ID         string                 `json:"id"`
	Channel    Channel                `json:"channel"`
	Program    scheduler.Entry        `json:"program"`
	Next       *scheduler.Entry       `json:"next"`
	Mode       string                 `json:"mode"`
	State      string                 `json:"state"`
	ServerTime int64                  `json:"serverTime"`
	LiveOffset int64                  `json:"liveOffsetMs"`
	Position   int64                  `json:"positionMs"`
	Duration   int64                  `json:"durationMs"`
	StreamURL  string                 `json:"streamUrl"`
	Mime       string                 `json:"mime"`
	Delivery   string                 `json:"delivery"`
	AddedAt    *int64                 `json:"addedAt"`
}
type Heartbeat struct {
	Position int64  `json:"positionMs"`
	State    string `json:"state"`
	Ended    bool   `json:"ended"`
}
type Update struct {
	ServerTime int64  `json:"serverTime"`
	Mode       string `json:"mode"`
	LiveOffset int64  `json:"liveOffsetMs"`
	Transition bool   `json:"transition"`
	Ended      bool   `json:"ended"`
	Message    string `json:"message,omitempty"`
}
type Active struct {
	ID       string `json:"id"`
	User     string `json:"user"`
	Channel  string `json:"channel"`
	Program  string `json:"program"`
	Client   string `json:"client"`
	Mode     string `json:"mode"`
	State    string `json:"state"`
	Delivery string `json:"delivery"`
	Started  int64  `json:"started"`
	LastSeen int64  `json:"lastSeen"`
}

func Offset(now, start, duration int64) int64 {
	if duration <= 0 {
		return 0
	}
	v := now - start
	if v < 0 {
		return 0
	}
	if v >= duration {
		return duration - 1
	}
	return v
}
