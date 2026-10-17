package plex

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"
)

func (s *Service) Artwork(ctx context.Context, id, kind string) (Artwork, error) {
	item, err := s.Item(ctx, id)
	if err != nil {
		return Artwork{}, err
	}
	path := ""
	switch kind {
	case "poster":
		path = item.Thumb
		if path == "" {
			path = item.ParentThumb
		}
		if path == "" {
			path = item.GrandparentThumb
		}
	case "background":
		path = item.Art
	default:
		return Artwork{}, failure("NOT_FOUND", "Unknown artwork type")
	}
	if !safeArtworkPath(path) {
		return Artwork{}, failure("NOT_FOUND", "No artwork is available for this item")
	}
	sum := sha256.Sum256([]byte(item.ServerID + ":" + path))
	key := hex.EncodeToString(sum[:])
	file := filepath.Join(s.dir, "cache", "artwork", key)
	// Serialize cache fills and eviction to prevent duplicate downloads or eviction/write races.
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	var cached Artwork
	var cachedAt int64
	err = s.db.QueryRowContext(ctx, "SELECT content_type,etag,last_modified,cached_at FROM artwork_cache WHERE cache_key=?", key).Scan(&cached.ContentType, &cached.ETag, &cached.LastModified, &cachedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Artwork{}, err
	}
	exists := false
	if err == nil {
		cached.Data, err = os.ReadFile(file)
		exists = err == nil
	}
	if exists && now()-cachedAt < 86400 {
		if _, err = s.db.ExecContext(ctx, "UPDATE artwork_cache SET last_access_at=? WHERE cache_key=?", now(), key); err != nil {
			return Artwork{}, err
		}
		return cached, nil
	}
	client, _, err := s.provider(ctx, item.ServerID)
	if err != nil {
		if exists {
			return cached, nil
		}
		return Artwork{}, err
	}
	if !exists {
		cached.ETag = ""
		cached.LastModified = ""
	}
	art, err := client.Artwork(ctx, path, cached.ETag, cached.LastModified)
	if err != nil {
		if exists {
			return cached, nil
		}
		return Artwork{}, err
	}
	if art.NotModified && exists {
		if _, err = s.db.ExecContext(ctx, "UPDATE artwork_cache SET cached_at=?,last_access_at=? WHERE cache_key=?", now(), now(), key); err != nil {
			return Artwork{}, err
		}
		return cached, nil
	}
	if art.NotModified {
		return Artwork{}, failure("RESPONSE", "Plex artwork cache validation was inconsistent")
	}
	temp, err := os.CreateTemp(filepath.Dir(file), ".art-")
	if err != nil {
		return Artwork{}, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err = temp.Write(art.Data); err != nil {
		temp.Close()
		return Artwork{}, err
	}
	if err = temp.Close(); err != nil {
		return Artwork{}, err
	}
	if err = os.Rename(tempName, file); err != nil {
		return Artwork{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO artwork_cache(cache_key,content_type,etag,last_modified,cached_at,last_access_at,size) VALUES(?,?,?,?,?,?,?) ON CONFLICT(cache_key) DO UPDATE SET content_type=excluded.content_type,etag=excluded.etag,last_modified=excluded.last_modified,cached_at=excluded.cached_at,last_access_at=excluded.last_access_at,size=excluded.size`, key, art.ContentType, art.ETag, art.LastModified, now(), now(), len(art.Data))
	if err != nil {
		return Artwork{}, err
	}
	if err = s.pruneArtwork(ctx); err != nil {
		return Artwork{}, err
	}
	return art, nil
}
func (s *Service) maintainArtwork(ctx context.Context) error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if err := s.pruneArtwork(ctx); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM jobs WHERE status IN ('SUCCEEDED','FAILED','CANCELLED') AND completed_at<?", time.Now().Add(-30*24*time.Hour).Unix()); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM metadata_entities WHERE id NOT IN (SELECT entity_id FROM media_item_entities)")
	return err
}
func (s *Service) pruneArtwork(ctx context.Context) error {
	options, err := s.Settings(ctx)
	if err != nil {
		return err
	}
	limit := int64(options.ArtworkCacheMB) * 1024 * 1024
	rows, err := s.db.QueryContext(ctx, "SELECT cache_key,size FROM artwork_cache ORDER BY last_access_at DESC")
	if err != nil {
		return err
	}
	type entry struct {
		key  string
		size int64
	}
	all := []entry{}
	for rows.Next() {
		var e entry
		if err = rows.Scan(&e.key, &e.size); err != nil {
			rows.Close()
			return err
		}
		all = append(all, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	retained := map[string]bool{}
	var total int64
	dir := filepath.Join(s.dir, "cache", "artwork")
	for _, e := range all {
		if total+e.size <= limit {
			retained[e.key] = true
			total += e.size
			continue
		}
		if err = os.Remove(filepath.Join(dir, e.key)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if _, err = s.db.ExecContext(ctx, "DELETE FROM artwork_cache WHERE cache_key=?", e.key); err != nil {
			return err
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, f := range files {
		if !f.IsDir() && !retained[f.Name()] {
			if err = os.Remove(filepath.Join(dir, f.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
