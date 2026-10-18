package config

import "testing"

func TestOriginConfiguration(t *testing.T) {
	for _, origin := range []string{"http://localhost:8080/path", "file://host", "http://name:password@host", "https://host/"} {
		t.Setenv("APP_ORIGIN", origin)
		if _, err := Load(); err == nil {
			t.Fatalf("accepted %s", origin)
		}
	}
	t.Setenv("APP_ORIGIN", "http://localhost:8080")
	t.Setenv("COOKIE_SECURE", "true")
	if _, err := Load(); err == nil {
		t.Fatal("secure cookies accepted on HTTP")
	}
	t.Setenv("APP_ORIGIN", "https://tv.example.com")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
}

func TestAutoOriginDefault(t *testing.T) {
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("COOKIE_SECURE", "false")
	c, err := Load()
	if err != nil || c.Origin != "auto" {
		t.Fatalf("auto default: %v", err)
	}
}
