package web

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

//go:embed templates/*
var templateFS embed.FS

const RoutePrefix = "/music"

// FeatureFlags 控制前端功能按钮的显示
type FeatureFlags struct {
	VgChangeCover bool
	VgChangeAudio bool
	VgChangeLyric bool
	VgExportVideo bool
}

var featureFlags FeatureFlags

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

func setDownloadHeader(c *gin.Context, filename string) {
	encoded := url.QueryEscape(filename)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=utf-8''%s", encoded, encoded))
}

func renderIndex(c *gin.Context, songs []model.Song, playlists []model.Playlist, q string, selected []string, errMsg string, searchType string, playlistLink string, colID string, colName string, isLocalColPage bool) {
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
		"ColID":              colID,
		"ColName":            colName,
		"IsLocalColPage":     isLocalColPage,
		"VgChangeCover":      featureFlags.VgChangeCover,
		"VgChangeAudio":      featureFlags.VgChangeAudio,
		"VgChangeLyric":      featureFlags.VgChangeLyric,
		"VgExportVideo":      featureFlags.VgExportVideo,
	})
}

func Start(port string, shouldOpenBrowser bool, flags FeatureFlags) {
	featureFlags = flags
	core.CM.Load()
	InitDB()
	defer CloseDB()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(corsMiddleware())

	tmpl := template.Must(template.New("").ParseFS(templateFS,
		"templates/pages/*.html",
		"templates/partials/*.html",
	))
	r.SetHTMLTemplate(tmpl)

	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, RoutePrefix)
	})

	videoDir := "data/video_output"
	os.MkdirAll(videoDir, 0755)

	api := r.Group(RoutePrefix)
	api.Static("/videos", videoDir)

	// 基础前端依赖路由
	api.GET("/icon.png", func(c *gin.Context) { c.FileFromFS("templates/static/images/icon.png", http.FS(templateFS)) })
	api.GET("/style.css", func(c *gin.Context) { c.FileFromFS("templates/static/css/style.css", http.FS(templateFS)) })
	api.GET("/videogen.css", func(c *gin.Context) { c.FileFromFS("templates/static/css/videogen.css", http.FS(templateFS)) })
	api.GET("/videogen.js", func(c *gin.Context) { c.FileFromFS("templates/static/js/videogen.js", http.FS(templateFS)) })
	api.GET("/app.js", func(c *gin.Context) { c.FileFromFS("templates/static/js/app.js", http.FS(templateFS)) })
	api.GET("/render", func(c *gin.Context) {
		c.HTML(200, "render.html", gin.H{
			"Root":          RoutePrefix,
			"VgExportVideo": featureFlags.VgExportVideo,
		})
	})

	api.GET("/cookies", func(c *gin.Context) { c.JSON(200, core.CM.GetAll()) })
	api.POST("/cookies", func(c *gin.Context) {
		var req map[string]string
		if c.ShouldBindJSON(&req) == nil {
			core.CM.SetAll(req)
			core.CM.Save()
			c.JSON(200, gin.H{"status": "ok"})
		}
	})

	RegisterMusicRoutes(api)
	RegisterCollectionRoutes(api)
	RegisterVideogenRoutes(api, videoDir)

	urlStr := "http://localhost:" + port + RoutePrefix
	fmt.Printf("Web started at %s\n", urlStr)
	if shouldOpenBrowser {
		go func() { time.Sleep(500 * time.Millisecond); core.OpenBrowser(urlStr) }()
	}
	r.Run(":" + port)
}
