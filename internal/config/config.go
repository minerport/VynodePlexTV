package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
)

type Config struct {
	Dir, Address, Origin, WebDir, LogLevel string
	Secure                                 bool
}

func Load() (Config, error) {
	c := Config{Dir: env("CONFIG_DIR", "/config"), Address: env("LISTEN_ADDR", ":8080"), Origin: env("APP_ORIGIN", "http://localhost:8080"), WebDir: env("WEB_DIR", "web/dist"), LogLevel: env("LOG_LEVEL", "INFO")}
	u, err := url.Parse(c.Origin)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return c, fmt.Errorf("APP_ORIGIN must be an http(s) origin without a path")
	}
	c.Secure, err = strconv.ParseBool(env("COOKIE_SECURE", "false"))
	if err != nil {
		return c, fmt.Errorf("invalid COOKIE_SECURE")
	}
	if c.Secure && u.Scheme != "https" {
		return c, fmt.Errorf("secure cookies require an https APP_ORIGIN")
	}
	return c, nil
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
