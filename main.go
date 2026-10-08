// 全站榜单 VPS 服务（骨朵/豆瓣/芒果/剧场/番剧 五源，Go 版，对应原 server.js）
// =============================================
// 常驻服务（默认端口 5555）：
//   - 每天定时抓取 5 个榜单源 + TMDB 匹配
//   - 网页管理面板：填 TMDB API Key、预览各源数据、手动更新
//   - 自动生成聚合 fw/rex 模块 widget.js（含 5 个子模块）
//
// 零依赖，只需 Go 构建；容器部署见 README。
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // 嵌入时区库（容器免装 tzdata）
	"unicode/utf8"

	"github.com/syuim/vps-widget/internal/sources"
	"github.com/syuim/vps-widget/internal/util"
	"github.com/syuim/vps-widget/internal/widget"
)

const (
	defaultPort       = "5555"
	defaultVPSAddress = "http://127.0.0.1:5555"
	sessionTTLMillis  = int64(24 * 3600 * 1000)
)

var (
	dataDir    = "data"
	configFile = envOr("CONFIG_PATH", "config.json") // 容器部署可指到挂载目录内（如 /app/data/config.json）
	indexFile  = filepath.Join("public", "index.html")
	reDataPath = regexp.MustCompile(`^/data/(\w+)\.json$`)
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ===== 配置 =====

// Config 服务配置（字段与 JS 版 config.json 一致）
type Config struct {
	TMDBAPIKey    string `json:"tmdbApiKey"`
	VPSAddress    string `json:"vpsAddress"`
	UpdateHour    int    `json:"updateHour"`
	UpdateEnabled bool   `json:"updateEnabled"`
	Password      string `json:"password,omitempty"`
}

var (
	configMu sync.Mutex
	config   Config
)

func loadConfig() {
	config = Config{VPSAddress: defaultVPSAddress, UpdateHour: 17, UpdateEnabled: true}
	// bind-mount 单文件时 docker 会把不存在的路径建成目录，此处修复
	if fi, err := os.Stat(configFile); err == nil && fi.IsDir() {
		os.Remove(configFile)
	}
	util.LoadJSON(configFile, &config)
}

func getConfig() Config {
	configMu.Lock()
	defer configMu.Unlock()
	return config
}

func updateConfig(fn func(*Config)) Config {
	configMu.Lock()
	defer configMu.Unlock()
	fn(&config)
	util.SaveJSON(configFile, config)
	return config
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func randomPassword(n int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return ""
		}
		b[i] = chars[idx.Int64()]
	}
	return string(b)
}

// ensurePassword config 里已经有密码哈希则不动；否则用 ADMIN_PASSWORD 初始化，未提供则随机生成并打印
func ensurePassword() {
	configMu.Lock()
	defer configMu.Unlock()
	if config.Password != "" {
		return
	}
	pw := os.Getenv("ADMIN_PASSWORD")
	generated := false
	if pw == "" {
		pw = randomPassword(10)
		generated = true
	}
	config.Password = sha256Hex(pw)
	util.SaveJSON(configFile, config)
	if generated {
		log.Printf("[init] 未设置 ADMIN_PASSWORD，已随机生成面板密码：%s（请在面板中修改）", pw)
	} else {
		log.Printf("[init] 已使用 ADMIN_PASSWORD 环境变量初始化面板密码")
	}
}

// ===== 登录会话 =====

var (
	sessMu   sync.Mutex
	sessions = map[string]int64{} // token -> 过期时间戳(ms)
)

func newSession() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		log.Printf("[session] rand error: %v", err)
	}
	token := hex.EncodeToString(b)
	sessMu.Lock()
	sessions[token] = time.Now().UnixMilli() + sessionTTLMillis
	sessMu.Unlock()
	return token
}

func isAuthed(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return false
	}
	t := strings.TrimPrefix(h, "Bearer ")
	sessMu.Lock()
	defer sessMu.Unlock()
	exp, ok := sessions[t]
	if !ok {
		return false
	}
	if exp < time.Now().UnixMilli() {
		delete(sessions, t)
		return false
	}
	return true
}

// ===== 状态 =====

type srcState struct {
	LastUpdated string `json:"last_updated"`
	Count       int    `json:"count"`
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
}

var (
	stateMu     sync.Mutex
	state       = map[string]*srcState{}
	updating    = map[string]bool{} // 每源独立锁
	lastRunDate string
)

func init() {
	for _, s := range sources.All {
		state[s.Name] = &srcState{}
	}
}

func dataFile(name string) string { return filepath.Join(dataDir, name+".json") }

// readSourceRaw 读数据文件原始字节；缺失或损坏时返回默认空结构
func readSourceRaw(name string) json.RawMessage {
	b, err := os.ReadFile(dataFile(name))
	if err == nil && json.Valid(b) {
		return b
	}
	if name == "bangumi" {
		return json.RawMessage(`{"last_updated":"","total_matched":0,"hot_anime":[]}`)
	}
	return json.RawMessage(`{"last_updated":""}`)
}

// ===== 抓取 =====

// analyze 从抓取结果统计条数与 last_updated（对应 JS countOf + data.last_updated）
func analyze(name string, v any) (int, string) {
	b, err := json.Marshal(v)
	if err != nil {
		return 0, ""
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return 0, ""
	}
	last := ""
	if s, ok := m["last_updated"].(string); ok {
		last = s
	}
	count := 0
	switch name {
	case "guduo":
		if cats, ok := m["categories"].(map[string]any); ok {
			for _, arr := range cats {
				if a, ok := arr.([]any); ok {
					count += len(a)
				}
			}
		}
	case "bangumi":
		if a, ok := m["hot_anime"].([]any); ok {
			count = len(a)
		}
	case "theater":
		for k, v := range m {
			if k == "last_updated" {
				continue
			}
			if tv, ok := v.(map[string]any); ok {
				if a, ok := tv["aired"].([]any); ok {
					count += len(a)
				}
			}
		}
	default: // douban / mgtv：统计非 last_updated 字段的数组
		for k, v := range m {
			if k == "last_updated" {
				continue
			}
			if a, ok := v.([]any); ok {
				count += len(a)
			}
		}
	}
	return count, last
}

func updateSource(name string) map[string]any {
	src := sources.ByName(name)
	if src == nil {
		return map[string]any{"ok": false, "error": "未知数据源"}
	}

	stateMu.Lock()
	if updating[name] {
		stateMu.Unlock()
		return map[string]any{"ok": false, "error": "该源已有更新任务在进行"}
	}
	stateMu.Unlock()

	apiKey := getConfig().TMDBAPIKey
	if apiKey == "" {
		return map[string]any{"ok": false, "error": "尚未配置 TMDB API Key"}
	}

	stateMu.Lock()
	updating[name] = true
	stateMu.Unlock()
	defer func() {
		stateMu.Lock()
		updating[name] = false
		stateMu.Unlock()
	}()

	os.MkdirAll(dataDir, 0o755)
	data, err := src.Fetch(apiKey)
	if err == nil {
		err = util.SaveJSON(dataFile(name), data)
	}
	if err != nil {
		msg := err.Error()
		stateMu.Lock()
		state[name].Error = msg
		state[name].OK = false
		stateMu.Unlock()
		log.Printf("[update:%s] error: %v", name, msg)
		return map[string]any{"ok": false, "source": name, "error": msg}
	}

	count, last := analyze(name, data)
	if last == "" {
		last = util.BjStamp()
	}
	stateMu.Lock()
	state[name] = &srcState{LastUpdated: last, Count: count, OK: true, Error: ""}
	stateMu.Unlock()
	log.Printf("[update:%s] ok, %d 条", name, count)
	return map[string]any{"ok": true, "count": count, "source": name}
}

func updateAll() *util.OMap {
	results := util.NewOMap()
	for _, s := range sources.All {
		results.Set(s.Name, updateSource(s.Name))
	}
	stateMu.Lock()
	lastRunDate = util.TodayString()
	stateMu.Unlock()
	return results
}

// ===== 定时 =====

func scheduleTick() {
	cfg := getConfig()
	if !cfg.UpdateEnabled {
		return
	}
	if util.BjNow().Hour() != cfg.UpdateHour {
		return
	}
	stateMu.Lock()
	ran := lastRunDate
	stateMu.Unlock()
	if util.TodayString() == ran {
		return
	}
	b, _ := json.Marshal(updateAll())
	log.Printf("[schedule] %s", b)
}

// ===== HTTP =====

func sendJSON(w http.ResponseWriter, status int, obj any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(obj); err != nil {
		log.Printf("[http] json encode error: %v", err)
	}
}

func sendText(w http.ResponseWriter, status int, text, ctype string) {
	if ctype == "" {
		ctype = "text/plain; charset=utf-8"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	io.WriteString(w, text)
}

func readBody(r *http.Request) map[string]any {
	m := map[string]any{}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return m
	}
	json.Unmarshal(body, &m)
	return m
}

// srcInfo /api/sources 列表项
type srcInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	LastUpdated string `json:"last_updated"`
	Count       int    `json:"count"`
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path

	// 面板
	if p == "/" && r.Method == "GET" {
		b, err := os.ReadFile(indexFile)
		if err != nil {
			sendText(w, 500, "index.html 不存在", "")
			return
		}
		sendText(w, 200, string(b), "text/html; charset=utf-8")
		return
	}

	// 数据接口
	if m := reDataPath.FindStringSubmatch(p); m != nil && r.Method == "GET" {
		name := m[1]
		if !sources.Has(name) {
			sendJSON(w, 404, map[string]any{"error": "未知数据源"})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.WriteHeader(200)
		w.Write(readSourceRaw(name))
		return
	}

	// 生成的聚合模块
	if p == "/widget.js" && r.Method == "GET" {
		sendText(w, 200, widget.Generate(getConfig().VPSAddress), "application/javascript; charset=utf-8")
		return
	}

	// ---- 认证（免登录）----
	if p == "/api/login" && r.Method == "POST" {
		body := readBody(r)
		pw, _ := body["password"].(string)
		if sha256Hex(pw) != getConfig().Password {
			sendJSON(w, 401, map[string]any{"ok": false, "error": "密码错误"})
			return
		}
		token := newSession()
		sendJSON(w, 200, map[string]any{"ok": true, "token": token})
		return
	}
	if p == "/api/logout" && r.Method == "POST" {
		body := readBody(r)
		if t, ok := body["token"].(string); ok && t != "" {
			sessMu.Lock()
			delete(sessions, t)
			sessMu.Unlock()
		}
		sendJSON(w, 200, map[string]any{"ok": true})
		return
	}
	if p == "/api/auth/status" && r.Method == "GET" {
		sendJSON(w, 200, map[string]any{"loggedIn": isAuthed(r)})
		return
	}

	// 其余 /api/* 一律需要登录
	if strings.HasPrefix(p, "/api/") && !isAuthed(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(401)
		w.Write([]byte(`{"ok":false,"error":"未登录"}`))
		return
	}

	// ---- API ----
	if p == "/api/sources" && r.Method == "GET" {
		cfg := getConfig()
		stateMu.Lock()
		list := make([]srcInfo, 0, len(sources.All))
		for _, s := range sources.All {
			st := state[s.Name]
			list = append(list, srcInfo{s.Name, s.Title, st.LastUpdated, st.Count, st.OK, st.Error})
		}
		lrd := lastRunDate
		stateMu.Unlock()
		sendJSON(w, 200, struct {
			Configured  bool      `json:"configured"`
			Sources     []srcInfo `json:"sources"`
			LastRunDate string    `json:"lastRunDate"`
		}{cfg.TMDBAPIKey != "", list, lrd})
		return
	}

	if p == "/api/config" && r.Method == "GET" {
		sendJSON(w, 200, getConfig())
		return
	}
	if p == "/api/config" && r.Method == "POST" {
		body := readBody(r)
		cfg := updateConfig(func(c *Config) {
			if v, ok := body["tmdbApiKey"].(string); ok {
				c.TMDBAPIKey = strings.TrimSpace(v)
			}
			if v, ok := body["vpsAddress"].(string); ok {
				t := strings.TrimSpace(v)
				if t == "" {
					t = defaultVPSAddress
				}
				c.VPSAddress = t
			}
			if v, ok := body["updateHour"].(float64); ok {
				h := int(v)
				if h == 0 {
					h = 17
				}
				if h < 0 {
					h = 0
				}
				if h > 23 {
					h = 23
				}
				c.UpdateHour = h
			}
			if v, ok := body["updateEnabled"].(bool); ok {
				c.UpdateEnabled = v
			}
		})
		sendJSON(w, 200, map[string]any{"ok": true, "configured": cfg.TMDBAPIKey != ""})
		return
	}

	if p == "/api/password" && r.Method == "POST" {
		body := readBody(r)
		oldPw, _ := body["oldPassword"].(string)
		newPw, _ := body["newPassword"].(string)
		if sha256Hex(oldPw) != getConfig().Password {
			sendJSON(w, 401, map[string]any{"ok": false, "error": "原密码错误"})
			return
		}
		if utf8.RuneCountInString(newPw) < 4 {
			sendJSON(w, 400, map[string]any{"ok": false, "error": "新密码至少 4 位"})
			return
		}
		updateConfig(func(c *Config) { c.Password = sha256Hex(newPw) })
		sendJSON(w, 200, map[string]any{"ok": true})
		return
	}

	if p == "/api/update" && r.Method == "POST" {
		body := readBody(r)
		if srcName, _ := body["source"].(string); srcName != "" {
			res := updateSource(srcName)
			status := 200
			if ok, _ := res["ok"].(bool); !ok {
				status = 400
			}
			sendJSON(w, status, res)
			return
		}
		res := updateAll()
		anyOk := false
		for _, s := range sources.All {
			if v, ok := res.Get(s.Name); ok {
				if m, ok := v.(map[string]any); ok {
					if b, _ := m["ok"].(bool); b {
						anyOk = true
						break
					}
				}
			}
		}
		status := 200
		if !anyOk {
			status = 400
		}
		sendJSON(w, status, res)
		return
	}

	if p == "/api/preview" && r.Method == "GET" {
		name := r.URL.Query().Get("source")
		if name == "" {
			name = "bangumi"
		}
		if !sources.Has(name) {
			sendJSON(w, 404, map[string]any{"error": "未知数据源"})
			return
		}
		stateMu.Lock()
		st := *state[name]
		stateMu.Unlock()
		sendJSON(w, 200, struct {
			Source      string          `json:"source"`
			LastUpdated string          `json:"last_updated"`
			Count       int             `json:"count"`
			OK          bool            `json:"ok"`
			Error       string          `json:"error"`
			Data        json.RawMessage `json:"data"`
		}{name, st.LastUpdated, st.Count, st.OK, st.Error, readSourceRaw(name)})
		return
	}

	if p == "/api/widget" && r.Method == "GET" {
		addr := strings.TrimRight(getConfig().VPSAddress, "/")
		dataURLs := make([]string, 0, len(sources.All))
		for _, s := range sources.All {
			dataURLs = append(dataURLs, addr+"/data/"+s.Name+".json")
		}
		sendJSON(w, 200, struct {
			URL      string   `json:"url"`
			DataURLs []string `json:"dataUrls"`
			Code     string   `json:"code"`
		}{addr + "/widget.js", dataURLs, widget.Generate(getConfig().VPSAddress)})
		return
	}

	sendJSON(w, 404, map[string]any{"error": "Not Found"})
}

func envPort() string {
	p := os.Getenv("PORT")
	if p == "" {
		return defaultPort
	}
	if n, err := strconv.Atoi(p); err != nil || n <= 0 || n > 65535 {
		return defaultPort
	}
	return p
}

func main() {
	loadConfig()
	ensurePassword()

	// 启动后若有 key 自动抓一轮（延迟 1.5s，同原版）
	if getConfig().TMDBAPIKey != "" {
		go func() {
			time.Sleep(1500 * time.Millisecond)
			b, _ := json.Marshal(updateAll())
			log.Printf("[startup] %s", b)
		}()
	}

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		for range ticker.C {
			scheduleTick()
		}
	}()

	port := envPort()
	log.Printf("全站榜单 VPS 服务已启动： http://0.0.0.0:%s", port)
	log.Printf("  面板： http://<VPS>:%s/", port)
	log.Printf("  数据： /data/{guduo,douban,mgtv,theater,bangumi}.json")
	log.Printf("  模块： http://<VPS>:%s/widget.js", port)

	srv := &http.Server{Addr: ":" + port, Handler: http.HandlerFunc(handleRoot)}
	log.Fatal(srv.ListenAndServe())
}
