package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"spm/internal/model"
	"spm/internal/store"
	"strconv"
	"strings"
	"sync"
	"time"
)

type attempt struct {
	count int
	until time.Time
}
type Server struct {
	DB             *store.Store
	Runtime        *store.Runtime
	Now            func() time.Time
	mu             sync.Mutex
	nodes          map[string]model.Node
	settings       model.Settings
	username       string
	password       []byte
	verifyPassword func([]byte, []byte) error
	loginSlot      chan struct{}
	sessions       map[string]time.Time
	attempts       map[string]attempt
}

func New(ctx context.Context, db *store.Store, runtime *store.Runtime, user, password string) (*Server, error) {
	if strings.TrimSpace(user) == "" || len(password) < 12 || len(password) > 72 {
		return nil, errors.New("ADMIN_USER and ADMIN_PASSWORD (12–72 bytes) are required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	settings, err := db.Settings(ctx)
	if err != nil {
		return nil, err
	}
	nodes, err := db.Nodes(ctx)
	if err != nil {
		return nil, err
	}
	s := &Server{DB: db, Runtime: runtime, Now: time.Now, nodes: map[string]model.Node{}, settings: settings, username: user, password: hash, sessions: map[string]time.Time{}, attempts: map[string]attempt{}}
	s.verifyPassword = bcrypt.CompareHashAndPassword
	s.loginSlot = make(chan struct{}, 1)
	for _, n := range nodes {
		s.nodes[n.ID] = n
	}
	return s, nil
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		failure(w, 400, "無效的 JSON 或請求過大")
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		failure(w, 400, "請求僅可包含一個 JSON 物件")
		return false
	}
	return true
}
func (s *Server) admin(r *http.Request) bool {
	c, e := r.Cookie("spm_session")
	if e != nil {
		return false
	}
	expires, ok := s.sessions[c.Value]
	return ok && expires.After(s.Now())
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/session", s.session)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.HandleFunc("GET /api/nodes", s.listNodes)
	mux.HandleFunc("POST /api/nodes", s.createNode)
	mux.HandleFunc("PUT /api/nodes/{id}", s.updateNode)
	mux.HandleFunc("DELETE /api/nodes/{id}", s.deleteNode)
	mux.HandleFunc("POST /api/nodes/{id}/token", s.rotateToken)
	mux.HandleFunc("GET /api/nodes/{id}/history", s.history)
	mux.HandleFunc("POST /api/ingest/{id}", s.ingest)
	mux.HandleFunc("GET /api/alerts", s.alerts)
	mux.HandleFunc("GET /api/settings", s.getSettings)
	mux.HandleFunc("PUT /api/settings", s.putSettings)
	mux.HandleFunc("PUT /api/database", s.database)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != "GET" && !strings.HasPrefix(r.URL.Path, "/api/ingest/") {
			origin := r.Header.Get("Origin")
			if origin != "" {
				u, e := url.Parse(origin)
				if e != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
					failure(w, 403, "不允許跨來源操作")
					return
				}
			}
			if r.Header.Get("X-SPM-CSRF") != "1" {
				failure(w, 403, "缺少操作驗證標頭")
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) require(w http.ResponseWriter, r *http.Request, write bool) bool {
	if s.admin(r) || !write && s.settings.Public {
		return true
	}
	failure(w, 401, "請先登入管理員帳號")
	return false
}
func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	respond(w, 200, map[string]any{"admin": s.admin(r), "public": s.settings.Public})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &v) {
		return
	}
	select {
	case s.loginSlot <- struct{}{}:
		defer func() { <-s.loginSlot }()
	default:
		failure(w, 429, "正在驗證登入，請稍後再試")
		return
	}
	s.mu.Lock()
	now := s.Now()
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	for key, a := range s.attempts {
		if !a.until.After(now) {
			delete(s.attempts, key)
		}
	}
	for key, e := range s.sessions {
		if !e.After(now) {
			delete(s.sessions, key)
		}
	}
	a := s.attempts[ip]
	if a.count >= 10 || len(s.attempts) >= 10000 || len(s.sessions) >= 1000 {
		s.mu.Unlock()
		failure(w, 429, "登入嘗試過多，請稍後再試")
		return
	}
	if a.count == 0 {
		a.until = now.Add(10 * time.Minute)
	}
	a.count++
	s.attempts[ip] = a
	s.mu.Unlock()
	passwordOK := s.verifyPassword(s.password, []byte(v.Password)) == nil
	if !passwordOK || v.Username != s.username {
		failure(w, 401, "帳號或密碼不正確")
		return
	}
	s.mu.Lock()
	delete(s.attempts, ip)
	token := rand.Text()
	s.sessions[token] = now.Add(24 * time.Hour)
	s.mu.Unlock()
	// Handler already validates Origin's host. Browsers retain the external
	// HTTPS origin when a reverse proxy terminates TLS before forwarding HTTP.
	origin, _ := url.Parse(r.Header.Get("Origin"))
	secure := r.TLS != nil || origin != nil && origin.Scheme == "https"
	http.SetCookie(w, &http.Cookie{Name: "spm_session", Value: token, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: 86400})
	respond(w, 200, map[string]bool{"admin": true})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, e := r.Cookie("spm_session"); e == nil {
		delete(s.sessions, c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "spm_session", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) listNodes(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if !s.require(w, r, false) {
		s.mu.Unlock()
		return
	}
	out := make([]model.Node, 0, len(s.nodes))
	now := s.Now().UnixMilli()
	for _, n := range s.nodes {
		rules := model.Effective(s.settings.Rules, n.Rules)
		n.Online = n.LastSeen > 0 && now-n.LastSeen <= int64(rules.OfflineSeconds)*1000
		out = append(out, n)
	}
	s.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].ID < out[j].ID
		}
		return out[i].Name < out[j].Name
	})
	respond(w, 200, out)
}
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
func validName(name string) bool { return strings.TrimSpace(name) != "" && len(name) <= 100 }
func (s *Server) createNode(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &v) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.require(w, r, true) {
		return
	}
	if !validName(v.Name) {
		failure(w, 400, "主機名稱須為 1–100 bytes")
		return
	}
	token := rand.Text()
	n := model.Node{ID: rand.Text(), Name: strings.TrimSpace(v.Name), Created: s.Now().UnixMilli(), TokenHash: hashToken(token), States: map[string]model.AlarmState{}}
	if err := s.DB.SaveNode(r.Context(), n); err != nil {
		failure(w, 500, "無法儲存主機")
		return
	}
	s.nodes[n.ID] = n
	respond(w, 201, map[string]string{"id": n.ID, "token": token})
}
func (s *Server) updateNode(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Name  string       `json:"name"`
		Rules *model.Rules `json:"rules"`
	}
	if !decode(w, r, &v) {
		return
	}
	s.mu.Lock()
	if !s.require(w, r, true) {
		s.mu.Unlock()
		return
	}
	n, ok := s.nodes[r.PathValue("id")]
	if !ok {
		s.mu.Unlock()
		failure(w, 404, "找不到主機")
		return
	}
	if !validName(v.Name) || v.Rules != nil && v.Rules.Validate() != nil {
		s.mu.Unlock()
		failure(w, 400, "名稱或告警規則無效")
		return
	}
	n.Name = strings.TrimSpace(v.Name)
	n.Rules = v.Rules
	if err := s.DB.SaveNode(r.Context(), n); err != nil {
		s.mu.Unlock()
		failure(w, 500, "無法儲存主機")
		return
	}
	s.nodes[n.ID] = n
	s.mu.Unlock()
	respond(w, 200, n)
}
func (s *Server) deleteNode(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.require(w, r, true) {
		return
	}
	id := r.PathValue("id")
	if _, ok := s.nodes[id]; !ok {
		failure(w, 404, "找不到主機")
		return
	}
	if err := s.DB.DeleteNode(r.Context(), id); err != nil {
		failure(w, 500, "無法刪除主機")
		return
	}
	delete(s.nodes, id)
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) rotateToken(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.require(w, r, true) {
		return
	}
	n, ok := s.nodes[r.PathValue("id")]
	if !ok {
		failure(w, 404, "找不到主機")
		return
	}
	token := rand.Text()
	n.TokenHash = hashToken(token)
	if err := s.DB.SaveNode(r.Context(), n); err != nil {
		failure(w, 500, "無法更新 Token")
		return
	}
	s.nodes[n.ID] = n
	respond(w, 200, map[string]string{"id": n.ID, "token": token})
}
func copyNode(n model.Node) model.Node {
	states := map[string]model.AlarmState{}
	for k, v := range n.States {
		states[k] = v
	}
	n.States = states
	return n
}
func (s *Server) events(n *model.Node, now int64) ([]model.Alert, []string) {
	events := model.Evaluate(n, model.Effective(s.settings.Rules, n.Rules), now)
	for i := range events {
		events[i].ID = fmt.Sprintf("%020d-%s", time.Now().UnixNano(), rand.Text())
	}
	channels := []string{}
	if s.settings.Notifications.WebhookEnabled {
		channels = append(channels, "webhook")
	}
	if s.settings.Notifications.TelegramEnabled {
		channels = append(channels, "telegram")
	}
	return events, channels
}
func (s *Server) ingest(w http.ResponseWriter, r *http.Request) {
	var snapshot model.Snapshot
	if !decode(w, r, &snapshot) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[r.PathValue("id")]
	auth := r.Header.Get("Authorization")
	hash := hashToken(strings.TrimPrefix(auth, "Bearer "))
	if !ok || !strings.HasPrefix(auth, "Bearer ") || subtle.ConstantTimeCompare([]byte(hash), []byte(n.TokenHash)) != 1 {
		failure(w, 401, "Agent 驗證失敗")
		return
	}
	if err := snapshot.Validate(); err != nil {
		failure(w, 400, err.Error())
		return
	}
	var previous *model.Snapshot
	if n.Latest != nil {
		previous = &n.Latest.Snapshot
		if snapshot.Time <= previous.Time && snapshot.BootTime == previous.BootTime {
			failure(w, 409, "樣本時間未遞增")
			return
		}
	}
	n = copyNode(n)
	now := s.Now().UnixMilli()
	rules := model.Effective(s.settings.Rules, n.Rules)
	if now-n.LastSeen > int64(rules.OfflineSeconds)*1000 || previous != nil && snapshot.BootTime != previous.BootTime {
		for kind, state := range n.States {
			if !state.Active {
				state.Since = 0
				n.States[kind] = state
			}
		}
	}
	sample := model.Sample{Time: now, Snapshot: snapshot, Metrics: model.Derive(previous, snapshot)}
	n.Latest = &sample
	n.LastSeen = now
	events, channels := s.events(&n, now)
	if err := s.DB.Record(r.Context(), n, &sample, events, channels); err != nil {
		failure(w, 503, "資料庫暫時無法寫入")
		return
	}
	s.nodes[n.ID] = n
	respond(w, 200, map[string]int{"intervalSeconds": model.Effective(s.settings.Rules, n.Rules).IntervalSeconds})
}
func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	allowed := s.require(w, r, false)
	_, exists := s.nodes[r.PathValue("id")]
	s.mu.Unlock()
	if !allowed {
		return
	}
	if !exists {
		failure(w, 404, "找不到主機")
		return
	}
	q := r.URL.Query()
	from, e1 := strconv.ParseInt(q.Get("from"), 10, 64)
	to, e2 := strconv.ParseInt(q.Get("to"), 10, 64)
	limit, e3 := strconv.Atoi(q.Get("limit"))
	if e1 != nil || e2 != nil || e3 != nil || from < 0 || to <= from || to-from > 366*86400*1000 || limit < 1 || limit > 1000 {
		failure(w, 400, "查詢範圍或筆數無效")
		return
	}
	points, err := s.DB.History(r.Context(), r.PathValue("id"), from, to, limit)
	if err != nil {
		failure(w, 500, "無法讀取歷史")
		return
	}
	respond(w, 200, points)
}
func (s *Server) alerts(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	allowed := s.require(w, r, false)
	s.mu.Unlock()
	if !allowed {
		return
	}
	events, err := s.DB.Alerts(r.Context(), 100)
	if err != nil {
		failure(w, 500, "無法讀取告警")
		return
	}
	respond(w, 200, events)
}
func safeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	return u.String()
}
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if !s.require(w, r, true) {
		s.mu.Unlock()
		return
	}
	v := s.settings
	hasToken := v.Notifications.TelegramToken != ""
	v.Notifications.TelegramToken = ""
	result := map[string]any{"settings": v, "telegramTokenSet": hasToken, "database": map[string]any{"active": safeURL(s.Runtime.Active), "pending": safeURL(s.Runtime.Pending), "environment": s.Runtime.Environment, "error": s.Runtime.LastError}}
	s.mu.Unlock()
	respond(w, 200, result)
}
func notificationValid(n model.Notifications) bool {
	if n.WebhookEnabled {
		u, e := url.Parse(n.WebhookURL)
		if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return false
		}
	}
	return !n.TelegramEnabled || (n.TelegramToken != "" && n.TelegramChat != "" && !strings.ContainsAny(n.TelegramToken, "/\r\n?#"))
}
func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var v model.Settings
	if !decode(w, r, &v) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.require(w, r, true) {
		return
	}
	if v.Notifications.TelegramToken == "" {
		v.Notifications.TelegramToken = s.settings.Notifications.TelegramToken
	}
	if v.Rules.Validate() != nil || v.RetentionDays < 1 || v.RetentionDays > 365 || !notificationValid(v.Notifications) {
		failure(w, 400, "規則、保留天數或通知設定無效")
		return
	}
	if err := s.DB.SaveSettings(r.Context(), v); err != nil {
		failure(w, 500, "無法儲存設定")
		return
	}
	s.settings = v
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) database(w http.ResponseWriter, r *http.Request) {
	var v struct {
		URL string `json:"url"`
	}
	if !decode(w, r, &v) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.require(w, r, true) {
		return
	}
	if err := s.Runtime.Schedule(v.URL); err != nil {
		failure(w, 400, err.Error())
		return
	}
	respond(w, 200, map[string]bool{"restartRequired": v.URL != ""})
}
func (s *Server) Tick(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Now().UnixMilli()
	for id, current := range s.nodes {
		n := copyNode(current)
		events, channels := s.events(&n, now)
		if len(events) > 0 {
			if err := s.DB.Record(ctx, n, nil, events, channels); err != nil {
				return err
			}
		}
		s.nodes[id] = n
	}
	return nil
}
