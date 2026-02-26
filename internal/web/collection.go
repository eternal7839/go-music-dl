package web

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/music-lib/model"
	_ "modernc.org/sqlite"
)

var db *sql.DB

func InitDB() {
	var err error
	db, err = sql.Open("sqlite", "favorites.db")
	if err != nil {
		panic("Failed to connect to SQLite: " + err.Error())
	}
	db.Exec("PRAGMA foreign_keys = ON;")

	schema := `
	CREATE TABLE IF NOT EXISTS collections (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT,
		cover TEXT,
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
	db.Exec("ALTER TABLE collections ADD COLUMN cover TEXT;")
}

func RegisterCollectionRoutes(api *gin.RouterGroup) {
	api.GET("/my_collections", func(c *gin.Context) {
		rows, err := db.Query("SELECT id, name, description, cover FROM collections ORDER BY id DESC")
		if err != nil {
			renderIndex(c, nil, nil, "我的自制歌单", nil, "获取收藏夹失败", "playlist", "", "", "", true)
			return
		}
		defer rows.Close()

		var playlists []model.Playlist
		for rows.Next() {
			var id int
			var name, desc string
			var cover sql.NullString
			rows.Scan(&id, &name, &desc, &cover)

			var count int
			db.QueryRow("SELECT count(*) FROM saved_songs WHERE collection_id = ?", id).Scan(&count)

			cvr := cover.String
			if cvr == "" {
				cvr = fmt.Sprintf("https://picsum.photos/seed/col_%d/400/400", id)
			}

			playlists = append(playlists, model.Playlist{
				ID:          fmt.Sprint(id),
				Name:        name,
				Description: desc,
				Cover:       cvr,
				Creator:     "我自己",
				TrackCount:  count,
				Source:      "local",
			})
		}
		renderIndex(c, nil, playlists, "我的自制歌单", nil, "", "playlist", "", "", "", true)
	})

	api.GET("/collection", func(c *gin.Context) {
		id := c.Query("id")
		if id == "" {
			renderIndex(c, nil, nil, "", nil, "缺少参数", "song", "", "", "", false)
			return
		}

		var colName string
		err := db.QueryRow("SELECT name FROM collections WHERE id = ?", id).Scan(&colName)
		if err != nil {
			renderIndex(c, nil, nil, "", nil, "自制歌单不存在", "song", "", "", "", false)
			return
		}

		rows, err := db.Query(`
			SELECT song_id, source, name, artist, cover, duration 
			FROM saved_songs WHERE collection_id = ? ORDER BY id DESC`, id)

		var songs []model.Song
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var s model.Song
				var dur int
				rows.Scan(&s.ID, &s.Source, &s.Name, &s.Artist, &s.Cover, &dur)

				s.Duration = dur
				songs = append(songs, s)
			}
		}
		renderIndex(c, songs, nil, "", nil, "", "song", "", id, colName, false)
	})

	colApi := api.Group("/collections")

	colApi.GET("", func(c *gin.Context) {
		rows, err := db.Query("SELECT id, name, description, cover, created_at FROM collections ORDER BY id DESC")
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var cols []map[string]interface{}
		for rows.Next() {
			var id int
			var name, desc, createdAt string
			var cover sql.NullString
			rows.Scan(&id, &name, &desc, &cover, &createdAt)
			cols = append(cols, gin.H{"id": id, "name": name, "description": desc, "cover": cover.String, "created_at": createdAt})
		}
		c.JSON(200, cols)
	})

	colApi.POST("", func(c *gin.Context) {
		var req struct {
			Name        string `json:"name" binding:"required"`
			Description string `json:"description"`
			Cover       string `json:"cover"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "参数错误"})
			return
		}

		res, err := db.Exec("INSERT INTO collections (name, description, cover) VALUES (?, ?, ?)", req.Name, req.Description, req.Cover)
		if err != nil {
			c.JSON(500, gin.H{"error": "创建失败: " + err.Error()})
			return
		}
		id, _ := res.LastInsertId()
		c.JSON(200, gin.H{"id": id, "name": req.Name})
	})

	colApi.PUT("/:id", func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Name        string `json:"name" binding:"required"`
			Description string `json:"description"`
			Cover       string `json:"cover"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "参数错误"})
			return
		}

		_, err := db.Exec("UPDATE collections SET name = ?, description = ?, cover = ? WHERE id = ?", req.Name, req.Description, req.Cover, id)
		if err != nil {
			c.JSON(500, gin.H{"error": "更新失败"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	colApi.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")
		_, err := db.Exec("DELETE FROM collections WHERE id = ?", id)
		if err != nil {
			c.JSON(500, gin.H{"error": "删除失败"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	colApi.POST("/:id/songs", func(c *gin.Context) {
		colID := c.Param("id")
		var req struct {
			SongID   string      `json:"id" binding:"required"`
			Source   string      `json:"source" binding:"required"`
			Name     string      `json:"name"`
			Artist   string      `json:"artist"`
			Cover    string      `json:"cover"`
			Duration int         `json:"duration"`
			Extra    interface{} `json:"extra"`
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
}