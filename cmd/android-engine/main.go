// android-engine is packaged inside the APK. It is not a separately installed server.
// Native UI and Media3 use a random authenticated loopback port; nothing listens on the LAN.
package main

import (
	"afterglow.local/tv/internal/config"
	"afterglow.local/tv/internal/database"
	"afterglow.local/tv/internal/httpapi"
	"afterglow.local/tv/internal/personalization"
	"afterglow.local/tv/internal/plex"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
	_ "time/tzdata"
)

func main() {
	if run(os.Stdin, os.Stdout) != nil {
		os.Stderr.WriteString("Afterglow local engine failed; local data preserved\n")
		os.Exit(1)
	}
}
func run(input io.Reader, output io.Writer) error {
	var boot struct {
		Dir, Token, CSRF, Key, ClientID, Timezone string
		DNS                                       []string
	}
	decoder := json.NewDecoder(io.LimitReader(input, 16384))
	if e := decoder.Decode(&boot); e != nil {
		return e
	}
	// Android does not expose resolv.conf to native processes. Use DNS servers
	// reported by ConnectivityManager; retain standard TLS certificate validation.
	if len(boot.DNS) > 0 {
		net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			var last error
			for _, server := range boot.DNS {
				if net.ParseIP(server) == nil {
					continue
				}
				conn, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, network, net.JoinHostPort(server, "53"))
				if err == nil {
					return conn, nil
				}
				last = err
			}
			if last == nil {
				last = io.ErrUnexpectedEOF
			}
			return nil, last
		}}
	}
	key, e := hex.DecodeString(boot.Key)
	if e != nil {
		return e
	}
	vault, e := plex.NewVault(key, boot.ClientID)
	if e != nil {
		return e
	}
	db, e := database.Open(boot.Dir)
	if e != nil {
		return e
	}
	defer db.Close()
	if len(boot.Token) != 64 || len(boot.CSRF) != 64 {
		return io.ErrUnexpectedEOF
	}
	hash := sha256.Sum256([]byte(boot.Token))
	now := time.Now().Unix()
	tx, e := db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`INSERT OR IGNORE INTO users VALUES('device-owner','Local Viewer','not-a-password-login','OWNER',?);INSERT OR IGNORE INTO user_preferences(user_id) VALUES('device-owner');DELETE FROM sessions;`, now); e != nil {
		return e
	}
	if _, e = tx.Exec(`INSERT INTO sessions VALUES(?,'device-owner',?,?)`, hex.EncodeToString(hash[:]), boot.CSRF, now+7*86400); e != nil {
		return e
	}
	if _, err := time.LoadLocation(boot.Timezone); err == nil {
		if _, e = tx.Exec("UPDATE app_settings SET timezone=? WHERE timezone='UTC'", boot.Timezone); e != nil {
			return e
		}
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return e
	}
	defer listener.Close()
	origin := "http://" + listener.Addr().String()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := httpapi.New(db, config.Config{Dir: boot.Dir, Origin: origin, WebDir: boot.Dir + "/no-web-ui"}, log, "0.9.1-android")
	media, e := plex.NewServiceWithVault(db, boot.Dir, "0.9.1", log, vault)
	if e != nil {
		return e
	}
	api.MountPlex(media)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if e = api.Playback().Recover(ctx); e != nil {
		return e
	}
	var wg sync.WaitGroup
	for _, worker := range []func(context.Context) error{media.Run, api.Scheduler().Run, api.Playback().Run, personalization.New(db).Run} {
		wg.Add(1)
		go func(f func(context.Context) error) {
			defer wg.Done()
			if f(ctx) != nil {
				cancel()
			}
		}(worker)
	}
	server := &http.Server{Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() { io.Copy(io.Discard, input); cancel() }()
	go func() {
		<-ctx.Done()
		c, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		server.Shutdown(c)
	}()
	if e = json.NewEncoder(output).Encode(map[string]string{"origin": origin}); e != nil {
		return e
	}
	e = server.Serve(listener)
	cancel()
	wg.Wait()
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
