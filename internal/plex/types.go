// Package plex owns all Plex protocol behavior and maps it into application inventory models.
package plex

import "context"

type Identity struct {
	MachineIdentifier string `json:"machineIdentifier"`
	FriendlyName      string `json:"friendlyName"`
	Product           string `json:"product"`
	Version           string `json:"version"`
	Platform          string `json:"platform"`
	PlatformVersion   string `json:"platformVersion"`
	Owned             *bool  `json:"owned"`
	SecureConnections *bool  `json:"secureConnections"`
}
type Server struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"baseUrl"`
	Identity
	Status          string `json:"status"`
	LastHealthCheck *int64 `json:"lastHealthCheckAt"`
	LastSuccess     *int64 `json:"lastSuccessAt"`
	LastError       string `json:"lastError"`
	Disabled        bool   `json:"disabled"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
}
type Connection struct {
	BaseURL string `json:"baseUrl"`
	Token   string `json:"token"`
	Name    string `json:"name"`
}
type Library struct {
	ID           string `json:"id"`
	ServerID     string `json:"serverId"`
	Key          string `json:"key"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	Agent        string `json:"agent"`
	Scanner      string `json:"scanner"`
	Language     string `json:"language"`
	UUID         string `json:"uuid"`
	Revision     string `json:"revision"`
	Selected     bool   `json:"selected"`
	Present      bool   `json:"present"`
	Supported    bool   `json:"supported"`
	LastSynced   *int64 `json:"lastSyncedAt"`
	LastFullSync *int64 `json:"lastFullSyncAt"`
	ItemCount    int    `json:"itemCount"`
}
type Tag struct {
	Kind         string `json:"kind"`
	SourceKey    string `json:"sourceKey"`
	Name         string `json:"name"`
	Relationship string `json:"relationship"`
	Character    string `json:"character"`
}
type ExternalID struct {
	Provider string `json:"provider"`
	Value    string `json:"value"`
}
type Stream struct {
	ID              string `json:"id"`
	Type            *int   `json:"type"`
	Codec           string `json:"codec"`
	Language        string `json:"language"`
	LanguageCode    string `json:"languageCode"`
	Title           string `json:"title"`
	Default         *bool  `json:"default"`
	Forced          *bool  `json:"forced"`
	HearingImpaired *bool  `json:"hearingImpaired"`
	Channels        *int   `json:"channels"`
	Width           *int   `json:"width"`
	Height          *int   `json:"height"`
	Profile         string `json:"profile"`
}
type MediaVersion struct {
	PartKey    string   `json:"-"`
	MediaIndex int      `json:"-"`
	PartIndex  int      `json:"-"`
	PartCount  int      `json:"-"`
	MediaID    string   `json:"mediaId"`
	PartID     string   `json:"partId"`
	Container  string   `json:"container"`
	VideoCodec string   `json:"videoCodec"`
	AudioCodec string   `json:"audioCodec"`
	Resolution string   `json:"resolution"`
	Width      *int     `json:"width"`
	Height     *int     `json:"height"`
	Bitrate    *int     `json:"bitrate"`
	DurationMS *int64   `json:"durationMs"`
	File       string   `json:"-"`
	Size       *int64   `json:"size"`
	Optimized  *bool    `json:"optimized"`
	Streams    []Stream `json:"streams"`
}
type MediaItem struct {
	ID                    string         `json:"id"`
	ServerID              string         `json:"serverId"`
	LibraryID             string         `json:"libraryId"`
	LibraryTitle          string         `json:"libraryTitle"`
	RatingKey             string         `json:"ratingKey"`
	ParentKey             string         `json:"parentRatingKey"`
	GrandparentKey        string         `json:"grandparentRatingKey"`
	Type                  string         `json:"type"`
	Title                 string         `json:"title"`
	SortTitle             string         `json:"sortTitle"`
	OriginalTitle         string         `json:"originalTitle"`
	Summary               string         `json:"summary"`
	Year                  *int           `json:"year"`
	ContentRating         string         `json:"contentRating"`
	Rating                *float64       `json:"rating"`
	AudienceRating        *float64       `json:"audienceRating"`
	DurationMS            *int64         `json:"durationMs"`
	AddedAt               *int64         `json:"addedAt"`
	SourceUpdatedAt       *int64         `json:"sourceUpdatedAt"`
	OriginallyAvailableAt string         `json:"originallyAvailableAt"`
	Studio                string         `json:"studio"`
	Tagline               string         `json:"tagline"`
	Index                 *int           `json:"index"`
	ParentIndex           *int           `json:"parentIndex"`
	GUID                  string         `json:"guid"`
	Thumb                 string         `json:"-"`
	Art                   string         `json:"-"`
	ParentThumb           string         `json:"-"`
	GrandparentThumb      string         `json:"-"`
	Available             bool           `json:"available"`
	UpdatedAt             int64          `json:"updatedAt"`
	Tags                  []Tag          `json:"tags,omitempty"`
	ExternalIDs           []ExternalID   `json:"externalIds,omitempty"`
	Versions              []MediaVersion `json:"versions,omitempty"`
}
type Page struct {
	Items  []MediaItem
	Total  *int
	Offset int
}
type Artwork struct {
	Data                            []byte
	ContentType, ETag, LastModified string
	NotModified                     bool
}
type Provider interface {
	Identity(context.Context) (Identity, error)
	Libraries(context.Context) ([]Library, error)
	Page(context.Context, string, int, int) (Page, error)
	Metadata(context.Context, string) (MediaItem, error)
	Artwork(context.Context, string, string, string) (Artwork, error)
}
type Options struct {
	HealthIntervalSeconds   int `json:"healthIntervalSeconds"`
	SyncIntervalSeconds     int `json:"syncIntervalSeconds"`
	FullSyncIntervalSeconds int `json:"fullSyncIntervalSeconds"`
	ArtworkCacheMB          int `json:"artworkCacheMb"`
}
