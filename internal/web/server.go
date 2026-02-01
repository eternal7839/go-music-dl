package web

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

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
)

//go:embed templates/*
var templateFS embed.FS

const (
	UA_Common    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
	UA_Mobile    = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	Ref_Bilibili = "https://www.bilibili.com/"
	Ref_Migu     = "http://music.migu.cn/"
	CookieFile   = "cookies.json"
)

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

// --- 工厂函数 ---

func getSearchFunc(source string) func(string) ([]model.Song, error) {
	c := cm.Get(source)
	switch source {
	case "netease": return netease.New(c).Search
	case "qq": return qq.New(c).Search
	case "kugou": return kugou.New(c).Search
	case "kuwo": return kuwo.New(c).Search
	case "migu": return migu.New(c).Search
	case "soda": return soda.New(c).Search
	case "bilibili": return bilibili.New(c).Search
	case "fivesing": return fivesing.New(c).Search
	case "jamendo": return jamendo.New(c).Search
	case "joox": return joox.New(c).Search
	case "qianqian": return qianqian.New(c).Search
	default: return nil
	}
}

// [新增] 歌单搜索工厂
func getPlaylistSearchFunc(source string) func(string) ([]model.Playlist, error) {
	c := cm.Get(source)
	switch source {
	case "netease": return netease.New(c).SearchPlaylist
	case "qq": return qq.New(c).SearchPlaylist
	case "kugou": return kugou.New(c).SearchPlaylist
	case "kuwo": return kuwo.New(c).SearchPlaylist
	case "soda": return soda.New(c).SearchPlaylist
	case "fivesing": return fivesing.New(c).SearchPlaylist
	default: return nil
	}
}

// [新增] 歌单详情工厂
func getPlaylistDetailFunc(source string) func(string) ([]model.Song, error) {
	c := cm.Get(source)
	switch source {
	case "netease": return netease.New(c).GetPlaylistSongs
	case "qq": return qq.New(c).GetPlaylistSongs
	case "kugou": return kugou.New(c).GetPlaylistSongs
	case "kuwo": return kuwo.New(c).GetPlaylistSongs
	case "soda": return soda.New(c).GetPlaylistSongs
	case "fivesing": return fivesing.New(c).GetPlaylistSongs
	default: return nil
	}
}

func getDownloadFunc(source string) func(*model.Song) (string, error) {
	c := cm.Get(source)
	switch source {
	case "netease": return netease.New(c).GetDownloadURL
	case "qq": return qq.New(c).GetDownloadURL
	case "kugou": return kugou.New(c).GetDownloadURL
	case "kuwo": return kuwo.New(c).GetDownloadURL
	case "migu": return migu.New(c).GetDownloadURL
	case "soda": return soda.New(c).GetDownloadURL
	case "bilibili": return bilibili.New(c).GetDownloadURL
	case "fivesing": return fivesing.New(c).GetDownloadURL
	case "jamendo": return jamendo.New(c).GetDownloadURL
	case "joox": return joox.New(c).GetDownloadURL
	case "qianqian": return qianqian.New(c).GetDownloadURL
	default: return nil
	}
}

func getLyricFunc(source string) func(*model.Song) (string, error) {
	c := cm.Get(source)
	switch source {
	case "netease": return netease.New(c).GetLyrics
	case "qq": return qq.New(c).GetLyrics
	case "kugou": return kugou.New(c).GetLyrics
	case "kuwo": return kuwo.New(c).GetLyrics
	case "migu": return migu.New(c).GetLyrics
	case "soda": return soda.New(c).GetLyrics
	case "bilibili": return bilibili.New(c).GetLyrics
	case "fivesing": return fivesing.New(c).GetLyrics
	case "jamendo": return jamendo.New(c).GetLyrics
	case "joox": return joox.New(c).GetLyrics
	case "qianqian": return qianqian.New(c).GetLyrics
	default: return nil
	}
}

func getParseFunc(source string) func(string) (*model.Song, error) {
	c := cm.Get(source)
	switch source {
	case "netease": return netease.New(c).Parse
	case "qq": return qq.New(c).Parse
	case "kugou": return kugou.New(c).Parse
	case "kuwo": return kuwo.New(c).Parse
	case "migu": return migu.New(c).Parse
	case "soda": return soda.New(c).Parse
	case "bilibili": return bilibili.New(c).Parse
	case "fivesing": return fivesing.New(c).Parse
	case "jamendo": return jamendo.New(c).Parse
	default: return nil
	}
}

func detectSource(link string) string {
	if strings.Contains(link, "163.com") { return "netease" }
	if strings.Contains(link, "qq.com") { return "qq" }
	if strings.Contains(link, "kugou.com") { return "kugou" }
	if strings.Contains(link, "kuwo.cn") { return "kuwo" }
	if strings.Contains(link, "migu.cn") { return "migu" }
	if strings.Contains(link, "bilibili.com") || strings.Contains(link, "b23.tv") { return "bilibili" }
	if strings.Contains(link, "douyin.com") || strings.Contains(link, "qishui") { return "soda" } 
	if strings.Contains(link, "5sing") { return "fivesing" }
	if strings.Contains(link, "jamendo.com") { return "jamendo" }
	return ""
}

// --- Main ---

func Start(port string) {
	cm.Load()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	tmpl := template.Must(template.New("").ParseFS(templateFS, "templates/*.html"))
	r.SetHTMLTemplate(tmpl)

	r.GET("/icon.png", func(c *gin.Context) { c.FileFromFS("templates/icon.png", http.FS(templateFS)) })

	r.GET("/cookies", func(c *gin.Context) { c.JSON(200, cm.cookies) })
	r.POST("/cookies", func(c *gin.Context) {
		var req map[string]string
		if c.ShouldBindJSON(&req) == nil {
			cm.SetAll(req)
			cm.Save()
			c.JSON(200, gin.H{"status": "ok"})
		}
	})

	r.GET("/", func(c *gin.Context) {
		renderIndex(c, nil, nil, "", nil, "", "song")
	})

	// Search (Song & Playlist)
	r.GET("/search", func(c *gin.Context) {
		keyword := strings.TrimSpace(c.Query("q"))
		searchType := c.DefaultQuery("type", "song") // song or playlist
		sources := c.QueryArray("sources")
		
		// 默认源逻辑
		if len(sources) == 0 { 
			if searchType == "playlist" {
				sources = core.GetPlaylistSourceNames() // 仅返回支持歌单的源
			} else {
				sources = core.GetDefaultSourceNames() 
			}
		}

		var allSongs []model.Song
		var allPlaylists []model.Playlist
		var errorMsg string

		// 1. 链接解析模式 (仅支持单曲)
		if strings.HasPrefix(keyword, "http") {
			src := detectSource(keyword)
			if src == "" {
				errorMsg = "不支持该链接的解析，或无法识别来源"
			} else {
				parseFn := getParseFunc(src)
				if parseFn == nil {
					errorMsg = fmt.Sprintf("暂不支持 %s 平台的链接解析", src)
				} else {
					song, err := parseFn(keyword)
					if err != nil {
						errorMsg = fmt.Sprintf("解析失败: %v", err)
					} else {
						allSongs = append(allSongs, *song)
					}
				}
			}
			// 强制为单曲模式展示结果
			searchType = "song"
		} else {
			// 2. 关键词搜索模式
			var wg sync.WaitGroup
			var mu sync.Mutex

			for _, src := range sources {
				wg.Add(1)
				go func(s string) {
					defer wg.Done()
					
					if searchType == "playlist" {
						// 歌单搜索
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
						// 单曲搜索
						fn := getSearchFunc(s)
						if fn != nil {
							res, err := fn(keyword)
							if err == nil {
								for i := range res { res[i].Source = s }
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
		
		renderIndex(c, allSongs, allPlaylists, keyword, sources, errorMsg, searchType)
	})

	// [新增] 获取歌单详情并渲染
	r.GET("/playlist", func(c *gin.Context) {
		id := c.Query("id")
		src := c.Query("source")
		if id == "" || src == "" {
			renderIndex(c, nil, nil, "", nil, "缺少参数", "song")
			return
		}

		fn := getPlaylistDetailFunc(src)
		if fn == nil {
			renderIndex(c, nil, nil, "", nil, "该源不支持查看歌单详情", "song")
			return
		}

		songs, err := fn(id)
		errMsg := ""
		if err != nil {
			errMsg = fmt.Sprintf("获取歌单失败: %v", err)
		}
		
		// 渲染为单曲列表模式，但保留上下文
		renderIndex(c, songs, nil, "", []string{src}, errMsg, "song")
	})

	// Inspect
	r.GET("/inspect", func(c *gin.Context) {
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

		req, _ := http.NewRequest("GET", urlStr, nil)
		req.Header.Set("Range", "bytes=0-1") 
		req.Header.Set("User-Agent", UA_Common)
		if src == "bilibili" { req.Header.Set("Referer", Ref_Bilibili) }
		if src == "migu" { 
			req.Header.Set("User-Agent", UA_Mobile)
			req.Header.Set("Referer", Ref_Migu) 
		}
		if src == "qq" { req.Header.Set("Referer", "http://y.qq.com") }

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

	// Download Logic (Same as before)
	r.GET("/download", func(c *gin.Context) {
		id := c.Query("id")
		source := c.Query("source")
		name := c.Query("name")
		artist := c.Query("artist")

		if id == "" || source == "" {
			c.String(400, "Missing params")
			return
		}
		if name == "" { name = "Unknown" }
		if artist == "" { artist = "Unknown" }

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
			req, _ := http.NewRequest("GET", info.URL, nil)
			req.Header.Set("User-Agent", UA_Common)
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

		req, _ := http.NewRequest("GET", downloadUrl, nil)
		if rangeHeader := c.GetHeader("Range"); rangeHeader != "" {
			req.Header.Set("Range", rangeHeader)
		}

		req.Header.Set("User-Agent", UA_Common)
		if source == "bilibili" { req.Header.Set("Referer", Ref_Bilibili) }
		if source == "migu" { 
			req.Header.Set("User-Agent", UA_Mobile)
			req.Header.Set("Referer", Ref_Migu) 
		}
		if source == "qq" { req.Header.Set("Referer", "http://y.qq.com") }

		client := &http.Client{} 
		resp, err := client.Do(req)
		if err != nil {
			c.String(502, "Upstream stream error")
			return
		}
		defer resp.Body.Close()

		for k, v := range resp.Header {
			if k != "Transfer-Encoding" && k != "Date" {
				c.Writer.Header()[k] = v
			}
		}

		setDownloadHeader(c, filename)
		c.Status(resp.StatusCode)
		io.Copy(c.Writer, resp.Body)
	})

	r.GET("/download_lrc", func(c *gin.Context) {
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

	r.GET("/download_cover", func(c *gin.Context) {
		u := c.Query("url")
		if u == "" { return }
		resp, err := utils.Get(u, utils.WithHeader("User-Agent", UA_Common))
		if err == nil {
			filename := fmt.Sprintf("%s - %s.jpg", c.Query("artist"), c.Query("name"))
			setDownloadHeader(c, filename)
			c.Data(200, "image/jpeg", resp)
		}
	})

	r.GET("/lyric", func(c *gin.Context) {
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

	urlStr := "http://localhost:" + port
	fmt.Printf("Web started at %s\n", urlStr)
	go func() { time.Sleep(500 * time.Millisecond); openBrowser(urlStr) }()
	r.Run(":" + port)
}

func renderIndex(c *gin.Context, songs []model.Song, playlists []model.Playlist, q string, selected []string, errMsg string, searchType string) {
	allSrc := core.GetAllSourceNames()
	desc := make(map[string]string)
	for _, s := range allSrc { desc[s] = core.GetSourceDescription(s) }
	
	// 标记哪些源支持歌单
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
	})
}

func formatSize(s int64) string {
	if s <= 0 { return "-" }
	return fmt.Sprintf("%.1f MB", float64(s)/1024/1024)
}

func setDownloadHeader(c *gin.Context, filename string) {
	encoded := url.QueryEscape(filename)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=utf-8''%s", encoded, encoded))
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows": cmd, args = "cmd", []string{"/c", "start"}
	case "darwin": cmd = "open"
	default: cmd = "xdg-open"
	}
	args = append(args, url)
	_ = exec.Command(cmd, args...).Start()
}