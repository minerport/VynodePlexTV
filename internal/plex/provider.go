// Package plex defines the source boundary for Phase 1. No adapter is implemented yet.
package plex

import "context"

type Library struct{ ServerID, Key, Title, Type string }
type MediaItem struct {
	ServerID, LibraryKey, RatingKey, Title, Type string
	DurationMS                                   int64
}
type Page struct {
	Items      []MediaItem
	NextCursor string
}
type Artwork struct {
	ContentType string
	Data        []byte
}
type Provider interface {
	Health(context.Context) error
	Libraries(context.Context) ([]Library, error)
	Media(context.Context, string, string) (Page, error)
	Artwork(context.Context, string) (Artwork, error)
}
