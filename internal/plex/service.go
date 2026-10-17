package plex

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Factory func(base, token string) Provider
type Service struct {
	db           *sql.DB
	vault        *Vault
	dir          string
	log          *slog.Logger
	factory      Factory
	wake         chan struct{}
	mu           sync.Mutex
	activeID     string
	activeCancel context.CancelFunc
	cacheMu      sync.Mutex
	linkMu       sync.Mutex
	links        map[string]*linkState
	accountURL   string
	accountHTTP  *http.Client
	version      string
}

func NewService(db *sql.DB, dir, version string, logger *slog.Logger) (*Service, error) {
	var count int
	if err := db.QueryRow("SELECT count(*) FROM plex_servers").Scan(&count); err != nil {
		return nil, err
	}
	vault, err := OpenVault(dir, count > 0)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Join(dir, "cache", "artwork"), 0700); err != nil {
		return nil, err
	}
	s := &Service{db: db, vault: vault, dir: dir, log: logger, wake: make(chan struct{}, 1), links: make(map[string]*linkState), accountURL: "https://plex.tv", accountHTTP: newAccountHTTP(), version: version}
	s.factory = func(base, token string) Provider { return NewClient(base, token, vault.ClientID, version) }
	return s, nil
}
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("secure random unavailable")
	}
	return hex.EncodeToString(b)
}
func now() int64 { return time.Now().UTC().Unix() }

const serverColumns = `id,name,base_url,machine_identifier,product,version,platform,platform_version,friendly_name,owned,secure_connections,last_health_check_at,last_success_at,last_error,status,disabled,created_at,updated_at`

type scanner interface{ Scan(...any) error }

func scanServer(row scanner) (Server, error) {
	var v Server
	err := row.Scan(&v.ID, &v.Name, &v.BaseURL, &v.MachineIdentifier, &v.Product, &v.Version, &v.Platform, &v.PlatformVersion, &v.FriendlyName, &v.Owned, &v.SecureConnections, &v.LastHealthCheck, &v.LastSuccess, &v.LastError, &v.Status, &v.Disabled, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}
func (s *Service) Servers(ctx context.Context) ([]Server, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+serverColumns+" FROM plex_servers ORDER BY disabled,name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Server{}
	for rows.Next() {
		v, e := scanServer(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) Server(ctx context.Context, id string) (Server, error) {
	v, err := scanServer(s.db.QueryRowContext(ctx, "SELECT "+serverColumns+" FROM plex_servers WHERE id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return v, failure("NOT_FOUND", "Plex server not found")
	}
	return v, err
}
func (s *Service) provider(ctx context.Context, id string) (Provider, Server, error) {
	server, err := s.Server(ctx, id)
	if err != nil {
		return nil, server, err
	}
	if server.Disabled {
		return nil, server, failure("INVALID", "Plex server is disconnected. Reconnect it before continuing.")
	}
	var encrypted []byte
	var version int
	if err = s.db.QueryRowContext(ctx, "SELECT token_encrypted,token_key_version FROM plex_servers WHERE id=?", id).Scan(&encrypted, &version); err != nil {
		return nil, server, err
	}
	token, err := s.vault.Decrypt(id, version, encrypted)
	if err != nil {
		return nil, server, failure("CREDENTIAL", "Stored Plex credential cannot be decrypted. Restore the key or reconnect.")
	}
	return s.factory(server.BaseURL, token), server, nil
}
func (s *Service) TestConnection(ctx context.Context, c Connection) (Identity, error) {
	base, err := NormalizeURL(c.BaseURL)
	if err != nil {
		return Identity{}, err
	}
	if len(strings.TrimSpace(c.Token)) < 1 || len(c.Token) > 2048 || strings.ContainsAny(c.Token, "\r\n") {
		return Identity{}, failure("INVALID", "Enter a valid Plex token")
	}
	client := s.factory(base, strings.TrimSpace(c.Token))
	identity, err := client.Identity(ctx)
	if err != nil {
		return identity, err
	}
	// Library access is checked as some Plex identity endpoints allow unauthenticated requests.
	if _, err = client.Libraries(ctx); err != nil {
		return Identity{}, err
	}
	return identity, nil
}
func (s *Service) SaveServer(ctx context.Context, id string, c Connection, actor string) (Server, error) {
	base, err := NormalizeURL(c.BaseURL)
	if err != nil {
		return Server{}, err
	}
	c.BaseURL = base
	c.Name = strings.TrimSpace(c.Name)
	c.Token = strings.TrimSpace(c.Token)
	if len(c.Name) > 100 {
		return Server{}, failure("INVALID", "Display name must be 100 characters or fewer")
	}
	var old Server
	if id != "" {
		old, err = s.Server(ctx, id)
		if err != nil {
			return Server{}, err
		}
		if c.Token == "" {
			var encrypted []byte
			var keyVersion int
			if err = s.db.QueryRowContext(ctx, "SELECT token_encrypted,token_key_version FROM plex_servers WHERE id=?", id).Scan(&encrypted, &keyVersion); err != nil {
				return Server{}, err
			}
			c.Token, err = s.vault.Decrypt(id, keyVersion, encrypted)
			if err != nil {
				return Server{}, failure("CREDENTIAL", "Enter a replacement Plex token")
			}
		}
	}
	identity, err := s.TestConnection(ctx, c)
	if err != nil {
		return Server{}, err
	}
	if id != "" && old.MachineIdentifier != identity.MachineIdentifier {
		return Server{}, failure("CONFLICT", "This URL belongs to a different Plex server. Add it separately to preserve media identities.")
	}
	var existing string
	err = s.db.QueryRowContext(ctx, "SELECT id FROM plex_servers WHERE machine_identifier=?", identity.MachineIdentifier).Scan(&existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Server{}, err
	}
	if existing != "" && existing != id {
		return Server{}, failure("CONFLICT", "This Plex server already exists. Use Edit / reconnect on its server card.")
	}
	if id == "" {
		id = newID()
	}
	if c.Name == "" {
		c.Name = identity.FriendlyName
	}
	if c.Name == "" {
		c.Name = "Plex Media Server"
	}
	encrypted, err := s.vault.Encrypt(id, c.Token)
	if err != nil {
		return Server{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Server{}, err
	}
	defer tx.Rollback()
	var running int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM jobs WHERE server_id=? AND status IN ('QUEUED','RUNNING')", id).Scan(&running); err != nil {
		return Server{}, err
	}
	if running > 0 {
		return Server{}, failure("CONFLICT", "Wait for this server's active jobs to finish or cancel them before editing")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO plex_servers(id,name,base_url,machine_identifier,product,version,platform,platform_version,friendly_name,owned,secure_connections,token_encrypted,token_key_version,last_health_check_at,last_success_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,1,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,base_url=excluded.base_url,product=excluded.product,version=excluded.version,platform=excluded.platform,platform_version=excluded.platform_version,friendly_name=excluded.friendly_name,owned=excluded.owned,secure_connections=excluded.secure_connections,token_encrypted=excluded.token_encrypted,token_key_version=1,status='CONNECTED',last_error='',disabled=0,last_health_check_at=excluded.last_health_check_at,last_success_at=excluded.last_success_at,updated_at=excluded.updated_at`, id, c.Name, base, identity.MachineIdentifier, identity.Product, identity.Version, identity.Platform, identity.PlatformVersion, identity.FriendlyName, identity.Owned, identity.SecureConnections, encrypted, now(), now(), now(), now())
	if err != nil {
		return Server{}, failure("CONFLICT", "Unable to save this server. It may already be connected.")
	}
	if err = audit(ctx, tx, actor, "plex.server.saved"); err != nil {
		return Server{}, err
	}
	if err = tx.Commit(); err != nil {
		return Server{}, err
	}
	return s.Server(ctx, id)
}
func audit(ctx context.Context, tx *sql.Tx, user, action string) error {
	var actor any
	if user != "" {
		actor = user
	}
	_, err := tx.ExecContext(ctx, "INSERT INTO audit_log(user_id,action,created_at) VALUES(?,?,?)", actor, action, now())
	return err
}

// Disconnect is deliberately non-destructive: inventory identities are retained.
func (s *Service) RemoveServer(ctx context.Context, id, actor string) error {
	if _, err := s.Server(ctx, id); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var active int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM jobs WHERE server_id=? AND status IN ('QUEUED','RUNNING')", id).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return failure("CONFLICT", "Cancel or finish this server's jobs before disconnecting")
	}
	if _, err = tx.ExecContext(ctx, "UPDATE plex_servers SET disabled=1,status='OFFLINE',token_encrypted=x'',last_error='Disconnected by administrator',updated_at=? WHERE id=?", now(), id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE plex_libraries SET selected=0 WHERE plex_server_id=?", id); err != nil {
		return err
	}
	if err = audit(ctx, tx, actor, "plex.server.disconnected"); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) health(ctx context.Context, id string) error {
	client, server, err := s.provider(ctx, id)
	if err != nil {
		return err
	}
	identity, err := client.Identity(ctx)
	if err == nil && identity.MachineIdentifier != server.MachineIdentifier {
		err = failure("IDENTITY", "Plex identity changed. Check the configured server address.")
	}
	if err == nil {
		_, err = client.Libraries(ctx)
	}
	status := "CONNECTED"
	summary := ""
	if err != nil {
		status = "DEGRADED"
		summary = SafeMessage(err)
		switch ErrorCode(err) {
		case "UNAUTHORIZED":
			status = "UNAUTHORIZED"
		case "OFFLINE", "DNS", "TIMEOUT":
			status = "OFFLINE"
		}
	}
	_, saveErr := s.db.ExecContext(ctx, `UPDATE plex_servers SET status=?,last_error=?,last_health_check_at=?,last_success_at=CASE WHEN ?='CONNECTED' THEN ? ELSE last_success_at END,updated_at=? WHERE id=?`, status, summary, now(), status, now(), now(), id)
	if saveErr != nil {
		return saveErr
	}
	return err
}
func (s *Service) Settings(ctx context.Context) (Options, error) {
	var o Options
	err := s.db.QueryRowContext(ctx, "SELECT health_interval_seconds,sync_interval_seconds,full_sync_interval_seconds,artwork_cache_mb FROM plex_settings WHERE id=1").Scan(&o.HealthIntervalSeconds, &o.SyncIntervalSeconds, &o.FullSyncIntervalSeconds, &o.ArtworkCacheMB)
	return o, err
}
func (s *Service) SaveSettings(ctx context.Context, o Options, actor string) error {
	if o.HealthIntervalSeconds < 30 || o.HealthIntervalSeconds > 86400 || o.SyncIntervalSeconds < 300 || o.SyncIntervalSeconds > 604800 || o.FullSyncIntervalSeconds < o.SyncIntervalSeconds || o.FullSyncIntervalSeconds > 2592000 || o.ArtworkCacheMB < 16 || o.ArtworkCacheMB > 10240 {
		return failure("INVALID", "Use health 30–86400 seconds, sync 300–604800 seconds, full sync at least as often as 30 days and no more often than incremental sync, cache 16–10240 MB")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE plex_settings SET health_interval_seconds=?,sync_interval_seconds=?,full_sync_interval_seconds=?,artwork_cache_mb=? WHERE id=1", o.HealthIntervalSeconds, o.SyncIntervalSeconds, o.FullSyncIntervalSeconds, o.ArtworkCacheMB); err != nil {
		return err
	}
	if err = audit(ctx, tx, actor, "plex.settings.updated"); err != nil {
		return err
	}
	return tx.Commit()
}
