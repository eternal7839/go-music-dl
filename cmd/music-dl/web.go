package main

import (
	"github.com/guohuiyuan/go-music-dl/internal/web"
	"github.com/spf13/cobra"
)

var port string
var noBrowser bool
var vgChangeCover bool
var vgChangeAudio bool
var vgChangeLyric bool
var vgExportVideo bool

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "启动 Web 服务模式",
	Run: func(cmd *cobra.Command, args []string) {
		web.Start(port, !noBrowser, web.FeatureFlags{
			VgChangeCover: vgChangeCover,
			VgChangeAudio: vgChangeAudio,
			VgChangeLyric: vgChangeLyric,
			VgExportVideo: vgExportVideo,
		})
	},
}

func init() {
	webCmd.Flags().StringVarP(&port, "port", "p", "8080", "服务端口")
	webCmd.Flags().BoolVar(&noBrowser, "no-browser", false, "不自动打开浏览器")
	webCmd.Flags().BoolVar(&vgChangeCover, "vg-cover", false, "启用视频生成-更换封面按钮")
	webCmd.Flags().BoolVar(&vgChangeAudio, "vg-audio", false, "启用视频生成-更换音频按钮")
	webCmd.Flags().BoolVar(&vgChangeLyric, "vg-lyric", false, "启用视频生成-更换歌词按钮")
	webCmd.Flags().BoolVar(&vgExportVideo, "vg-export", false, "启用视频生成-导出视频按钮")
	rootCmd.AddCommand(webCmd)
}
