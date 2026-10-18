package interstitials

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var probes = make(chan struct{}, 2)

// Upload accepts only bounded MP4/H.264/AAC assets, inspected server-side without a shell or network protocols.
func (s *Service) Upload(ctx context.Context, name, kind, filename, actor string, src io.Reader, max int64) (Asset, error) {
	a := Asset{Name: name, Kind: kind, Source: "UPLOAD", Enabled: true, Available: true, Tags: []string{}}
	if !validKind(kind) || len(name) < 1 || len(name) > 160 || strings.ToLower(filepath.Ext(filename)) != ".mp4" {
		return a, bad("Choose a name, type and MP4 video")
	}
	select {
	case probes <- struct{}{}:
		defer func() { <-probes }()
	default:
		return a, bad("Two uploads are being inspected. Try again shortly")
	}
	if e := os.MkdirAll(s.Dir, 0700); e != nil {
		return a, e
	}
	f, e := os.CreateTemp(s.Dir, "incoming-*")
	if e != nil {
		return a, e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	n, e := io.Copy(f, io.LimitReader(src, max+1))
	f.Close()
	if e != nil {
		return a, bad("Upload interrupted")
	}
	if n > max {
		return a, bad("Video exceeds configured upload size")
	}
	head := make([]byte, 12)
	f, e = os.Open(tmp)
	if e != nil {
		return a, e
	}
	_, e = io.ReadFull(f, head)
	f.Close()
	if e != nil || string(head[4:8]) != "ftyp" {
		return a, bad("File is not an MP4 video")
	}
	probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, "ffprobe", "-v", "error", "-protocol_whitelist", "file,pipe", "-show_entries", "format=duration,format_name:stream=codec_type,codec_name,width,height", "-of", "json", tmp)
	limited := &probeOutput{}
	cmd.Stdout = limited
	cmd.Stderr = io.Discard
	e = cmd.Run()
	output := limited.Bytes()
	if e != nil || len(output) > 65536 {
		return a, bad("Video could not be inspected; use a valid MP4/H.264/AAC clip")
	}
	var info struct {
		Format struct {
			Duration string `json:"duration"`
			Name     string `json:"format_name"`
		} `json:"format"`
		Streams []struct {
			Type   string `json:"codec_type"`
			Codec  string `json:"codec_name"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"streams"`
	}
	if json.Unmarshal(output, &info) != nil {
		return a, bad("Invalid video metadata")
	}
	d, e := strconv.ParseFloat(info.Format.Duration, 64)
	if e != nil || d <= 0 || d > 600 {
		return a, bad("Clips must be shorter than 10 minutes")
	}
	a.Duration = int64(d * 1000)
	a.Container = "mp4"
	videos := 0
	for _, v := range info.Streams {
		switch v.Type {
		case "video":
			videos++
			a.VideoCodec = v.Codec
			a.Width = v.Width
			a.Height = v.Height
		case "audio":
			a.AudioCodec = v.Codec
			if v.Codec != "aac" {
				return a, bad("Audio must be AAC")
			}
		default:
			return a, bad("Use video/audio only; remove embedded attachments or subtitles")
		}
	}
	if videos != 1 || a.VideoCodec != "h264" || a.Width > 1920 || a.Height > 1080 || a.Width < 1 || a.Height < 1 {
		return a, bad("Use one H.264 video track, at most 1920×1080")
	}
	a.ID = ID()
	a.Size = n
	p, _ := s.Path(a.ID)
	if e = os.Rename(tmp, p); e != nil {
		return a, e
	}
	_, e = s.DB.ExecContext(ctx, `INSERT INTO interstitial_assets(id,name,kind,source,duration_ms,container,video_codec,audio_codec,width,height,size_bytes,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,unixepoch(),unixepoch())`, a.ID, a.Name, a.Kind, a.Source, a.Duration, a.Container, a.VideoCodec, a.AudioCodec, a.Width, a.Height, a.Size, actor)
	if e != nil {
		os.Remove(p)
	}
	return a, e
}

type LocalSource struct{ Path string }

func (s LocalSource) Close(context.Context) error { return nil }

type sectionBody struct {
	io.Reader
	io.Closer
}

func (s LocalSource) Fetch(ctx context.Context, resource, rangeHeader string) (*http.Response, error) {
	if resource != "main" {
		return nil, bad("Asset resource unavailable")
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	f, e := os.Open(s.Path)
	if e != nil {
		return nil, bad("Presentation file unavailable")
	}
	st, e := f.Stat()
	if e != nil {
		f.Close()
		return nil, e
	}
	start, end := int64(0), st.Size()-1
	code := 200
	h := http.Header{"Content-Type": []string{"video/mp4"}, "Accept-Ranges": []string{"bytes"}, "Cache-Control": []string{"private, max-age=3600"}}
	if rangeHeader != "" {
		invalid := func() (*http.Response, error) {
			f.Close()
			return &http.Response{StatusCode: 416, Header: http.Header{"Content-Range": []string{fmt.Sprintf("bytes */%d", st.Size())}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		if !strings.HasPrefix(rangeHeader, "bytes=") {
			return invalid()
		}
		parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
		if len(parts) != 2 {
			return invalid()
		}
		if parts[0] == "" {
			n, e := strconv.ParseInt(parts[1], 10, 64)
			if e != nil || n <= 0 {
				return invalid()
			}
			start = max(0, st.Size()-n)
		} else {
			start, e = strconv.ParseInt(parts[0], 10, 64)
			if e != nil {
				return invalid()
			}
			if parts[1] != "" {
				end, e = strconv.ParseInt(parts[1], 10, 64)
				if e != nil {
					return invalid()
				}
				end = min(end, st.Size()-1)
			}
		}
		if start < 0 || start > end {
			return invalid()
		}
		code = 206
		h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, st.Size()))
	}
	h.Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	return &http.Response{StatusCode: code, Header: h, ContentLength: end - start + 1, Body: sectionBody{io.NewSectionReader(f, start, end-start+1), f}}, nil
}

type probeOutput struct{ bytes.Buffer }

func (b *probeOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 65536 {
		return 0, io.ErrShortBuffer
	}
	return b.Buffer.Write(p)
}
