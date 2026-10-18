package plex

import (
	"bytes"
	"testing"
)

func TestPlatformVaultRejectsWrongKeyAndIdentity(t *testing.T) {
	v, e := NewVault(bytes.Repeat([]byte{1}, 32), "synthetic-installation")
	if e != nil {
		t.Fatal(e)
	}
	encrypted, e := v.Encrypt("fixture-server", "synthetic-only-credential")
	if e != nil {
		t.Fatal(e)
	}
	same, _ := NewVault(bytes.Repeat([]byte{1}, 32), "synthetic-installation")
	if plain, e := same.Decrypt("fixture-server", 1, encrypted); e != nil || plain != "synthetic-only-credential" {
		t.Fatal("platform key cannot reopen ciphertext")
	}
	wrong, _ := NewVault(bytes.Repeat([]byte{2}, 32), "synthetic-installation")
	if _, e = wrong.Decrypt("fixture-server", 1, encrypted); e == nil {
		t.Fatal("wrong key accepted")
	}
	if _, e = v.Decrypt("other-server", 1, encrypted); e == nil {
		t.Fatal("wrong server accepted")
	}
	if _, e = NewVault([]byte{1}, "synthetic-installation"); e == nil {
		t.Fatal("short key accepted")
	}
}
func TestNativeQualityAndHDR(t *testing.T) {
	height := 2160
	v := MediaVersion{PartCount: 1, Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac", Height: &height, HDR: "HDR10"}
	c := Capabilities{ClientType: "android-tv", Containers: []string{"mkv"}, VideoCodecs: []string{"hevc"}, AudioCodecs: []string{"aac"}, MaxHeight: 2160, Quality: "ORIGINAL", HDRFormats: []string{"HDR10"}}
	if !DirectCompatible(v, c) {
		t.Fatal("compatible 4K rejected")
	}
	c.VideoMaxHeights = map[string]int{"hevc": 1080}
	if DirectCompatible(v, c) {
		t.Fatal("per-codec ceiling ignored")
	}
	c.VideoMaxHeights = nil
	c.HDRFormats = nil
	if DirectCompatible(v, c) {
		t.Fatal("unsupported HDR allowed")
	}
	c.HDRFormats = []string{"HDR10"}
	c.Quality = "720p"
	if DirectCompatible(v, c) {
		t.Fatal("quality ceiling ignored")
	}
	c.Quality = "ORIGINAL"
	c.AudioStreamID = "2"
	if DirectCompatible(v, c) {
		t.Fatal("track selection bypasses Plex negotiation")
	}
}
