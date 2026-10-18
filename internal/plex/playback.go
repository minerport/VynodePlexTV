package plex

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Capabilities describe formats actually reported by the client, not formats in the library.
type Capabilities struct {
	ClientType     string   `json:"clientType"`
	Containers     []string `json:"containers"`
	VideoCodecs    []string `json:"videoCodecs"`
	AudioCodecs    []string `json:"audioCodecs"`
	MaxHeight      int      `json:"maxHeight"`
	HLS            bool     `json:"hls"`
	ForceTranscode bool     `json:"forceTranscode"`
}
type PlaybackSource interface {
	Fetch(context.Context, string, string) (*http.Response, error)
	Close(context.Context) error
}
type PlaybackResolution struct {
	Source   PlaybackSource
	Mode     string
	Mime     string
	Duration int64
	ServerID string
	AddedAt  *int64
}
type streamSource struct {
	client                *Client
	session, root, prefix string
	mu                    sync.Mutex
	resources             map[string]string
	resourceIDs           map[string]string
	closed                bool
	transcode             bool
}

func (s *Service) StopPlayback(ctx context.Context, server, id, mode string) error {
	if mode == "DIRECT_PLAY" {
		return nil
	}
	provider, _, e := s.provider(ctx, server)
	if e != nil {
		return e
	}
	c, ok := provider.(*Client)
	if !ok {
		return failure("UNSUPPORTED", "Playback provider unavailable")
	}
	return (&streamSource{client: c, session: id, transcode: true}).Close(ctx)
}
func contains(v []string, s string) bool {
	for _, x := range v {
		if x == s {
			return true
		}
	}
	return false
}
func DirectCompatible(v MediaVersion, c Capabilities) bool {
	return !c.ForceTranscode && v.PartCount == 1 && v.Container == "mp4" && v.VideoCodec == "h264" && contains(c.Containers, "mp4") && contains(c.VideoCodecs, "h264") && contains(c.AudioCodecs, v.AudioCodec) && (c.MaxHeight <= 0 || v.Height != nil && *v.Height <= c.MaxHeight)
}
func (s *Service) ResolvePlayback(ctx context.Context, id string, caps Capabilities, session string) (PlaybackResolution, error) {
	local, e := s.Item(ctx, id)
	if e != nil {
		return PlaybackResolution{}, e
	}
	if !local.Available {
		return PlaybackResolution{}, failure("NOT_FOUND", "This program is currently unavailable")
	}
	return s.resolveMedia(ctx, local, caps, session)
}
func (s *Service) resolveMedia(ctx context.Context, local MediaItem, caps Capabilities, session string) (PlaybackResolution, error) {
	provider, _, e := s.provider(ctx, local.ServerID)
	if e != nil {
		return PlaybackResolution{}, e
	}
	client, ok := provider.(*Client)
	if !ok {
		return PlaybackResolution{}, failure("UNSUPPORTED", "Playback provider is unavailable")
	}
	live, e := client.Metadata(ctx, local.RatingKey)
	if e != nil {
		return PlaybackResolution{}, e
	}
	if live.Type != local.Type || local.GUID != "" && live.GUID != "" && local.GUID != live.GUID {
		return PlaybackResolution{}, failure("NOT_FOUND", "This program's Plex identity changed")
	}
	var selected *MediaVersion
	direct := false
	for i := range live.Versions {
		v := &live.Versions[i]
		if v.PartCount != 1 {
			continue
		}
		if selected == nil {
			selected = v
		}
		// Hosted extras may advertise MP4 codecs but have a remote Part key.
		// Plex must resolve those through its HLS pipeline; never proxy that URL.
		if DirectCompatible(*v, caps) && safeArtworkPath(v.PartKey) && strings.HasPrefix(v.PartKey, "/library/parts/") {
			selected = v
			direct = true
			break
		}
	}
	if selected == nil || live.DurationMS == nil || *live.DurationMS <= 0 {
		return PlaybackResolution{}, failure("UNSUPPORTED", "No supported single-part version is available")
	}
	v := *selected
	source := &streamSource{client: client, session: session, prefix: "/api/v1/playback/" + session + "/stream/", resources: map[string]string{}}
	mode, mime := "DIRECT_PLAY", "video/mp4"
	if direct {
		if !safeArtworkPath(v.PartKey) || !strings.HasPrefix(v.PartKey, "/library/parts/") {
			return PlaybackResolution{}, failure("RESPONSE", "Plex media location is invalid")
		}
		source.root = v.PartKey
	} else {
		if !caps.HLS {
			return PlaybackResolution{}, failure("UNSUPPORTED", "This browser cannot play this format. An HLS-capable browser is required")
		}
		source.transcode = true
		mode = "PLEX_HLS"
		mime = "application/vnd.apple.mpegurl"
		q := url.Values{"path": {"/library/metadata/" + local.RatingKey}, "mediaIndex": {strconv.Itoa(v.MediaIndex)}, "partIndex": {"0"}, "protocol": {"hls"}, "offset": {"0"}, "fastSeek": {"1"}, "directPlay": {"0"}, "directStream": {"1"}, "directStreamAudio": {"1"}, "videoQuality": {"100"}, "videoResolution": {"1920x1080"}, "maxVideoBitrate": {"12000"}, "audioBoost": {"100"}, "subtitleSize": {"100"}, "subtitles": {"none"}, "session": {session}, "X-Plex-Client-Profile-Name": {"Chrome"}}
		if caps.ForceTranscode {
			q.Set("directStream", "0")
		}
		source.root = "/video/:/transcode/universal/start.m3u8?" + q.Encode()
		q.Set("hasMDE", "1")
		source.root = "/video/:/transcode/universal/start.m3u8?" + q.Encode()
		// Plex may require a successful media decision for this exact session before start.
		decision, e := source.request(ctx, "/video/:/transcode/universal/decision?"+q.Encode(), "")
		if e != nil {
			return PlaybackResolution{}, e
		}
		var result struct {
			Code          int `xml:"generalDecisionCode,attr"`
			TranscodeCode int `xml:"transcodeDecisionCode,attr"`
			Video         []struct {
				Media []struct {
					Video string `xml:"videoDecision,attr"`
					Audio string `xml:"audioDecision,attr"`
					Parts []struct {
						Streams []struct {
							Type     int    `xml:"streamType,attr"`
							Decision string `xml:"decision,attr"`
						} `xml:"Stream"`
					} `xml:"Part"`
				} `xml:"Media"`
			} `xml:"Video"`
		}
		e = xml.NewDecoder(io.LimitReader(decision.Body, 2*1024*1024)).Decode(&result)
		decision.Body.Close()
		if e != nil || result.Code >= 2000 || result.TranscodeCode >= 2000 {
			return PlaybackResolution{}, failure("UNSUPPORTED", "Plex cannot convert this program for this browser")
		}
		for _, video := range result.Video {
			for _, media := range video.Media {
				for _, part := range media.Parts {
					for _, stream := range part.Streams {
						if stream.Type == 1 {
							media.Video = stream.Decision
						}
						if stream.Type == 2 {
							media.Audio = stream.Decision
						}
					}
				}
				if media.Video == "transcode" || media.Audio == "transcode" {
					mode = "TRANSCODE"
				} else if media.Video == "copy" && media.Audio == "copy" {
					mode = "DIRECT_STREAM"
				}
			}
		}
	}
	source.resources["main"] = source.root
	return PlaybackResolution{Source: source, Mode: mode, Mime: mime, Duration: *live.DurationMS, ServerID: local.ServerID, AddedAt: local.AddedAt}, nil
}

var playlistURI = regexp.MustCompile(`URI="([^"]+)"`)
var byteRange = regexp.MustCompile(`^bytes=([0-9]+-[0-9]*|-[0-9]+)$`)

func (s *streamSource) request(ctx context.Context, target, rangeHeader string) (*http.Response, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", s.client.base+target, nil)
	if e != nil {
		return nil, failure("INVALID", "Invalid playback request")
	}
	req.Header.Set("X-Plex-Token", s.client.token)
	req.Header.Set("X-Plex-Product", "Afterglow TV")
	req.Header.Set("X-Plex-Version", s.client.version)
	req.Header.Set("X-Plex-Client-Identifier", s.client.clientID+"-"+s.session)
	req.Header.Set("X-Plex-Platform", "Chrome")
	req.Header.Set("X-Plex-Device", "Afterglow Browser")
	req.Header.Set("Accept", "*/*")
	if rangeHeader != "" {
		if len(rangeHeader) > 100 || !byteRange.MatchString(rangeHeader) {
			return nil, failure("INVALID", "Invalid media range")
		}
		req.Header.Set("Range", rangeHeader)
	}
	client := *s.client.http
	client.Timeout = 0 // context and bounded dial/header timeouts; video may run for hours.
	r, e := client.Do(req)
	if e != nil {
		return nil, transportError(e)
	}
	stopping := strings.HasPrefix(target, "/video/:/transcode/universal/stop?")
	if stopping && (r.StatusCode == 204 || r.StatusCode == 400 || r.StatusCode == 404) {
		return r, nil
	}
	if r.StatusCode != 200 && r.StatusCode != 206 {
		r.Body.Close()
		switch r.StatusCode {
		case 401, 403:
			return nil, failure("UNAUTHORIZED", "Plex denied playback. Ask an administrator to reconnect the server")
		case 404:
			return nil, failure("NOT_FOUND", "This program is currently unavailable")
		default:
			return nil, failure("PLAYBACK", "Plex could not start playback. Check transcoding availability and retry")
		}
	}
	return r, nil
}
func (s *streamSource) resource(base, raw string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.IsAbs() || u.Host != "" || u.User != nil || u.Fragment != "" || strings.ContainsAny(raw, "\\\r\n") {
		return "", failure("RESPONSE", "Invalid Plex stream location")
	}
	b, _ := url.Parse(base)
	u = b.ResolveReference(u)
	if !strings.HasPrefix(u.Path, "/video/:/transcode/universal/session/"+s.session+"/") || strings.Contains(u.Path, "..") {
		return "", failure("RESPONSE", "Unexpected Plex stream location")
	}
	q := u.Query()
	for k := range q {
		if strings.HasPrefix(strings.ToLower(k), "x-plex-") {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	target := u.String()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resourceIDs == nil {
		s.resourceIDs = map[string]string{}
		for id, v := range s.resources {
			s.resourceIDs[v] = id
		}
	}
	if id, ok := s.resourceIDs[target]; ok {
		return s.prefix + id, nil
	}
	if len(s.resources) >= 20000 {
		return "", failure("RESPONSE", "Plex stream exceeds resource limit")
	}
	id := newID()
	s.resources[id] = target
	s.resourceIDs[target] = id
	return s.prefix + id, nil
}
func (s *streamSource) Fetch(ctx context.Context, id, rangeHeader string) (*http.Response, error) {
	s.mu.Lock()
	target, ok := s.resources[id]
	closed := s.closed
	s.mu.Unlock()
	if !ok || closed {
		return nil, failure("NOT_FOUND", "Playback session has ended")
	}
	r, e := s.request(ctx, target, rangeHeader)
	if e != nil {
		return nil, e
	}
	u, _ := url.Parse(target)
	if !strings.HasSuffix(u.Path, ".m3u8") {
		return r, nil
	}
	defer r.Body.Close()
	data, e := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024+1))
	if e != nil || len(data) > 2*1024*1024 || !bytes.HasPrefix(data, []byte("#EXTM3U")) {
		return nil, failure("RESPONSE", "Plex returned an invalid playlist")
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			line, e = s.resource(target, line)
		} else {
			line = playlistURI.ReplaceAllStringFunc(line, func(attr string) string {
				if e != nil {
					return ""
				}
				var dest string
				dest, e = s.resource(target, playlistURI.FindStringSubmatch(attr)[1])
				return `URI="` + dest + `"`
			})
		}
		if e != nil {
			return nil, e
		}
		lines[i] = line
	}
	data = []byte(strings.Join(lines, "\n"))
	if s.client.token != "" && bytes.Contains(data, []byte(s.client.token)) {
		return nil, failure("RESPONSE", "Invalid Plex playlist")
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	r.ContentLength = int64(len(data))
	r.Header.Set("Content-Type", "application/vnd.apple.mpegurl")
	r.Header.Set("Content-Length", fmt.Sprint(len(data)))
	return r, nil
}
func (s *streamSource) Close(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	if !s.transcode {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	r, e := s.request(ctx, "/video/:/transcode/universal/stop?session="+url.QueryEscape(s.session), "")
	if r != nil {
		r.Body.Close()
	}
	return e
}

// SetProxyPrefix separates a stable application viewer from each Plex stream session.
func (s *streamSource) SetProxyPrefix(prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prefix = prefix
}
