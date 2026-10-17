// Package channels resolves channel membership from local application inventory only.
// It has no Plex transport, clock-based scheduling, or playback dependency.
package channels

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type Issue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}
type Error struct {
	Code    string  `json:"code"`
	Message string  `json:"error"`
	Issues  []Issue `json:"issues,omitempty"`
}

func (e *Error) Error() string           { return e.Message }
func invalid(path, message string) error { return &Error{"INVALID", message, []Issue{{path, message}}} }
func fail(code, message string) error    { return &Error{Code: code, Message: message} }
func newID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic("secure random unavailable")
	}
	return hex.EncodeToString(b)
}
func stamp() int64                { return time.Now().UTC().Unix() }
func IsPlayable(kind string) bool { return kind == "movie" || kind == "episode" }

const playableSQL = "m.media_type IN ('movie','episode')"
const eligibleSQL = "m.available=1 AND l.selected=1 AND l.present=1 AND s.disabled=0"

type Rule struct {
	Version int  `json:"version"`
	Root    Node `json:"root"`
}
type Node struct {
	Type     string          `json:"type"`
	Operator string          `json:"operator"`
	Field    string          `json:"field,omitempty"`
	Value    json.RawMessage `json:"value,omitempty"`
	Children []Node          `json:"children,omitempty"`
}
type Source struct {
	Type       string            `json:"type"`
	Static     *StaticConfig     `json:"static,omitempty"`
	Show       *ShowConfig       `json:"show,omitempty"`
	Collection *CollectionConfig `json:"collection,omitempty"`
	Rule       *Rule             `json:"rule,omitempty"`
}
type StaticConfig struct {
	MediaIDs []string `json:"mediaIds"`
}
type ShowConfig struct {
	ShowID             string   `json:"showId"`
	EpisodeScope       string   `json:"episodeScope"`
	SeasonIDs          []string `json:"seasonIds"`
	IncludeSpecials    bool     `json:"includeSpecials"`
	ExcludeUnavailable *bool    `json:"excludeUnavailable,omitempty"`
}
type CollectionConfig struct {
	CollectionID string `json:"collectionId"`
}
type Appearance struct {
	Source   string  `json:"source"`
	UploadID *string `json:"uploadId"`
}
type Definition struct {
	Number         string     `json:"number"`
	Name           string     `json:"name"`
	ShortName      string     `json:"shortName"`
	Description    string     `json:"description"`
	GroupID        *string    `json:"groupId"`
	Enabled        bool       `json:"enabled"`
	Visibility     string     `json:"visibility"`
	SortOrder      int        `json:"sortOrder"`
	Logo           Appearance `json:"logo"`
	Background     Appearance `json:"background"`
	ArtworkMediaID *string    `json:"artworkMediaId"`
	Source         Source     `json:"source"`
}
type Channel struct {
	Definition
	ID        string   `json:"id"`
	CreatedAt int64    `json:"createdAt"`
	UpdatedAt int64    `json:"updatedAt"`
	Count     int      `json:"count"`
	Warnings  []string `json:"warnings"`
}
type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	SortOrder   int    `json:"sortOrder"`
	System      bool   `json:"system"`
	Count       int    `json:"count"`
}
type Media struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Year       *int   `json:"year"`
	ShowTitle  string `json:"showTitle"`
	Season     *int   `json:"season"`
	Episode    *int   `json:"episode"`
	DurationMS *int64 `json:"durationMs"`
	Library    string `json:"library"`
	Available  bool   `json:"available"`
	Eligible   bool   `json:"eligible"`
}
type PreviewRequest struct {
	Source             Source `json:"source"`
	IncludeUnavailable bool   `json:"includeUnavailable"`
	Offset             int    `json:"offset"`
	Limit              int    `json:"limit"`
}
type Preview struct {
	Count    int      `json:"count"`
	Items    []Media  `json:"items"`
	Offset   int      `json:"offset"`
	Limit    int      `json:"limit"`
	Warnings []string `json:"warnings"`
}
type Option struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Context   string `json:"context"`
	Count     int    `json:"count"`
	ArtworkID string `json:"artworkId"`
	Seasons   int    `json:"seasons"`
	Episodes  int    `json:"episodes"`
}
type Options struct {
	Items []Option `json:"items"`
	More  bool     `json:"more"`
}

func placeholders(n int) string {
	out := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			out += ","
		}
		out += "?"
	}
	return out
}
func pathChild(path string, i int) string { return fmt.Sprintf("%s.children.%d", path, i) }
