package web

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/guohuiyuan/music-lib/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var db *gorm.DB

// Collection 收藏夹模型 (自制歌单)
type Collection struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	Cover       string    `json:"cover"`
	CreatedAt   time.Time `json:"created_at"`
	// 配置外键以及级联删除
	SavedSongs  []SavedSong `gorm:"constraint:OnDelete:CASCADE;" json:"-"`
}

// SavedSong 收藏的歌曲模型
type SavedSong struct {
	ID           uint      `gorm:"primaryKey" json:"db_id"`
	CollectionID uint      `gorm:"uniqueIndex:idx_col_song_src" json:"collection_id"`
	SongID       string    `gorm:"uniqueIndex:idx_col_song_src;not null" json:"song_id"`
	Source       string    `gorm:"uniqueIndex:idx_col_song_src;not null" json:"source"`
	Extra        string    `json:"extra"`
	Name         string    `json:"name"`
	Artist       string    `json:"artist"`
	Cover        string    `json:"cover"`
	Duration     int       `json:"duration"`
	AddedAt      time.Time `json:"added_at"`
}

// InitDB 初始化 GORM 与 SQLite
func InitDB() {
	var err error
	// 开启外键约束，保证联级删除生效
	db, err = gorm.Open(sqlite.Open("favorites.db?_pragma=foreign_keys(1)"), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to SQLite: " + err.Error())
	}

	// 自动迁移表结构 (自动建表、增减字段、索引)
	err = db.AutoMigrate(&Collection{}, &SavedSong{})
	if err != nil {
		panic("Failed to migrate database: " + err.Error())
	}
}

// CloseDB 提供给 server.go 在退出时关闭连接池
func CloseDB() {
	if db != nil {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func RegisterCollectionRoutes(api *gin.RouterGroup) {
	// 渲染瀑布流列表
	api.GET("/my_collections", func(c *gin.Context) {
		var collections []Collection
		if err := db.Order("id DESC").Find(&collections).Error; err != nil {
			renderIndex(c, nil, nil, "我的自制歌单", nil, "获取收藏夹失败", "playlist", "", "", "", true)
			return
		}

		var playlists []model.Playlist
		for _, col := range collections {
			var count int64
			db.Model(&SavedSong{}).Where("collection_id = ?", col.ID).Count(&count)

			cvr := col.Cover
			if cvr == "" {
				cvr = fmt.Sprintf("https://picsum.photos/seed/col_%d/400/400", col.ID)
			}

			playlists = append(playlists, model.Playlist{
				ID:          fmt.Sprint(col.ID),
				Name:        col.Name,
				Description: col.Description,
				Cover:       cvr,
				Creator:     "我自己",
				TrackCount:  int(count),
				Source:      "local",
			})
		}
		renderIndex(c, nil, playlists, "我的自制歌单", nil, "", "playlist", "", "", "", true)
	})

	// 渲染具体的收藏夹（把它变成歌单输出）
	api.GET("/collection", func(c *gin.Context) {
		id := c.Query("id")
		if id == "" {
			renderIndex(c, nil, nil, "", nil, "缺少参数", "song", "", "", "", false)
			return
		}

		var collection Collection
		if err := db.First(&collection, id).Error; err != nil {
			renderIndex(c, nil, nil, "", nil, "自制歌单不存在", "song", "", "", "", false)
			return
		}

		var savedSongs []SavedSong
		db.Where("collection_id = ?", id).Order("id DESC").Find(&savedSongs)

		var songs []model.Song
		for _, ss := range savedSongs {
			songs = append(songs, model.Song{
				ID:       ss.SongID,
				Source:   ss.Source,
				Name:     ss.Name,
				Artist:   ss.Artist,
				Cover:    ss.Cover,
				Duration: ss.Duration,
			})
		}
		renderIndex(c, songs, nil, "", nil, "", "song", "", id, collection.Name, false)
	})

	colApi := api.Group("/collections")

	// 获取所有收藏夹数据 (用于弹窗)
	colApi.GET("", func(c *gin.Context) {
		var collections []Collection
		db.Order("id DESC").Find(&collections)
		c.JSON(200, collections)
	})

	// 创建新收藏夹
	colApi.POST("", func(c *gin.Context) {
		var req Collection
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
			c.JSON(400, gin.H{"error": "参数错误，必须提供歌单名"})
			return
		}

		if err := db.Create(&req).Error; err != nil {
			c.JSON(500, gin.H{"error": "创建失败: " + err.Error()})
			return
		}
		c.JSON(200, gin.H{"id": req.ID, "name": req.Name})
	})

	// 更新收藏夹
	colApi.PUT("/:id", func(c *gin.Context) {
		id := c.Param("id")
		var req Collection
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
			c.JSON(400, gin.H{"error": "参数错误"})
			return
		}

		if err := db.Model(&Collection{}).Where("id = ?", id).Updates(Collection{
			Name:        req.Name,
			Description: req.Description,
			Cover:       req.Cover,
		}).Error; err != nil {
			c.JSON(500, gin.H{"error": "更新失败"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 删除收藏夹 (利用 GORM 级联，自动清空其内部的所有歌曲)
	colApi.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := db.Delete(&Collection{}, id).Error; err != nil {
			c.JSON(500, gin.H{"error": "删除失败"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 获取某收藏夹下的歌曲 (纯 API)
	colApi.GET("/:id/songs", func(c *gin.Context) {
		colID := c.Param("id")
		var savedSongs []SavedSong
		db.Where("collection_id = ?", colID).Order("id DESC").Find(&savedSongs)

		var resp []gin.H
		for _, s := range savedSongs {
			var extraObj interface{}
			if err := json.Unmarshal([]byte(s.Extra), &extraObj); err != nil {
				extraObj = s.Extra
			}
			resp = append(resp, gin.H{
				"db_id":         s.ID,
				"collection_id": s.CollectionID,
				"id":            s.SongID,
				"source":        s.Source,
				"extra":         extraObj,
				"name":          s.Name,
				"artist":        s.Artist,
				"cover":         s.Cover,
				"duration":      s.Duration,
				"added_at":      s.AddedAt,
			})
		}
		c.JSON(200, resp)
	})

	// 添加一首歌到指定收藏夹 (利用 GORM 的 OnConflict 防止重复插入)
	colApi.POST("/:id/songs", func(c *gin.Context) {
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

		colID := c.Param("id")
		var cid uint
		fmt.Sscanf(colID, "%d", &cid)

		extraStr := ""
		if req.Extra != nil {
			b, _ := json.Marshal(req.Extra)
			extraStr = string(b)
		}

		song := SavedSong{
			CollectionID: cid,
			SongID:       req.SongID,
			Source:       req.Source,
			Name:         req.Name,
			Artist:       req.Artist,
			Cover:        req.Cover,
			Duration:     req.Duration,
			Extra:        extraStr,
		}

		// 如果同一歌单下已有同 Source 同 ID 的歌，则静默忽略 (DoNothing)
		err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&song).Error
		if err != nil {
			c.JSON(500, gin.H{"error": "添加失败: " + err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 将歌曲移出收藏夹
	colApi.DELETE("/:id/songs", func(c *gin.Context) {
		colID := c.Param("id")
		songID := c.Query("id")
		source := c.Query("source")

		if songID == "" || source == "" {
			c.JSON(400, gin.H{"error": "需要通过 query 传递 id 和 source"})
			return
		}

		err := db.Where("collection_id = ? AND song_id = ? AND source = ?", colID, songID, source).Delete(&SavedSong{}).Error
		if err != nil {
			c.JSON(500, gin.H{"error": "删除失败"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})
}