package plex

import (
	"encoding/xml"
	"strings"
)

// XML types never leave this adapter. Optional Plex fields remain nullable.
type xmlContainer struct {
	XMLName           xml.Name
	Size              *int      `xml:"size,attr"`
	TotalSize         *int      `xml:"totalSize,attr"`
	Offset            *int      `xml:"offset,attr"`
	MachineIdentifier string    `xml:"machineIdentifier,attr"`
	FriendlyName      string    `xml:"friendlyName,attr"`
	Product           string    `xml:"product,attr"`
	Version           string    `xml:"version,attr"`
	Platform          string    `xml:"platform,attr"`
	PlatformVersion   string    `xml:"platformVersion,attr"`
	Owned             *bool     `xml:"owned,attr"`
	Secure            *bool     `xml:"secure,attr"`
	Directories       []xmlItem `xml:"Directory"`
	Videos            []xmlItem `xml:"Video"`
}
type xmlItem struct {
	ExtraType int `xml:"extraType,attr"`
	Extras    struct {
		Videos []xmlItem `xml:"Video"`
	} `xml:"Extras"`
	Key                   string     `xml:"key,attr"`
	RatingKey             string     `xml:"ratingKey,attr"`
	ParentKey             string     `xml:"parentRatingKey,attr"`
	GrandparentKey        string     `xml:"grandparentRatingKey,attr"`
	Type                  string     `xml:"type,attr"`
	Title                 string     `xml:"title,attr"`
	SortTitle             string     `xml:"titleSort,attr"`
	OriginalTitle         string     `xml:"originalTitle,attr"`
	Summary               string     `xml:"summary,attr"`
	Year                  *int       `xml:"year,attr"`
	ContentRating         string     `xml:"contentRating,attr"`
	Rating                *float64   `xml:"rating,attr"`
	AudienceRating        *float64   `xml:"audienceRating,attr"`
	Duration              *int64     `xml:"duration,attr"`
	AddedAt               *int64     `xml:"addedAt,attr"`
	Updated               *int64     `xml:"updatedAt,attr"`
	OriginallyAvailableAt string     `xml:"originallyAvailableAt,attr"`
	Studio                string     `xml:"studio,attr"`
	Tagline               string     `xml:"tagline,attr"`
	Index                 *int       `xml:"index,attr"`
	ParentIndex           *int       `xml:"parentIndex,attr"`
	GUID                  string     `xml:"guid,attr"`
	Thumb                 string     `xml:"thumb,attr"`
	Art                   string     `xml:"art,attr"`
	ParentThumb           string     `xml:"parentThumb,attr"`
	GrandparentThumb      string     `xml:"grandparentThumb,attr"`
	Agent                 string     `xml:"agent,attr"`
	Scanner               string     `xml:"scanner,attr"`
	Language              string     `xml:"language,attr"`
	UUID                  string     `xml:"uuid,attr"`
	ScannedAt             string     `xml:"scannedAt,attr"`
	Genres                []xmlTag   `xml:"Genre"`
	Actors                []xmlTag   `xml:"Role"`
	Directors             []xmlTag   `xml:"Director"`
	Writers               []xmlTag   `xml:"Writer"`
	Collections           []xmlTag   `xml:"Collection"`
	Labels                []xmlTag   `xml:"Label"`
	GUIDs                 []xmlGUID  `xml:"Guid"`
	Media                 []xmlMedia `xml:"Media"`
}
type xmlTag struct {
	ID   string `xml:"id,attr"`
	Tag  string `xml:"tag,attr"`
	Role string `xml:"role,attr"`
}
type xmlGUID struct {
	ID string `xml:"id,attr"`
}
type xmlMedia struct {
	ID         string    `xml:"id,attr"`
	Container  string    `xml:"container,attr"`
	VideoCodec string    `xml:"videoCodec,attr"`
	AudioCodec string    `xml:"audioCodec,attr"`
	Resolution string    `xml:"videoResolution,attr"`
	Width      *int      `xml:"width,attr"`
	Height     *int      `xml:"height,attr"`
	Bitrate    *int      `xml:"bitrate,attr"`
	Duration   *int64    `xml:"duration,attr"`
	Optimized  *bool     `xml:"optimizedForStreaming,attr"`
	Parts      []xmlPart `xml:"Part"`
}
type xmlPart struct {
	Key       string      `xml:"key,attr"`
	ID        string      `xml:"id,attr"`
	File      string      `xml:"file,attr"`
	Size      *int64      `xml:"size,attr"`
	Container string      `xml:"container,attr"`
	Duration  *int64      `xml:"duration,attr"`
	Streams   []xmlStream `xml:"Stream"`
}
type xmlStream struct {
	ID              string `xml:"id,attr"`
	Type            *int   `xml:"streamType,attr"`
	Codec           string `xml:"codec,attr"`
	Language        string `xml:"language,attr"`
	LanguageCode    string `xml:"languageCode,attr"`
	Title           string `xml:"displayTitle,attr"`
	Default         *bool  `xml:"default,attr"`
	Forced          *bool  `xml:"forced,attr"`
	HearingImpaired *bool  `xml:"hearingImpaired,attr"`
	Channels        *int   `xml:"channels,attr"`
	Width           *int   `xml:"width,attr"`
	Height          *int   `xml:"height,attr"`
	Profile         string `xml:"profile,attr"`
}

func (n xmlItem) model() MediaItem {
	m := MediaItem{RatingKey: n.RatingKey, ParentKey: n.ParentKey, GrandparentKey: n.GrandparentKey, Type: n.Type, Title: n.Title, SortTitle: n.SortTitle, OriginalTitle: n.OriginalTitle, Summary: n.Summary, Year: n.Year, ContentRating: n.ContentRating, Rating: n.Rating, AudienceRating: n.AudienceRating, DurationMS: n.Duration, AddedAt: n.AddedAt, SourceUpdatedAt: n.Updated, OriginallyAvailableAt: n.OriginallyAvailableAt, Studio: n.Studio, Tagline: n.Tagline, Index: n.Index, ParentIndex: n.ParentIndex, GUID: n.GUID, Thumb: n.Thumb, Art: n.Art, ParentThumb: n.ParentThumb, GrandparentThumb: n.GrandparentThumb, Available: true}
	groups := []struct {
		tags               []xmlTag
		kind, relationship string
	}{{n.Genres, "genre", "genre"}, {n.Actors, "person", "actor"}, {n.Directors, "person", "director"}, {n.Writers, "person", "writer"}, {n.Collections, "collection", "collection"}, {n.Labels, "label", "label"}}
	for _, g := range groups {
		for _, tag := range g.tags {
			if tag.Tag == "" {
				continue
			}
			key := "id:" + tag.ID
			if tag.ID == "" {
				key = "name:" + strings.ToLower(strings.TrimSpace(tag.Tag))
			}
			m.Tags = append(m.Tags, Tag{Kind: g.kind, SourceKey: key, Name: tag.Tag, Relationship: g.relationship, Character: tag.Role})
		}
	}
	ids := append([]xmlGUID{{ID: n.GUID}}, n.GUIDs...)
	seen := map[string]bool{}
	for _, id := range ids {
		if id.ID == "" || seen[id.ID] {
			continue
		}
		seen[id.ID] = true
		provider, value, found := strings.Cut(id.ID, "://")
		if !found {
			provider = "unknown"
			value = id.ID
		}
		m.ExternalIDs = append(m.ExternalIDs, ExternalID{Provider: provider, Value: value})
	}
	for mi, media := range n.Media {
		for pi, part := range media.Parts {
			container := part.Container
			if container == "" {
				container = media.Container
			}
			duration := part.Duration
			if duration == nil {
				duration = media.Duration
			}
			v := MediaVersion{MediaID: media.ID, PartID: part.ID, Container: container, VideoCodec: media.VideoCodec, AudioCodec: media.AudioCodec, Resolution: media.Resolution, Width: media.Width, Height: media.Height, Bitrate: media.Bitrate, DurationMS: duration, File: part.File, Size: part.Size, Optimized: media.Optimized}
			v.PartKey = part.Key
			v.MediaIndex = mi
			v.PartIndex = pi
			v.PartCount = len(media.Parts)
			for _, s := range part.Streams {
				v.Streams = append(v.Streams, Stream{ID: s.ID, Type: s.Type, Codec: s.Codec, Language: s.Language, LanguageCode: s.LanguageCode, Title: s.Title, Default: s.Default, Forced: s.Forced, HearingImpaired: s.HearingImpaired, Channels: s.Channels, Width: s.Width, Height: s.Height, Profile: s.Profile})
			}
			m.Versions = append(m.Versions, v)
		}
	}
	return m
}
