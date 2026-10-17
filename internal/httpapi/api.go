package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"afterglow.local/tv/internal/config"
	"golang.org/x/crypto/bcrypt"
)

type API struct {
	db        *sql.DB
	cfg       config.Config
	log       *slog.Logger
	version   string
	limiter   rateLimiter
	dummyHash []byte
}
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}
type Session struct {
	User    User   `json:"user"`
	CSRF    string `json:"csrfToken"`
	Expires int64  `json:"expiresAt"`
}
type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type Settings struct {
	DisplayName string `json:"displayName"`
	Timezone    string `json:"timezone"`
}

func New(db *sql.DB, cfg config.Config, logger *slog.Logger, version string) *API {
	dummy, err := bcrypt.GenerateFromPassword([]byte("dummy-password-never-used"), 12)
	if err != nil {
		panic(err)
	}
	return &API{db: db, cfg: cfg, log: logger, version: version, dummyHash: dummy, limiter: rateLimiter{buckets: map[string]bucket{}}}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/v1/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := a.db.PingContext(r.Context()); err != nil {
			problem(w, 503, "Database unavailable")
			return
		}
		reply(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /api/v1/version", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, map[string]string{"version": a.version}) })
	mux.HandleFunc("GET /api/v1/auth/status", a.status)
	mux.HandleFunc("POST /api/v1/auth/setup", a.setup)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("GET /api/v1/auth/session", a.auth(func(w http.ResponseWriter, r *http.Request, s Session) { reply(w, 200, s) }))
	mux.HandleFunc("POST /api/v1/auth/logout", a.auth(a.logout))
	mux.HandleFunc("GET /api/v1/settings", a.auth(a.getSettings))
	mux.HandleFunc("PUT /api/v1/settings", a.auth(a.putSettings))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { problem(w, 404, "API endpoint not found") })
	mux.HandleFunc("/", a.static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		if a.cfg.Secure {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" && r.Header.Get("Origin") != a.cfg.Origin {
			problem(w, 403, "Request origin is not allowed")
			return
		}
		defer func() {
			if recover() != nil {
				a.log.Error("request panic", "method", r.Method)
				problem(w, 500, "Unexpected server error")
			}
		}()
		a.log.Debug("request", "method", r.Method, "path", r.URL.Path)
		mux.ServeHTTP(w, r)
	})
}

func (a *API) status(w http.ResponseWriter, r *http.Request) {
	var count int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		a.fail(w, err)
		return
	}
	reply(w, 200, map[string]bool{"setupRequired": count == 0})
}

func (a *API) setup(w http.ResponseWriter, r *http.Request) {
	if !a.allow(w, r) {
		return
	}
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	c.Username = strings.TrimSpace(c.Username)
	if len(c.Username) < 3 || len(c.Username) > 40 || !validUsername(c.Username) || len(c.Password) < 12 || len(c.Password) > 72 || !utf8.ValidString(c.Password) {
		problem(w, 400, "Use a 3–40 character username (letters, numbers, . _ -) and a password of 12–72 UTF-8 bytes")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), 12)
	if err != nil {
		a.fail(w, err)
		return
	}
	id, err := randomToken()
	if err != nil {
		a.fail(w, err)
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.fail(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `INSERT INTO users(id,username,password_hash,role,created_at) SELECT ?,?,?,'OWNER',? WHERE NOT EXISTS (SELECT 1 FROM users)`, id, c.Username, string(hash), time.Now().Unix())
	if err != nil {
		a.fail(w, err)
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		a.fail(w, err)
		return
	}
	if n != 1 {
		problem(w, 409, "Owner setup is already complete")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO user_preferences(user_id) VALUES(?)", id); err != nil {
		a.fail(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO audit_log(user_id,action,created_at) VALUES(?,'owner.created',?)", id, time.Now().Unix()); err != nil {
		a.fail(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		a.fail(w, err)
		return
	}
	reply(w, 201, map[string]string{"status": "created"})
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if !a.allow(w, r) {
		return
	}
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	var user User
	var hash string
	err := a.db.QueryRowContext(r.Context(), "SELECT id,username,role,password_hash FROM users WHERE username=?", strings.TrimSpace(c.Username)).Scan(&user.ID, &user.Username, &user.Role, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		a.fail(w, err)
		return
	}
	candidate := []byte(hash)
	if err != nil {
		candidate = a.dummyHash
	}
	passwordErr := bcrypt.CompareHashAndPassword(candidate, []byte(c.Password))
	if err != nil || passwordErr != nil {
		problem(w, 401, "Username or password is incorrect")
		return
	}
	token, err := randomToken()
	if err != nil {
		a.fail(w, err)
		return
	}
	csrf, err := randomToken()
	if err != nil {
		a.fail(w, err)
		return
	}
	expires := time.Now().Add(24 * time.Hour).Unix()
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.fail(w, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE expires_at<=?", time.Now().Unix()); err != nil {
		a.fail(w, err)
		return
	}
	if cookie, e := r.Cookie("afterglow_session"); e == nil {
		if _, err = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash=?", digest(cookie.Value)); err != nil {
			a.fail(w, err)
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO sessions VALUES(?,?,?,?)", digest(token), user.ID, csrf, expires); err != nil {
		a.fail(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		a.fail(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "afterglow_session", Value: token, Path: "/", HttpOnly: true, Secure: a.cfg.Secure, SameSite: http.SameSiteStrictMode, MaxAge: 86400, Expires: time.Unix(expires, 0)})
	reply(w, 200, Session{User: user, CSRF: csrf, Expires: expires})
}

func (a *API) auth(next func(http.ResponseWriter, *http.Request, Session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("afterglow_session")
		if err != nil {
			problem(w, 401, "Sign in to continue")
			return
		}
		var s Session
		err = a.db.QueryRowContext(r.Context(), `SELECT u.id,u.username,u.role,s.csrf_token,s.expires_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_at>?`, digest(cookie.Value), time.Now().Unix()).Scan(&s.User.ID, &s.User.Username, &s.User.Role, &s.CSRF, &s.Expires)
		if errors.Is(err, sql.ErrNoRows) {
			problem(w, 401, "Your session has expired. Sign in again")
			return
		}
		if err != nil {
			a.fail(w, err)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(s.CSRF)) != 1 {
			problem(w, 403, "Request verification failed. Refresh and try again")
			return
		}
		next(w, r, s)
	}
}
func (a *API) logout(w http.ResponseWriter, r *http.Request, s Session) {
	cookie, err := r.Cookie("afterglow_session")
	if err != nil {
		problem(w, 401, "Sign in to continue")
		return
	}
	if _, err = a.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash=?", digest(cookie.Value)); err != nil {
		a.fail(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "afterglow_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.cfg.Secure, SameSite: http.SameSiteStrictMode})
	reply(w, 200, map[string]string{"status": "signed_out"})
}
func (a *API) getSettings(w http.ResponseWriter, r *http.Request, s Session) {
	var settings Settings
	if err := a.db.QueryRowContext(r.Context(), "SELECT display_name,timezone FROM app_settings WHERE id=1").Scan(&settings.DisplayName, &settings.Timezone); err != nil {
		a.fail(w, err)
		return
	}
	reply(w, 200, settings)
}
func (a *API) putSettings(w http.ResponseWriter, r *http.Request, s Session) {
	if s.User.Role != "OWNER" && s.User.Role != "ADMIN" {
		problem(w, 403, "Administrator access required")
		return
	}
	var settings Settings
	if !decode(w, r, &settings) {
		return
	}
	settings.DisplayName = strings.TrimSpace(settings.DisplayName)
	_, err := time.LoadLocation(settings.Timezone)
	if err != nil || settings.Timezone == "Local" || settings.Timezone == "" || len(settings.DisplayName) < 1 || utf8.RuneCountInString(settings.DisplayName) > 60 {
		problem(w, 400, "Enter a name up to 60 characters and a valid IANA timezone")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.fail(w, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "UPDATE app_settings SET display_name=?, timezone=? WHERE id=1", settings.DisplayName, settings.Timezone); err != nil {
		a.fail(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO audit_log(user_id,action,created_at) VALUES(?,'settings.updated',?)", s.User.ID, time.Now().Unix()); err != nil {
		a.fail(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		a.fail(w, err)
		return
	}
	reply(w, 200, settings)
}

func (a *API) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		problem(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	path := filepath.Join(a.cfg.WebDir, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/")))
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/assets/") || filepath.Ext(r.URL.Path) != "" {
		http.NotFound(w, r)
		return
	}
	index := filepath.Join(a.cfg.WebDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		problem(w, 503, "Frontend not built. Use the Vite development server or build the frontend")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, index)
}
func (a *API) fail(w http.ResponseWriter, err error) {
	a.log.Error("operation failed", "error", err)
	problem(w, 500, "Unable to complete this request")
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, message string) {
	reply(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		problem(w, 415, "Content-Type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		problem(w, 400, "Invalid request body")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		problem(w, 400, "Expected one JSON object")
		return false
	}
	return true
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func digest(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }
func validUsername(s string) bool {
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

type bucket struct {
	count int
	reset time.Time
}
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]bucket
}

func (a *API) allow(w http.ResponseWriter, r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	a.limiter.mu.Lock()
	defer a.limiter.mu.Unlock()
	now := time.Now()
	for key, b := range a.limiter.buckets {
		if now.After(b.reset) {
			delete(a.limiter.buckets, key)
		}
	}
	b := a.limiter.buckets[host]
	if b.reset.IsZero() {
		b = bucket{reset: now.Add(15 * time.Minute)}
	}
	if b.count >= 20 || len(a.limiter.buckets) >= 10000 {
		w.Header().Set("Retry-After", "900")
		problem(w, 429, "Too many sign-in attempts. Try again in 15 minutes")
		return false
	}
	b.count++
	a.limiter.buckets[host] = b
	return true
}
