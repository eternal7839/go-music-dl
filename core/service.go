package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	// 引入基础定义
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"

	// 引入具体的源
	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
)

// 定义搜索函数类型
type SearchFunc func(keyword string) ([]model.Song, error)

// SourceMap 管理所有支持的源
var SourceMap = map[string]SearchFunc{
	"netease":  netease.Search,
	"qq":       qq.Search,
	"kugou":    kugou.Search,
	"kuwo":     kuwo.Search,
	"migu":     migu.Search,
	"bilibili": bilibili.Search,
	"fivesing": fivesing.Search,
	"jamendo":  jamendo.Search,
	"joox":     joox.Search,
	"qianqian": qianqian.Search,
	"soda":     soda.Search,
}

// GetAllSourceNames 获取所有源的名称列表（固定顺序）
func GetAllSourceNames() []string {
	// 返回固定的源顺序，确保 Web 界面和 CLI 的一致性
	return []string{
		"netease",  // 网易云音乐
		"qq",       // QQ音乐
		"kugou",    // 酷狗音乐
		"kuwo",     // 酷我音乐
		"migu",     // 咪咕音乐
		"fivesing", // 5sing
		"jamendo",  // Jamendo
		"joox",     // JOOX
		"qianqian", // 千千音乐
		"soda",     // Soda音乐
		"bilibili", // Bilibili（放在最后，通常不推荐使用）
	}
}

// GetDefaultSourceNames 获取默认启用的源名称列表（排除 bilibili, joox, jamendo, fivesing）
func GetDefaultSourceNames() []string {
	allSources := GetAllSourceNames()
	var defaultSources []string
	excluded := map[string]bool{
		"bilibili": true,
		"joox":     true,
		"jamendo":  true,
		"fivesing": true,
	}

	for _, source := range allSources {
		if !excluded[source] {
			defaultSources = append(defaultSources, source)
		}
	}
	return defaultSources
}

// GetSourceDescription 获取音乐源的描述信息
func GetSourceDescription(source string) string {
	descriptions := map[string]string{
		"netease":  "网易云音乐 - 中国领先的在线音乐平台，以个性化推荐和社区氛围著称",
		"qq":       "QQ音乐 - 腾讯旗下音乐平台，拥有海量正版音乐资源",
		"kugou":    "酷狗音乐 - 中国知名的数字音乐交互服务提供商，以音效和K歌功能见长",
		"kuwo":     "酷我音乐 - 提供高品质音乐播放和下载服务，专注于无损音乐",
		"migu":     "咪咕音乐 - 中国移动旗下音乐平台，拥有丰富的正版音乐版权",
		"fivesing": "5sing - 中国原创音乐基地，专注于原创音乐和翻唱作品",
		"jamendo":  "Jamendo - 国际免费音乐平台，提供 Creative Commons 许可的音乐",
		"joox":     "JOOX - 腾讯在东南亚推出的音乐流媒体服务",
		"qianqian": "千千音乐 - 百度旗下音乐平台，前身为千千静听",
		"soda":     "Soda音乐 - 抖音旗下音乐平台，提供高品质音乐流媒体服务",
		"bilibili": "Bilibili - 中国知名视频弹幕网站，包含大量用户上传的音乐内容",
	}

	if desc, exists := descriptions[source]; exists {
		return desc
	}
	return "未知音乐源"
}

// SearchAndFilter 支持指定源搜索 + 并发处理
func SearchAndFilter(keyword string, selectedSources []string) ([]model.Song, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allSongs []model.Song

	// 如果未指定源，默认全选
	if len(selectedSources) == 0 {
		selectedSources = GetAllSourceNames()
	}

	// 按照固定顺序处理源，确保结果的一致性
	for _, sourceName := range selectedSources {
		searchFunc, exists := SourceMap[sourceName]
		if !exists {
			continue
		}

		wg.Add(1)
		go func(src string, sFunc SearchFunc) {
			defer wg.Done()

			// 调用具体的搜索
			songs, err := sFunc(keyword)
			if err != nil {
				fmt.Printf("搜索源 %s 失败: %v\n", src, err)
				return
			}

			// 标记来源
			for i := range songs {
				songs[i].Source = src
			}

			mu.Lock()
			allSongs = append(allSongs, songs...)
			mu.Unlock()
		}(sourceName, searchFunc)
	}

	wg.Wait()
	return allSongs, nil
}

// GetDownloadURL 根据源获取下载链接 (保持不变)
func GetDownloadURL(song *model.Song) (string, error) {
	switch song.Source {
	case "netease":
		return netease.GetDownloadURL(song)
	case "qq":
		return qq.GetDownloadURL(song)
	case "kugou":
		return kugou.GetDownloadURL(song)
	case "kuwo":
		return kuwo.GetDownloadURL(song)
	case "migu":
		return migu.GetDownloadURL(song)
	case "bilibili":
		return bilibili.GetDownloadURL(song)
	case "fivesing":
		return fivesing.GetDownloadURL(song)
	case "jamendo":
		return jamendo.GetDownloadURL(song)
	case "joox":
		return joox.GetDownloadURL(song)
	case "qianqian":
		return qianqian.GetDownloadURL(song)
	case "soda":
		return soda.GetDownloadURL(song)
	default:
		return "", fmt.Errorf("不支持的源: %s", song.Source)
	}
}

// GetLyrics 根据源获取歌词
func GetLyrics(song *model.Song) (string, error) {
	switch song.Source {
	case "netease":
		return netease.GetLyrics(song)
	case "qq":
		return qq.GetLyrics(song)
	case "kugou":
		return kugou.GetLyrics(song)
	case "kuwo":
		return kuwo.GetLyrics(song)
	case "migu":
		return migu.GetLyrics(song)
	case "bilibili":
		return bilibili.GetLyrics(song)
	case "fivesing":
		return fivesing.GetLyrics(song)
	case "jamendo":
		return jamendo.GetLyrics(song)
	case "joox":
		return joox.GetLyrics(song)
	case "qianqian":
		return qianqian.GetLyrics(song)
	case "soda":
		return soda.GetLyrics(song)
	default:
		return "", fmt.Errorf("不支持的源: %s", song.Source)
	}
}

// DownloadSong CLI使用的默认下载函数
func DownloadSong(song *model.Song) error {
	// 默认保存到 downloads 目录，不下载封面和歌词
	return DownloadSongWithOptions(song, "downloads", false, false)
}

// [修改] DownloadSongWithOptions: 统一路径管理，确保封面、歌词、音乐在同一目录
func DownloadSongWithOptions(song *model.Song, saveDir string, downloadCover bool, downloadLyrics bool) error {
	filename := song.Filename()

	// 1. 路径预处理
	if saveDir == "" {
		saveDir = "downloads"
	}
	if _, err := os.Stat(saveDir); os.IsNotExist(err) {
		os.MkdirAll(saveDir, 0755)
	}

	// 音乐文件路径
	filePath := filepath.Join(saveDir, filename)
	// 文件基础名 (不含后缀)，用于统一封面和歌词的命名
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))

	// 2. 下载音频 (区分 Soda 和 通用源)
	if song.Source == "soda" {
		// Soda 需要解密
		if err := soda.Download(song, filePath); err != nil {
			return err
		}
	} else {
		// 通用下载
		url, err := GetDownloadURL(song)
		if err != nil {
			return fmt.Errorf("获取下载链接失败: %v", err)
		}
		if url == "" {
			return fmt.Errorf("该歌曲无下载链接")
		}

		data, err := utils.Get(url)
		if err != nil {
			return fmt.Errorf("下载失败: %v", err)
		}
		
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return err
		}
	}

	// 3. 下载封面 (通用逻辑，统一路径)
	if downloadCover && song.Cover != "" {
		coverPath := filepath.Join(saveDir, baseName+".jpg")
		// 异步或忽略错误，不阻塞主流程
		_ = downloadFile(song.Cover, coverPath)
	}

	// 4. 下载歌词 (通用逻辑，统一路径)
	if downloadLyrics {
		lrc, err := GetLyrics(song)
		// 只要获取成功且内容不为空，就保存
		if err == nil && lrc != "" {
			lrcPath := filepath.Join(saveDir, baseName+".lrc")
			_ = os.WriteFile(lrcPath, []byte(lrc), 0644)
		}
	}

	return nil
}

// DownloadSongWithCover 保留是为了兼容旧代码，建议逐步废弃
func DownloadSongWithCover(song *model.Song, downloadCover bool) error {
	return DownloadSongWithOptions(song, "downloads", downloadCover, false)
}

// 辅助函数: 下载通用文件
func downloadFile(url, destPath string) error {
	data, err := utils.Get(url)
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0644)
}