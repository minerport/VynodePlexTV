// Package scheduler defines a Plex-independent future scheduling boundary.
package scheduler

import (
	"context"
	"time"
)

type Item struct {
	ID       string
	Duration time.Duration
}
type Request struct {
	ChannelID, Revision, Mode string
	Seed                      int64
	From, Until               time.Time
	Items                     []Item
}
type Entry struct {
	ChannelID, ItemID string
	Start, End        time.Time
}
type Generator interface {
	Generate(context.Context, Request) ([]Entry, error)
}
