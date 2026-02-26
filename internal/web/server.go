package web

import (
	"bytes"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"

	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
	"github.com/guohuiyuan/music-lib/utils"

	// 引入 sqlite 驱动
	_ "modernc.org/sqlite"
)

//go:embed templates/*
var templateFS embed.FS

const (
	UA_Common    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
	UA_Mobile    = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	Ref_Bilibili = "https://www.bilibili.com/"
	Ref_Migu     = "http://music.migu.cn/"
	CookieFile   = "cookies.json"
	RoutePrefix  = "/music"
)

// --- SQLite 数据库管理 ---
var db *sql.DB

func initDB() {
	var err error
	// 数据库文件将与 cookies.json 在同一目录
	db, err = sql.Open("sqlite", "favorites.db")
	if err != nil {
		panic("Failed to connect to SQLite: " + err.Error())
	}

	// 启用外键约束
	db.Exec("PRAGMA foreign_keys = ON;")

	schema := `
	CREATE TABLE IF NOT EXISTS collections (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS saved_songs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		collection_id INTEGER NOT NULL,
		song_id TEXT NOT NULL,
		source TEXT NOT NULL,
		extra TEXT,
		name TEXT,
		artist TEXT,
		cover TEXT,
		duration INTEGER,
		added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(collection_id, song_id, source),
		FOREIGN KEY(collection_id) REFERENCES collections(id) ON DELETE CASCADE
	);
	`
	if _, err := db.Exec(schema); err != nil {
		panic("Failed to init SQLite schema: " + err.Error())
	}
}

// --- Cookie 管理 ---
type CookieManager struct {
	mu      sync.RWMutex
	cookies map[string]string
}

var cm = &CookieManager{cookies: make(map[string]string)}

func (m *CookieManager) Load() {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := os.ReadFile(CookieFile)
	if err == nil {
		json.Unmarshal(data, &m.cookies)
	}
}

func (m *CookieManager) Save() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, _ := json.MarshalIndent(m.cookies, "", "  ")
	_ = os.WriteFile(CookieFile, data, 0644)
}

func (m *CookieManager) Get(source string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cookies[source]
}

func (m *CookieManager) SetAll(c map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range c {
		if v != "" {
			m.cookies[k] = v
		}
	}
}

// --- 中间件 ---
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		c.Next()
	}
}

func buildSourceRequest(method, urlStr, source, rangeHeader string) (*http.Request, error) {
	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		return nil, err
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	req.Header.Set("User-Agent", UA_Common)
	if source == "bilibili" {
		req.Header.Set("Referer", Ref_Bilibili)
	}
	if source == "migu" {
		req.Header.Set("User-Agent", UA_Mobile)
		req.Header.Set("Referer", Ref_Migu)
	}
	if source == "qq" {
		req.Header.Set("Referer", "http://y.qq.com")
	}
	if cookie := cm.Get(source); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	return req, nil
}

// --- 视频生成状态管理 ---
type RenderSession struct {
	ID        string
	Dir       string
	AudioPath string
	Total     int
	Mutex     sync.Mutex
}

var (
	sessions = make(map[string]*RenderSession)
	sessMu   sync.Mutex
)

func cleanupOldFiles(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > maxAge {
			os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

func saveBase64(dataURI, path string) error {
	if len(dataURI) > 23 {
		dataURI = dataURI[23:]
	}
	data, err := base64.StdEncoding.DecodeString(dataURI)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// --- 工厂函数 ---
func getSearchFunc(source string) func(string) ([]model.Song, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).Search
	case "qq":
		return qq.New(c).Search
	case "kugou":
		return kugou.New(c).Search
	case "kuwo":
		return kuwo.New(c).Search
	case "migu":
		return migu.New(c).Search
	case "soda":
		return soda.New(c).Search
	case "bilibili":
		return bilibili.New(c).Search
	case "fivesing":
		return fivesing.New(c).Search
	case "jamendo":
		return jamendo.New(c).Search
	case "joox":
		return joox.New(c).Search
	case "qianqian":
		return qianqian.New(c).Search
	default:
		return nil
	}
}

func getPlaylistSearchFunc(source string) func(string) ([]model.Playlist, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).SearchPlaylist
	case "qq":
		return qq.New(c).SearchPlaylist
	case "kugou":
		return kugou.New(c).SearchPlaylist
	case "kuwo":
		return kuwo.New(c).SearchPlaylist
	case "bilibili":
		return bilibili.New(c).SearchPlaylist
	case "soda":
		return soda.New(c).SearchPlaylist
	case "fivesing":
		return fivesing.New(c).SearchPlaylist
	default:
		return nil
	}
}

func getPlaylistDetailFunc(source string) func(string) ([]model.Song, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetPlaylistSongs
	case "qq":
		return qq.New(c).GetPlaylistSongs
	case "kugou":
		return kugou.New(c).GetPlaylistSongs
	case "kuwo":
		return kuwo.New(c).GetPlaylistSongs
	case "bilibili":
		return bilibili.New(c).GetPlaylistSongs
	case "soda":
		return soda.New(c).GetPlaylistSongs
	case "fivesing":
		return fivesing.New(c).GetPlaylistSongs
	default:
		return nil
	}
}

func getRecommendFunc(source string) func() ([]model.Playlist, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetRecommendedPlaylists
	case "qq":
		return qq.New(c).GetRecommendedPlaylists
	case "kugou":
		return kugou.New(c).GetRecommendedPlaylists
	case "kuwo":
		return kuwo.New(c).GetRecommendedPlaylists
	default:
		return nil
	}
}

func getDownloadFunc(source string) func(*model.Song) (string, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetDownloadURL
	case "qq":
		return qq.New(c).GetDownloadURL
	case "kugou":
		return kugou.New(c).GetDownloadURL
	case "kuwo":
		return kuwo.New(c).GetDownloadURL
	case "migu":
		return migu.New(c).GetDownloadURL
	case "soda":
		return soda.New(c).GetDownloadURL
	case "bilibili":
		return bilibili.New(c).GetDownloadURL
	case "fivesing":
		return fivesing.New(c).GetDownloadURL
	case "jamendo":
		return jamendo.New(c).GetDownloadURL
	case "joox":
		return joox.New(c).GetDownloadURL
	case "qianqian":
		return qianqian.New(c).GetDownloadURL
	default:
		return nil
	}
}

func getLyricFunc(source string) func(*model.Song) (string, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetLyrics
	case "qq":
		return qq.New(c).GetLyrics
	case "kugou":
		return kugou.New(c).GetLyrics
	case "kuwo":
		return kuwo.New(c).GetLyrics
	case "migu":
		return migu.New(c).GetLyrics
	case "soda":
		return soda.New(c).GetLyrics
	case "bilibili":
		return bilibili.New(c).GetLyrics
	case "fivesing":
		return fivesing.New(c).GetLyrics
	case "jamendo":
		return jamendo.New(c).GetLyrics
	case "joox":
		return joox.New(c).GetLyrics
	case "qianqian":
		return qianqian.New(c).GetLyrics
	default:
		return nil
	}
}

func getParseFunc(source string) func(string) (*model.Song, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).Parse
	case "qq":
		return qq.New(c).Parse
	case "kugou":
		return kugou.New(c).Parse
	case "kuwo":
		return kuwo.New(c).Parse
	case "migu":
		return migu.New(c).Parse
	case "soda":
		return soda.New(c).Parse
	case "bilibili":
		return bilibili.New(c).Parse
	case "fivesing":
		return fivesing.New(c).Parse
	case "jamendo":
		return jamendo.New(c).Parse
	default:
		return nil
	}
}

func getParsePlaylistFunc(source string) func(string) (*model.Playlist, []model.Song, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).ParsePlaylist
	case "qq":
		return qq.New(c).ParsePlaylist
	case "kugou":
		return kugou.New(c).ParsePlaylist
	case "kuwo":
		return kuwo.New(c).ParsePlaylist
	case "bilibili":
		return bilibili.New(c).ParsePlaylist
	case "soda":
		return soda.New(c).ParsePlaylist
	case "fivesing":
		return fivesing.New(c).ParsePlaylist
	default:
		return nil
	}
}

func detectSource(link string) string {
	if strings.Contains(link, "163.com") {
		return "netease"
	}
	if strings.Contains(link, "qq.com") {
		return "qq"
	}
	if strings.Contains(link, "5sing") {
		return "fivesing"
	}
	if strings.Contains(link, "kugou.com") {
		return "kugou"
	}
	if strings.Contains(link, "kuwo.cn") {
		return "kuwo"
	}
	if strings.Contains(link, "migu.cn") {
		return "migu"
	}
	if strings.Contains(link, "bilibili.com") || strings.Contains(link, "b23.tv") {
		return "bilibili"
	}
	if strings.Contains(link, "douyin.com") || strings.Contains(link, "qishui") {
		return "soda"
	}
	if strings.Contains(link, "jamendo.com") {
		return "jamendo"
	}
	return ""
}

func getOriginalLink(source, id, typeStr string) string {
	switch source {
	case "netease":
		if typeStr == "playlist" {
			return "https://music.163.com/#/playlist?id=" + id
		}
		return "https://music.163.com/#/song?id=" + id
	case "qq":
		if typeStr == "playlist" {
			return "https://y.qq.com/n/ryqq/playlist/" + id
		}
		return "https://y.qq.com/n/ryqq/songDetail/" + id
	case "kugou":
		if typeStr == "playlist" {
			return "https://www.kugou.com/yy/special/single/" + id + ".html"
		}
		return "https://www.kugou.com/song/#hash=" + id
	case "kuwo":
		if typeStr == "playlist" {
			return "http://www.kuwo.cn/playlist_detail/" + id
		}
		return "http://www.kuwo.cn/play_detail/" + id
	case "migu":
		if typeStr == "song" {
			return "https://music.migu.cn/v3/music/song/" + id
		}
	case "bilibili":
		return "https://www.bilibili.com/video/" + id
	case "fivesing":
		if strings.Contains(id, "/") {
			return "http://5sing.kugou.com/" + id + ".html"
		}
	}
	return ""
}

// --- Main ---
func Start(port string, shouldOpenBrowser bool) {
	cm.Load()
	initDB()
	defer db.Close()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.Use(corsMiddleware())

	tmpl := template.Must(template.New("").ParseFS(templateFS, "templates/*.html"))
	r.SetHTMLTemplate(tmpl)

	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, RoutePrefix)
	})

	videoDir := "video_output"
	os.MkdirAll(videoDir, 0755)
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			cleanupOldFiles(videoDir, 10*time.Minute)
		}
	}()
	r.Static(RoutePrefix+"/videos", videoDir)

	api := r.Group(RoutePrefix)

	api.GET("/icon.png", func(c *gin.Context) { c.FileFromFS("templates/icon.png", http.FS(templateFS)) })
	api.GET("/videogen.js", func(c *gin.Context) { c.FileFromFS("templates/videogen.js", http.FS(templateFS)) })
	api.GET("/app.js", func(c *gin.Context) { c.FileFromFS("templates/app.js", http.FS(templateFS)) })
	
	api.GET("/render", func(c *gin.Context) {
		c.HTML(200, "render.html", gin.H{"Root": RoutePrefix})
	})

	api.GET("/cookies", func(c *gin.Context) { c.JSON(200, cm.cookies) })
	api.POST("/cookies", func(c *gin.Context) {
		var req map[string]string
		if c.ShouldBindJSON(&req) == nil {
			cm.SetAll(req)
			cm.Save()
			c.JSON(200, gin.H{"status": "ok"})
		}
	})

	api.GET("/", func(c *gin.Context) {
		renderIndex(c, nil, nil, "", nil, "", "song", "")
	})

	api.GET("/recommend", func(c *gin.Context) {
		sources := c.QueryArray("sources")
		if len(sources) == 0 {
			sources = []string{"netease", "qq", "kugou", "kuwo"}
		}

		var allPlaylists []model.Playlist
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, src := range sources {
			fn := getRecommendFunc(src)
			if fn == nil {
				continue
			}
			wg.Add(1)
			go func(s string) {
				defer wg.Done()
				res, err := fn()
				if err == nil && len(res) > 0 {
					mu.Lock()
					allPlaylists = append(allPlaylists, res...)
					mu.Unlock()
				}
			}(src)
		}
		wg.Wait()

		renderIndex(c, nil, allPlaylists, "🔥 每日推荐", sources, "", "playlist", "")
	})

	api.GET("/search", func(c *gin.Context) {
		keyword := strings.TrimSpace(c.Query("q"))
		searchType := c.DefaultQuery("type", "song")
		sources := c.QueryArray("sources")

		if len(sources) == 0 {
			if searchType == "playlist" {
				sources = core.GetPlaylistSourceNames()
			} else {
				sources = core.GetDefaultSourceNames()
			}
		}

		var allSongs []model.Song
		var allPlaylists []model.Playlist
		var errorMsg string

		if strings.HasPrefix(keyword, "http") {
			src := detectSource(keyword)
			if src == "" {
				errorMsg = "不支持该链接的解析，或无法识别来源"
			} else {
				parsed := false
				parseFn := getParseFunc(src)
				if parseFn != nil {
					if song, err := parseFn(keyword); err == nil {
						allSongs = append(allSongs, *song)
						searchType = "song"
						parsed = true
					}
				}
				if !parsed {
					parsePlaylistFn := getParsePlaylistFunc(src)
					if parsePlaylistFn != nil {
						if playlist, songs, err := parsePlaylistFn(keyword); err == nil {
							if searchType == "playlist" {
								allPlaylists = append(allPlaylists, *playlist)
							} else {
								allSongs = append(allSongs, songs...)
								searchType = "song"
							}
							parsed = true
						}
					}
				}
				if !parsed {
					errorMsg = fmt.Sprintf("解析失败: 暂不支持 %s 平台的此链接类型或解析出错", src)
				}
			}
		} else {
			var wg sync.WaitGroup
			var mu sync.Mutex

			for _, src := range sources {
				wg.Add(1)
				go func(s string) {
					defer wg.Done()
					if searchType == "playlist" {
						fn := getPlaylistSearchFunc(s)
						if fn != nil {
							res, err := fn(keyword)
							if err == nil {
								mu.Lock()
								allPlaylists = append(allPlaylists, res...)
								mu.Unlock()
							}
						}
					} else {
						fn := getSearchFunc(s)
						if fn != nil {
							res, err := fn(keyword)
							if err == nil {
								for i := range res {
									res[i].Source = s
								}
								mu.Lock()
								allSongs = append(allSongs, res...)
								mu.Unlock()
							}
						}
					}
				}(src)
			}
			wg.Wait()
		}

		renderIndex(c, allSongs, allPlaylists, keyword, sources, errorMsg, searchType, "")
	})

	api.GET("/playlist", func(c *gin.Context) {
		id := c.Query("id")
		src := c.Query("source")
		if id == "" || src == "" {
			renderIndex(c, nil, nil, "", nil, "缺少参数", "song", "")
			return
		}
		fn := getPlaylistDetailFunc(src)
		if fn == nil {
			renderIndex(c, nil, nil, "", nil, "该源不支持查看歌单详情", "song", "")
			return
		}
		songs, err := fn(id)
		errMsg := ""
		if err != nil {
			errMsg = fmt.Sprintf("获取歌单失败: %v", err)
		}
		playlistLink := getOriginalLink(src, id, "playlist")
		renderIndex(c, songs, nil, "", []string{src}, errMsg, "song", playlistLink)
	})

	api.GET("/inspect", func(c *gin.Context) {
		id := c.Query("id")
		src := c.Query("source")
		durStr := c.Query("duration")

		var urlStr string
		var err error

		if src == "soda" {
			cookie := cm.Get("soda")
			sodaInst := soda.New(cookie)
			info, sErr := sodaInst.GetDownloadInfo(&model.Song{ID: id, Source: src})
			if sErr != nil {
				c.JSON(200, gin.H{"valid": false})
				return
			}
			urlStr = info.URL
		} else {
			fn := getDownloadFunc(src)
			if fn == nil {
				c.JSON(200, gin.H{"valid": false})
				return
			}
			urlStr, err = fn(&model.Song{ID: id, Source: src})
			if err != nil || urlStr == "" {
				c.JSON(200, gin.H{"valid": false})
				return
			}
		}

		req, reqErr := buildSourceRequest("GET", urlStr, src, "bytes=0-1")
		if reqErr != nil {
			c.JSON(200, gin.H{"valid": false})
			return
		}

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)

		valid := false
		var size int64 = 0

		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 || resp.StatusCode == 206 {
				valid = true
				cr := resp.Header.Get("Content-Range")
				if parts := strings.Split(cr, "/"); len(parts) == 2 {
					size, _ = strconv.ParseInt(parts[1], 10, 64)
				} else {
					size = resp.ContentLength
				}
			}
		}

		bitrate := "-"
		if valid && size > 0 {
			dur, _ := strconv.Atoi(durStr)
			if dur > 0 {
				kbps := int((size * 8) / int64(dur) / 1000)
				bitrate = fmt.Sprintf("%d kbps", kbps)
			}
		}

		c.JSON(200, gin.H{
			"valid":   valid,
			"url":     urlStr,
			"size":    formatSize(size),
			"bitrate": bitrate,
		})
	})

	api.GET("/switch_source", func(c *gin.Context) {
		name := strings.TrimSpace(c.Query("name"))
		artist := strings.TrimSpace(c.Query("artist"))
		current := strings.TrimSpace(c.Query("source"))
		target := strings.TrimSpace(c.Query("target"))
		durationStr := strings.TrimSpace(c.Query("duration"))

		origDuration, _ := strconv.Atoi(durationStr)

		if name == "" {
			c.JSON(400, gin.H{"error": "missing name"})
			return
		}

		keyword := name
		if artist != "" {
			keyword = name + " " + artist
		}

		var sources []string
		if target != "" {
			sources = []string{target}
		} else {
			sources = core.GetAllSourceNames()
		}

		type candidate struct {
			song    model.Song
			score   float64
			durDiff int
		}
		var wg sync.WaitGroup
		var mu sync.Mutex
		var candidates []candidate

		for _, src := range sources {
			if src == "" || src == current {
				continue
			}
			if src == "soda" || src == "fivesing" {
				continue
			}
			fn := getSearchFunc(src)
			if fn == nil {
				continue
			}

			wg.Add(1)
			go func(s string) {
				defer wg.Done()

				res, err := fn(keyword)
				if (err != nil || len(res) == 0) && artist != "" {
					res, _ = fn(name)
				}
				if len(res) == 0 {
					return
				}

				limit := len(res)
				if limit > 8 {
					limit = 8
				}

				for i := 0; i < limit; i++ {
					cand := res[i]
					cand.Source = s
					score := calcSongSimilarity(name, artist, cand.Name, cand.Artist)
					if score <= 0 {
						continue
					}

					durDiff := 0
					if origDuration > 0 && cand.Duration > 0 {
						durDiff = intAbs(origDuration - cand.Duration)
						if !isDurationClose(origDuration, cand.Duration) {
							continue
						}
					}

					mu.Lock()
					candidates = append(candidates, candidate{song: cand, score: score, durDiff: durDiff})
					mu.Unlock()
				}
			}(src)
		}

		wg.Wait()
		if len(candidates) == 0 {
			c.JSON(404, gin.H{"error": "no match"})
			return
		}

		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].score == candidates[j].score {
				return candidates[i].durDiff < candidates[j].durDiff
			}
			return candidates[i].score > candidates[j].score
		})

		var selected *model.Song
		var selectedScore float64
		for _, cand := range candidates {
			ok := validatePlayable(&cand.song)
			if ok {
				tmp := cand.song
				selected = &tmp
				selectedScore = cand.score
				break
			}
		}
		if selected == nil {
			c.JSON(404, gin.H{"error": "no playable match"})
			return
		}

		c.JSON(200, gin.H{
			"id":       selected.ID,
			"name":     selected.Name,
			"artist":   selected.Artist,
			"album":    selected.Album,
			"duration": selected.Duration,
			"source":   selected.Source,
			"cover":    selected.Cover,
			"score":    selectedScore,
			"link":     selected.Link,
		})
	})

	api.GET("/download", func(c *gin.Context) {
		id := c.Query("id")
		source := c.Query("source")
		name := c.Query("name")
		artist := c.Query("artist")

		if id == "" || source == "" {
			c.String(400, "Missing params")
			return
		}
		if name == "" {
			name = "Unknown"
		}
		if artist == "" {
			artist = "Unknown"
		}

		tempSong := &model.Song{ID: id, Source: source, Name: name, Artist: artist}
		filename := fmt.Sprintf("%s - %s.mp3", artist, name)

		if source == "soda" {
			cookie := cm.Get("soda")
			sodaInst := soda.New(cookie)
			info, err := sodaInst.GetDownloadInfo(tempSong)
			if err != nil {
				c.String(502, "Soda info error")
				return
			}
			req, reqErr := buildSourceRequest("GET", info.URL, "soda", "")
			if reqErr != nil {
				c.String(502, "Soda request error")
				return
			}
			resp, err := (&http.Client{}).Do(req)
			if err != nil {
				c.String(502, "Soda stream error")
				return
			}
			defer resp.Body.Close()
			encryptedData, _ := io.ReadAll(resp.Body)
			finalData, err := soda.DecryptAudio(encryptedData, info.PlayAuth)
			if err != nil {
				c.String(500, "Decrypt failed")
				return
			}
			setDownloadHeader(c, filename)
			http.ServeContent(c.Writer, c.Request, filename, time.Now(), bytes.NewReader(finalData))
			return
		}

		dlFunc := getDownloadFunc(source)
		if dlFunc == nil {
			c.String(400, "Unknown source")
			return
		}

		downloadUrl, err := dlFunc(tempSong)
		if err != nil {
			c.String(404, "Failed to get URL")
			return
		}

		req, reqErr := buildSourceRequest("GET", downloadUrl, source, c.GetHeader("Range"))
		if reqErr != nil {
			c.String(502, "Upstream request error")
			return
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			c.String(502, "Upstream stream error")
			return
		}
		defer resp.Body.Close()

		for k, v := range resp.Header {
			if k != "Transfer-Encoding" && k != "Date" && k != "Access-Control-Allow-Origin" {
				c.Writer.Header()[k] = v
			}
		}

		setDownloadHeader(c, filename)
		c.Status(resp.StatusCode)
		io.Copy(c.Writer, resp.Body)
	})

	api.GET("/download_lrc", func(c *gin.Context) {
		id := c.Query("id")
		src := c.Query("source")
		name := c.Query("name")
		artist := c.Query("artist")

		fn := getLyricFunc(src)
		if fn == nil {
			c.String(404, "No support")
			return
		}

		lrc, err := fn(&model.Song{ID: id, Source: src})
		if err != nil || lrc == "" {
			c.String(404, "Lyric not found")
			return
		}

		filename := fmt.Sprintf("%s - %s.lrc", artist, name)
		setDownloadHeader(c, filename)
		c.String(200, lrc)
	})

	api.GET("/download_cover", func(c *gin.Context) {
		u := c.Query("url")
		if u == "" {
			return
		}
		resp, err := utils.Get(u, utils.WithHeader("User-Agent", UA_Common))
		if err == nil {
			filename := fmt.Sprintf("%s - %s.jpg", c.Query("artist"), c.Query("name"))
			setDownloadHeader(c, filename)
			c.Data(200, "image/jpeg", resp)
		}
	})

	api.GET("/lyric", func(c *gin.Context) {
		id := c.Query("id")
		src := c.Query("source")
		fn := getLyricFunc(src)
		if fn != nil {
			lrc, _ := fn(&model.Song{ID: id, Source: src})
			if lrc != "" {
				c.String(200, lrc)
				return
			}
		}
		c.String(200, "[00:00.00] 暂无歌词")
	})

	// ==========================================
	// 收藏夹系统 API 路由
	// ==========================================
	colApi := api.Group("/collections")

	// 获取所有收藏夹
	colApi.GET("", func(c *gin.Context) {
		rows, err := db.Query("SELECT id, name, description, created_at FROM collections ORDER BY id DESC")
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var cols []map[string]interface{}
		for rows.Next() {
			var id int
			var name, desc, createdAt string
			rows.Scan(&id, &name, &desc, &createdAt)
			cols = append(cols, gin.H{"id": id, "name": name, "description": desc, "created_at": createdAt})
		}
		c.JSON(200, cols)
	})

	// 创建收藏夹
	colApi.POST("", func(c *gin.Context) {
		var req struct {
			Name        string `json:"name" binding:"required"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "参数错误"})
			return
		}

		res, err := db.Exec("INSERT INTO collections (name, description) VALUES (?, ?)", req.Name, req.Description)
		if err != nil {
			c.JSON(500, gin.H{"error": "创建失败: " + err.Error()})
			return
		}
		id, _ := res.LastInsertId()
		c.JSON(200, gin.H{"id": id, "name": req.Name, "description": req.Description})
	})

	// 更新收藏夹
	colApi.PUT("/:id", func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Name        string `json:"name" binding:"required"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "参数错误"})
			return
		}

		_, err := db.Exec("UPDATE collections SET name = ?, description = ? WHERE id = ?", req.Name, req.Description, id)
		if err != nil {
			c.JSON(500, gin.H{"error": "更新失败"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 删除收藏夹
	colApi.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")
		_, err := db.Exec("DELETE FROM collections WHERE id = ?", id)
		if err != nil {
			c.JSON(500, gin.H{"error": "删除失败"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 获取收藏夹内所有歌曲
	colApi.GET("/:id/songs", func(c *gin.Context) {
		colID := c.Param("id")
		rows, err := db.Query(`
			SELECT id, song_id, source, extra, name, artist, cover, duration, added_at 
			FROM saved_songs WHERE collection_id = ? ORDER BY id DESC`, colID)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var songs []map[string]interface{}
		for rows.Next() {
			var id, duration int
			var songID, source, extra, name, artist, cover, addedAt string
			rows.Scan(&id, &songID, &source, &extra, &name, &artist, &cover, &duration, &addedAt)
			
			// 将存入的额外信息字符串尝试转回 JSON 对象返回，若失败则原样返回
			var extraObj interface{}
			if err := json.Unmarshal([]byte(extra), &extraObj); err != nil {
				extraObj = extra
			}

			songs = append(songs, gin.H{
				"db_id":    id,
				"id":       songID,
				"source":   source,
				"extra":    extraObj,
				"name":     name,
				"artist":   artist,
				"cover":    cover,
				"duration": duration,
				"added_at": addedAt,
			})
		}
		c.JSON(200, songs)
	})

	// 收藏单曲到指定的收藏夹
	colApi.POST("/:id/songs", func(c *gin.Context) {
		colID := c.Param("id")
		var req struct {
			SongID   string      `json:"id" binding:"required"`
			Source   string      `json:"source" binding:"required"`
			Name     string      `json:"name"`
			Artist   string      `json:"artist"`
			Cover    string      `json:"cover"`
			Duration int         `json:"duration"`
			Extra    interface{} `json:"extra"` // 接受任意格式并转存为字符串
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "参数错误，缺失id或source"})
			return
		}

		extraStr := ""
		if req.Extra != nil {
			b, _ := json.Marshal(req.Extra)
			extraStr = string(b)
		}

		_, err := db.Exec(`
			INSERT OR IGNORE INTO saved_songs 
			(collection_id, song_id, source, extra, name, artist, cover, duration) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			colID, req.SongID, req.Source, extraStr, req.Name, req.Artist, req.Cover, req.Duration,
		)

		if err != nil {
			c.JSON(500, gin.H{"error": "添加失败: " + err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 从收藏夹中删除单曲
	colApi.DELETE("/:id/songs", func(c *gin.Context) {
		colID := c.Param("id")
		songID := c.Query("id")
		source := c.Query("source")

		if songID == "" || source == "" {
			c.JSON(400, gin.H{"error": "需要通过 query 传递 id 和 source"})
			return
		}

		_, err := db.Exec("DELETE FROM saved_songs WHERE collection_id = ? AND song_id = ? AND source = ?", colID, songID, source)
		if err != nil {
			c.JSON(500, gin.H{"error": "删除失败"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})


	// --- 视频生成模块 API 路由集成 ---
	videoApi := api.Group("/videogen")

	// 1. 初始化渲染会话 (已支持前端本地上传文件直传)
	videoApi.POST("/init", func(c *gin.Context) {
		var id, source string
		var hasCustomAudio bool

		// 判断请求格式：如果带有本地音频文件，则是 FormData
		if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
			id = c.PostForm("id")
			source = c.PostForm("source")
			hasCustomAudio = true
		} else {
			// 如果没有传本地文件，还是走原来的 JSON 解析逻辑
			var req struct {
				ID     string `json:"id"`
				Source string `json:"source"`
			}
			if c.ShouldBindJSON(&req) != nil {
				c.JSON(400, gin.H{"error": "Args error"})
				return
			}
			id = req.ID
			source = req.Source
		}

		sessionID := fmt.Sprintf("%s_%s_%d", source, id, time.Now().Unix())
		tempDir, _ := os.MkdirTemp("", "vg_render_"+sessionID+"_*")
		audioPath := filepath.Join(tempDir, "audio.mp3")

		var proxyAudioUrl string

		if hasCustomAudio {
			// 接收前端传过来的自定义本地音频文件，并直接保存到服务器作为底层音轨
			file, err := c.FormFile("audio_file")
			if err != nil {
				c.JSON(400, gin.H{"error": "Failed to receive custom audio"})
				return
			}
			if err := c.SaveUploadedFile(file, audioPath); err != nil {
				c.JSON(500, gin.H{"error": "Failed to save custom audio"})
				return
			}
			// 既然本地已有，前端不需要代理链接去下载了
			proxyAudioUrl = "" 
		} else {
			// 原版逻辑：通过 ID 去云端源站扒原始音乐
			fn := getDownloadFunc(source)
			if fn == nil {
				c.JSON(500, gin.H{"error": "Source not supported"})
				return
			}
			audioUrl, err := fn(&model.Song{ID: id, Source: source})
			if err != nil {
				c.JSON(500, gin.H{"error": "Audio download failed"})
				return
			}

			reqHttp, _ := buildSourceRequest("GET", audioUrl, source, "")
			client := &http.Client{}
			resp, err := client.Do(reqHttp)
			if err != nil {
				c.JSON(500, gin.H{"error": "Save audio failed"})
				return
			}
			defer resp.Body.Close()
			out, _ := os.Create(audioPath)
			io.Copy(out, resp.Body)
			out.Close()

			proxyAudioUrl = fmt.Sprintf("%s/download?id=%s&source=%s", RoutePrefix, url.QueryEscape(id), source)
		}

		sess := &RenderSession{
			ID:        sessionID,
			Dir:       tempDir,
			AudioPath: audioPath,
		}

		sessMu.Lock()
		sessions[sessionID] = sess
		sessMu.Unlock()

		c.JSON(200, gin.H{"session_id": sessionID, "audio_url": proxyAudioUrl})
	})

	videoApi.POST("/frame", func(c *gin.Context) {
		var req struct {
			SessionID string   `json:"session_id"`
			Frames    []string `json:"frames"`
			StartIdx  int      `json:"start_idx"`
		}
		if c.ShouldBindJSON(&req) != nil {
			c.JSON(400, gin.H{"error": "Bad request"})
			return
		}

		sessMu.Lock()
		sess, ok := sessions[req.SessionID]
		sessMu.Unlock()
		if !ok {
			c.JSON(404, gin.H{"error": "Session not found"})
			return
		}

		sess.Mutex.Lock()
		defer sess.Mutex.Unlock()

		for i, dataURI := range req.Frames {
			frameNum := req.StartIdx + i
			fileName := filepath.Join(sess.Dir, fmt.Sprintf("frame_%05d.jpg", frameNum))
			saveBase64(dataURI, fileName)
		}
		sess.Total += len(req.Frames)

		c.JSON(200, gin.H{"status": "ok", "received": len(req.Frames)})
	})

	videoApi.POST("/finish", func(c *gin.Context) {
		var req struct {
			SessionID string `json:"session_id"`
			Name      string `json:"name"`
		}
		c.ShouldBindJSON(&req)

		sessMu.Lock()
		sess, ok := sessions[req.SessionID]
		delete(sessions, req.SessionID)
		sessMu.Unlock()

		if !ok {
			c.JSON(404, gin.H{"error": "Session not found"})
			return
		}

		absVideoDir, _ := filepath.Abs(videoDir)
		outName := fmt.Sprintf("render_%s_%d.mp4", sess.ID, time.Now().Unix())
		outPath := filepath.Join(absVideoDir, outName)

		cmd := exec.Command("ffmpeg",
			"-y",
			"-framerate", "30",
			"-i", filepath.Join(sess.Dir, "frame_%05d.jpg"),
			"-i", sess.AudioPath,
			"-c:v", "libx264",
			"-preset", "ultrafast",
			"-c:a", "aac",
			"-b:a", "320k",
			"-pix_fmt", "yuv420p",
			"-shortest",
			outPath,
		)

		output, err := cmd.CombinedOutput()
		os.RemoveAll(sess.Dir) 

		if err != nil {
			fmt.Println("FFmpeg Error:", string(output))
			c.JSON(500, gin.H{"error": "Render failed: " + err.Error()})
			return
		}

		c.JSON(200, gin.H{"url": "/videos/" + outName})
	})

	urlStr := "http://localhost:" + port + RoutePrefix
	fmt.Printf("Web started at %s\n", urlStr)
	if shouldOpenBrowser {
		go func() { time.Sleep(500 * time.Millisecond); openBrowser(urlStr) }()
	}
	r.Run(":" + port)
}

func renderIndex(c *gin.Context, songs []model.Song, playlists []model.Playlist, q string, selected []string, errMsg string, searchType string, playlistLink string) {
	allSrc := core.GetAllSourceNames()
	desc := make(map[string]string)
	for _, s := range allSrc {
		desc[s] = core.GetSourceDescription(s)
	}

	playlistSupported := make(map[string]bool)
	for _, s := range core.GetPlaylistSourceNames() {
		playlistSupported[s] = true
	}

	c.HTML(200, "index.html", gin.H{
		"Result":             songs,
		"Playlists":          playlists,
		"Keyword":            q,
		"AllSources":         allSrc,
		"DefaultSources":     core.GetDefaultSourceNames(),
		"SourceDescriptions": desc,
		"Selected":           selected,
		"Error":              errMsg,
		"SearchType":         searchType,
		"PlaylistSupported":  playlistSupported,
		"Root":               RoutePrefix,
		"PlaylistLink":       playlistLink,
	})
}

func formatSize(s int64) string {
	if s <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f MB", float64(s)/1024/1024)
}

func setDownloadHeader(c *gin.Context, filename string) {
	encoded := url.QueryEscape(filename)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=utf-8''%s", encoded, encoded))
}

func validatePlayable(song *model.Song) bool {
	if song == nil || song.ID == "" || song.Source == "" {
		return false
	}
	if song.Source == "soda" || song.Source == "fivesing" {
		return false
	}

	fn := getDownloadFunc(song.Source)
	if fn == nil {
		return false
	}
	urlStr, err := fn(&model.Song{ID: song.ID, Source: song.Source})
	if err != nil || urlStr == "" {
		return false
	}

	req, reqErr := buildSourceRequest("GET", urlStr, song.Source, "bytes=0-1")
	if reqErr != nil {
		return false
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200 || resp.StatusCode == 206
}

func isDurationClose(a, b int) bool {
	if a <= 0 || b <= 0 {
		return true
	}
	diff := intAbs(a - b)
	if diff <= 10 {
		return true
	}
	maxAllowed := int(float64(a) * 0.15)
	if maxAllowed < 10 {
		maxAllowed = 10
	}
	return diff <= maxAllowed
}

func intAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func calcSongSimilarity(name, artist, candName, candArtist string) float64 {
	nameA := normalizeText(name)
	nameB := normalizeText(candName)
	if nameA == "" || nameB == "" {
		return 0
	}
	nameSim := similarityScore(nameA, nameB)

	artistA := normalizeText(artist)
	artistB := normalizeText(candArtist)
	if artistA == "" || artistB == "" {
		return nameSim
	}

	artistSim := similarityScore(artistA, artistB)
	return nameSim*0.7 + artistSim*0.3
}

func normalizeText(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.In(r, unicode.Han) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func similarityScore(a, b string) float64 {
	if a == b {
		return 1
	}
	if a == "" || b == "" {
		return 0
	}
	la := len([]rune(a))
	lb := len([]rune(b))
	maxLen := la
	if lb > maxLen {
		maxLen = lb
	}
	if maxLen == 0 {
		return 0
	}
	dist := levenshteinDistance(a, b)
	if dist >= maxLen {
		return 0
	}
	return 1 - float64(dist)/float64(maxLen)
}

func levenshteinDistance(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	la := len(ra)
	lb := len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 0
			if ra[i-1] != rb[j-1] {
				cost = 1
			}
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			cur[j] = del
			if ins < cur[j] {
				cur[j] = ins
			}
			if sub < cur[j] {
				cur[j] = sub
			}
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd, args = "cmd", []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	_ = exec.Command(cmd, args...).Start()
}