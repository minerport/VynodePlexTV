// Package scheduler owns persisted linear timelines. It never contacts Plex.
package scheduler

import (
	"afterglow.local/tv/internal/interstitials"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"error"`
}

func (e *Error) Error() string { return e.Message }
func invalid(m string) error   { return &Error{"INVALID", m} }
func SafeError(e error) string {
	var v *Error
	if errors.As(e, &v) {
		return v.Message
	}
	return "Scheduling operation failed; existing timeline preserved"
}
func uid() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic("secure random unavailable")
	}
	return hex.EncodeToString(b)
}
func hash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type Settings struct {
	RecencyWeighted    bool   `json:"recencyWeighted"`
	AvoidSameShow      bool   `json:"avoidSameShow"`
	MinOtherShows      int    `json:"minOtherShows"`
	ChannelID          string `json:"channelId"`
	Mode               string `json:"mode"`
	Timezone           string `json:"timezone"`
	Anchor             int64  `json:"anchorMs"`
	Seed               string `json:"seed"`
	MovieDays          int    `json:"movieRepeatDays"`
	EpisodeDays        int    `json:"episodeRepeatDays"`
	PastDays           int    `json:"pastDays"`
	FutureDays         int    `json:"futureDays"`
	IncludeUnavailable bool   `json:"includeUnavailable"`
	Enabled            bool   `json:"enabled"`
}
type Item struct {
	ID       string `json:"id"`
	Duration int64  `json:"durationMs"`
	Type     string `json:"type"`
	Show     string `json:"showId"`
	Season   int    `json:"season"`
	AddedAt  int64  `json:"addedAt,omitempty"`
}
type Entry struct {
	Kind         string                    `json:"entryType"`
	AssetID      string                    `json:"assetId,omitempty"`
	FollowingID  string                    `json:"followingProgramId,omitempty"`
	Presentation []interstitials.Placement `json:"-"`
	ID           string                    `json:"id"`
	ChannelID    string                    `json:"channelId"`
	MediaID      string                    `json:"mediaId"`
	Start        int64                     `json:"start"`
	End          int64                     `json:"end"`
	Duration     int64                     `json:"durationMs"`
	Sequence     int64                     `json:"sequence"`
	Revision     int                       `json:"revision"`
	Reason       string                    `json:"policyReason"`
	Title        string                    `json:"title"`
	ShowTitle    string                    `json:"showTitle"`
	Season       *int                      `json:"season"`
	Episode      *int                      `json:"episode"`
	Available    bool                      `json:"available"`
	Artwork      string                    `json:"artwork"`
}
type State struct {
	Fingerprint, Config, Membership string
	Revision                        int
	Next                            int64
	Until                           *int64
	Status                          string
	Last                            *int64
	Error                           string
}
type Status struct {
	ChannelID   string   `json:"channelId"`
	Name        string   `json:"name"`
	Number      string   `json:"number"`
	Mode        string   `json:"mode"`
	Timezone    string   `json:"timezone"`
	Status      string   `json:"status"`
	Resolved    int      `json:"resolved"`
	Schedulable int      `json:"schedulable"`
	Excluded    int      `json:"excluded"`
	Start       *int64   `json:"coverageStart"`
	End         *int64   `json:"coverageEnd"`
	Count       int      `json:"programCount"`
	Last        *int64   `json:"lastGenerated"`
	Error       string   `json:"lastError"`
	Integrity   []string `json:"integrity"`
	Warnings    []string `json:"warnings"`
	Revision    int      `json:"revision"`
}
type Page struct {
	Items    []Entry `json:"items"`
	More     bool    `json:"more"`
	Start    int64   `json:"start"`
	End      int64   `json:"end"`
	Timezone string  `json:"timezone"`
}
type Result struct {
	Generated int    `json:"generated"`
	Repaired  int    `json:"repaired"`
	Start     int64  `json:"start"`
	End       int64  `json:"end"`
	Status    string `json:"status"`
}

const dayMS int64 = 86400000
const maxEntries = 100000

func DayRange(day, zone string) (time.Time, time.Time, error) {
	loc, e := time.LoadLocation(zone)
	if e != nil {
		return time.Time{}, time.Time{}, invalid("Unknown IANA timezone")
	}
	start, e := time.ParseInLocation("2006-01-02", day, loc)
	if e != nil {
		return start, start, invalid("Use a YYYY-MM-DD day")
	}
	return start.UTC(), start.AddDate(0, 0, 1).UTC(), nil
}
