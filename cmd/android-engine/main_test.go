package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStandaloneBootAndPrivateSession(t *testing.T) {
	input, write := io.Pipe()
	output, emit := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- run(input, emit); emit.Close() }()
	dir := t.TempDir()
	go func() {
		json.NewEncoder(write).Encode(map[string]any{"Dir": dir, "Token": strings.Repeat("a", 64), "CSRF": strings.Repeat("b", 64), "Key": strings.Repeat("c", 64), "ClientID": "test-standalone-installation", "Timezone": "America/New_York"})
	}()
	var ready struct{ Origin string }
	if err := json.NewDecoder(output).Decode(&ready); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ready.Origin, "http://127.0.0.1:") {
		t.Fatal("engine exposed beyond loopback")
	}
	client := &http.Client{Timeout: 3 * time.Second}
	request := func(path string, authenticated bool) int {
		r, _ := http.NewRequest("GET", ready.Origin+path, nil)
		if authenticated {
			r.AddCookie(&http.Cookie{Name: "afterglow_session", Value: strings.Repeat("a", 64)})
		}
		resp, e := client.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	if code := request("/api/v1/plex/servers", false); code != 401 {
		t.Fatalf("unauthorized=%d", code)
	}
	if code := request("/api/v1/plex/servers", true); code != 200 {
		t.Fatalf("local session=%d", code)
	}
	if code := request("/api/v1/guide/on-now", true); code != 200 {
		t.Fatalf("local guide=%d", code)
	}
	write.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("engine did not exit when app pipe closed")
	}
}
