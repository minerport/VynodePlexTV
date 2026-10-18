package channels

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
)

const MaxUpload = 8 * 1024 * 1024
const MaxUploadStorage = 256 * 1024 * 1024

type Upload struct {
	ID          string `json:"id"`
	ContentType string `json:"contentType"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}
type Artwork struct {
	Data        []byte
	ContentType string
	MediaID     string
	Kind        string
}

var assetID = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (s *Service) Upload(ctx context.Context, data []byte, actor string) (Upload, error) {
	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	if len(data) == 0 || len(data) > MaxUpload {
		return Upload{}, invalid("artwork", "Choose an image no larger than 8 MB")
	}
	config, format, e := image.DecodeConfig(bytes.NewReader(data))
	types := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "webp": "image/webp"}
	if e != nil || types[format] == "" || config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > 36000000 {
		return Upload{}, invalid("artwork", "Choose a valid PNG, JPEG or WebP image up to 8192 pixels and 36 megapixels")
	}
	if _, _, e = image.Decode(bytes.NewReader(data)); e != nil {
		return Upload{}, invalid("artwork", "The image is incomplete or damaged")
	}
	sum := sha256.Sum256(data)
	id := hex.EncodeToString(sum[:])
	out := Upload{id, types[format], config.Width, config.Height}
	dir := filepath.Join(s.dir, "uploads", "channels")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return out, e
	}
	// Remove only old unreferenced uploads; content-addressed files may be shared by duplicates.
	rows, e := s.db.QueryContext(ctx, "SELECT id FROM channel_uploads u WHERE created_at<? AND NOT EXISTS(SELECT 1 FROM channels c WHERE c.logo_upload_id=u.id OR c.background_upload_id=u.id) AND NOT EXISTS(SELECT 1 FROM channel_presentation p WHERE p.compact_logo_id=u.id)", stamp()-86400)
	if e != nil {
		return out, e
	}
	obsolete := []string{}
	for rows.Next() {
		var key string
		if e = rows.Scan(&key); e != nil {
			rows.Close()
			return out, e
		}
		obsolete = append(obsolete, key)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, key := range obsolete {
		if !assetID.MatchString(key) {
			return out, fail("INTERNAL", "Invalid stored artwork identity")
		}
		if e = os.Remove(filepath.Join(dir, key)); e != nil && !os.IsNotExist(e) {
			return out, e
		}
		if _, e = s.db.ExecContext(ctx, "DELETE FROM channel_uploads WHERE id=?", key); e != nil {
			return out, e
		}
	}
	var size int64
	if e = s.db.QueryRowContext(ctx, "SELECT COALESCE(sum(size),0) FROM channel_uploads WHERE id<>?", id).Scan(&size); e != nil {
		return out, e
	}
	if size+int64(len(data)) > MaxUploadStorage {
		return out, invalid("artwork", "Channel upload storage is full (256 MB). Remove unused artwork and retry after cleanup.")
	}
	tmp, e := os.CreateTemp(dir, ".upload-")
	if e != nil {
		return out, e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(data); e != nil {
		tmp.Close()
		return out, e
	}
	if e = tmp.Sync(); e != nil {
		tmp.Close()
		return out, e
	}
	if e = tmp.Close(); e != nil {
		return out, e
	}
	if e = os.Rename(name, filepath.Join(dir, id)); e != nil {
		return out, e
	}
	_, e = s.db.ExecContext(ctx, "INSERT INTO channel_uploads VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING", id, out.ContentType, len(data), out.Width, out.Height, optional(actor), stamp())
	return out, e
}
func (s *Service) UploadArtwork(ctx context.Context, id string) (Artwork, error) {
	if !assetID.MatchString(id) {
		return Artwork{}, fail("NOT_FOUND", "Artwork not found")
	}
	var mime string
	if e := s.db.QueryRowContext(ctx, "SELECT content_type FROM channel_uploads WHERE id=?", id).Scan(&mime); e != nil {
		return Artwork{}, fail("NOT_FOUND", "Artwork not found")
	}
	data, e := os.ReadFile(filepath.Join(s.dir, "uploads", "channels", id))
	if e != nil {
		return Artwork{}, fail("NOT_FOUND", "Artwork file unavailable")
	}
	return Artwork{Data: data, ContentType: mime}, nil
}
func (s *Service) ChannelArtwork(ctx context.Context, id, kind string) (Artwork, error) {
	c, e := s.raw(ctx, id)
	if e != nil {
		return Artwork{}, e
	}
	if kind == "compact" {
		var compact string
		e = s.db.QueryRowContext(ctx, "SELECT COALESCE(compact_logo_id,'') FROM channel_presentation WHERE channel_id=?", id).Scan(&compact)
		if e != nil && e != sql.ErrNoRows {
			return Artwork{}, e
		}
		if compact != "" {
			return s.UploadArtwork(ctx, compact)
		}
		kind = "logo"
	}
	a := c.Logo
	plexKind := "poster"
	if kind == "background" {
		a = c.Background
		plexKind = "background"
	} else if kind != "logo" {
		return Artwork{}, fail("NOT_FOUND", "Artwork type not found")
	}
	if a.Source == "UPLOAD" && a.UploadID != nil {
		return s.UploadArtwork(ctx, *a.UploadID)
	}
	media := ""
	if c.ArtworkMediaID != nil {
		media = *c.ArtworkMediaID
	} else if c.Source.Show != nil {
		media = c.Source.Show.ShowID
	} else {
		preview, e := s.Preview(ctx, PreviewRequest{Source: c.Source, Limit: 1})
		if e == nil && len(preview.Items) > 0 {
			media = preview.Items[0].ID
			if c.Source.Recent != nil && c.Source.Recent.Kind == "TV" {
				var showID string
				err := s.db.QueryRowContext(ctx, `SELECT show.id FROM plex_media_items ep JOIN plex_media_items show ON show.plex_server_id=ep.plex_server_id AND show.plex_rating_key=ep.grandparent_rating_key WHERE ep.id=?`, media).Scan(&showID)
				if err == nil {
					media = showID
				} else if err != sql.ErrNoRows {
					return Artwork{}, err
				}
			}
		}
	}
	name := c.ShortName
	if name == "" {
		name = c.Name
	}
	fallback := Fallback(c.Number, name)
	fallback.MediaID = media
	fallback.Kind = plexKind
	return fallback, nil
}
func Fallback(number, name string) Artwork {
	escape := func(s string) string { var b bytes.Buffer; xml.EscapeText(&b, []byte(s)); return b.String() }
	data := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360" viewBox="0 0 640 360"><rect width="640" height="360" rx="28" fill="#191329"/><circle cx="560" cy="20" r="240" fill="#342249"/><text x="40" y="190" fill="#dbc1fc" font-family="sans-serif" font-size="76">%s</text><text x="44" y="250" fill="#ede6f5" font-family="sans-serif" font-size="30">%s</text></svg>`, escape(number), escape(name))
	return Artwork{Data: []byte(data), ContentType: "image/svg+xml"}
}
