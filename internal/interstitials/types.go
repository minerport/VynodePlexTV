// Package interstitials owns presentation assets and deterministic placement policies.
package interstitials

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"time"
)

type Asset struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Source      string   `json:"source"`
	MediaID     string   `json:"mediaId"`
	Duration    int64    `json:"durationMs"`
	Enabled     bool     `json:"enabled"`
	Available   bool     `json:"available"`
	Tags        []string `json:"tags"`
	ActiveFrom  string   `json:"activeFrom"`
	ActiveUntil string   `json:"activeUntil"`
	Container   string   `json:"container"`
	VideoCodec  string   `json:"videoCodec"`
	AudioCodec  string   `json:"audioCodec"`
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	Size        int64    `json:"sizeBytes"`
}
type Rule struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ChannelID     string `json:"channelId"`
	Enabled       bool   `json:"enabled"`
	Kind          string `json:"kind"`
	Tag           string `json:"tag"`
	Placement     string `json:"placement"`
	MediaType     string `json:"mediaType"`
	Every         int    `json:"every"`
	RepeatMinutes int    `json:"repeatMinutes"`
	MaxDuration   int64  `json:"maxDurationMs"`
	Priority      int    `json:"priority"`
}
type Branding struct {
	ChannelID     string  `json:"channelId"`
	Enabled       bool    `json:"enabled"`
	Inherit       bool    `json:"inherit"`
	Bug           bool    `json:"bug"`
	Corner        string  `json:"corner"`
	Opacity       float64 `json:"opacity"`
	Size          int     `json:"size"`
	CompactLogoID string  `json:"compactLogoId"`
	NextUp        int     `json:"nextUpSeconds"`
}

func DefaultBranding(id string) Branding {
	return Branding{ChannelID: id, Inherit: true, Corner: "top-right", Opacity: .35, Size: 72, NextUp: 45}
}

type Snapshot struct {
	Rules    []Rule
	Assets   []Asset
	Branding Branding
}
type Placement struct {
	AssetID    string
	RuleID     string
	Start, End int64
	Reason     string
}

func Fingerprint(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Select fills only real asset time. No synthetic black-screen padding or clipping.
// HOURLY means the first safe program boundary after a clock-hour crossing, not a hard clock.
func (s Snapshot) Select(seed, mediaType, mediaID string, seq, at, previousStart int64, zone string, history map[string]int64) []Placement {
	out := []Placement{}
	if !s.Branding.Enabled {
		return out
	}
	loc, e := time.LoadLocation(zone)
	if e != nil {
		loc = time.UTC
	}
	breakStart := at
	for _, r := range s.Rules {
		if !r.Enabled || r.Every < 1 || seq%int64(r.Every) != 0 || r.MediaType != "" && r.MediaType != mediaType || r.Placement == "BETWEEN" && seq == 0 {
			continue
		}
		if r.Placement == "HOURLY" && previousStart > 0 && time.UnixMilli(previousStart).In(loc).Format("2006-01-02T15 -0700") == time.UnixMilli(at).In(loc).Format("2006-01-02T15 -0700") {
			continue
		}
		day := time.UnixMilli(at).In(loc).Format("2006-01-02")
		pool := []Asset{}
		for _, a := range s.Assets {
			if !a.Enabled || !a.Available || a.Kind != r.Kind || a.Duration <= 0 || a.Duration > r.MaxDuration || at+a.Duration-breakStart > 600000 || a.ActiveFrom != "" && day < a.ActiveFrom || a.ActiveUntil != "" && day > a.ActiveUntil {
				continue
			}
			if r.Tag != "" {
				found := false
				for _, tag := range a.Tags {
					if tag == r.Tag {
						found = true
					}
				}
				if !found {
					continue
				}
			}
			pool = append(pool, a)
		}
		sort.Slice(pool, func(i, j int) bool {
			a, b := pool[i], pool[j]
			la, sa := history[a.ID]
			lb, sb := history[b.ID]
			ta, tb := 0, 0
			if sa && at-la < int64(r.RepeatMinutes)*60000 {
				ta = 1
			}
			if sb && at-lb < int64(r.RepeatMinutes)*60000 {
				tb = 1
			}
			if ta != tb {
				return ta < tb
			}
			if r.Kind == "TRAILER" {
				ar, br := a.MediaID == mediaID, b.MediaID == mediaID
				if ar != br {
					return ar
				}
			}
			if sa != sb {
				return !sa
			}
			if la != lb {
				return la < lb
			}
			return Fingerprint([]string{seed, strconv.FormatInt(seq, 10), r.ID, a.ID}) < Fingerprint([]string{seed, strconv.FormatInt(seq, 10), r.ID, b.ID})
		})
		if len(pool) == 0 {
			continue
		}
		a := pool[0]
		reason := "eligible presentation"
		if last, ok := history[a.ID]; ok && at-last < int64(r.RepeatMinutes)*60000 {
			reason = "repeat window relaxed; oldest asset"
		}
		out = append(out, Placement{a.ID, r.ID, at, at + a.Duration, reason})
		history[a.ID] = at
		at += a.Duration
	}
	return out
}
