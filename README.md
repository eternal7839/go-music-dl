# 🎵 Go Music DL: 你的全能音乐下载神器 🎵

还在为找歌、下歌烦恼吗？`Go Music DL` 是一款为你量身打造的音乐下载工具，它将强大的命令行功能与简洁的网页界面合二为一，让你轻松搜索、试听、下载来自多个主流音乐平台的歌曲。

无论你是喜欢在终端里敲命令的极客，还是偏爱在浏览器里点点鼠标的普通用户，`Go Music DL` 都能给你带来丝滑流畅的音乐体验。

## ✨ 亮点功能

- 💻 **双模操作，任你选择**: 提供酷炫的 **命令行终端 (CLI)** 和简洁的 **网页 (Web)** 两种模式，满足不同使用习惯。
- 🌐 **聚合搜索，一网打尽**: 支持 **网易云、QQ音乐、酷狗** 等十多个主流音乐平台，想听的歌基本都能找到。
- 🎨 **精美界面，赏心悦目**:
  - **CLI**: 交互式表格，键盘操作，极客范十足。
  - **Web**: 现代化设计，还有可爱的 **Live2D 看板娘** 陪伴你。
- 📥 **智能下载，井井有条**:
  - 自动将下载的音乐命名为 `歌手 - 歌名.mp3`。
  - 歌曲封面、歌词也能一并下载，信息超完整。
- 🎧 **在线试听，同步歌词**: 内置网页播放器，下载前可以先听为快，还支持滚动歌词。
- 🔓 **特殊解锁**: 支持解密**汽水音乐**的加密音频。
- 💰 **免费优先，智能过滤**: 自动跳过 VIP 和付费歌曲，为你筛选出可免费下载的音源。
- 🔢 **花式选择，批量下载**: 支持 `1-3`、`1 3 5` 等多种方式，一次性下载多首歌曲。

## 🚀 快速上手

只需几步，即可开启你的音乐之旅！

### 1. 安装

首先，请确保你的电脑上安装了 **Go (1.20 或更高版本)** 和 **Git**。

```bash
# 1. 克隆项目到你的电脑
git clone https://github.com/guohuiyuan/go-music-dl.git

# 2. 进入项目目录
cd go-music-dl

# 3. 编译项目
go build -o music-dl ./cmd/music-dl
```

看到目录下多出了一个 `music-dl` (或 `music-dl.exe`) 文件，就说明成功啦！

### 2. 使用

#### 方式一：在网页中点点点

```bash
# 启动 Web 服务
./music-dl web
```

然后打开浏览器，访问 `http://localhost:8080` 就可以看到搜索界面了。

![Web UI](https://raw.githubusercontent.com/guohuiyuan/go-music-dl/main/screenshots/web.png)

#### 方式二：在命令行里敲敲敲

```bash
# 简单搜索
./music-dl -k "周杰伦"
```

程序会进入一个交互界面，让你挑选要下载的歌曲。

![TUI](https://raw.githubusercontent.com/guohuiyuan/go-music-dl/main/screenshots/tui.png)

是不是很简单？更详细的用法请继续往下看。

## 🎶 支持的音乐平台

本项目支持以下音乐源，每个源都有其特色和适用场景：

- **网易云音乐 (netease)**: 国内主流音乐平台，曲库丰富，包含大量原创和独立音乐人作品
- **QQ音乐 (qq)**: 腾讯旗下音乐平台，拥有大量正版音乐版权，特别是华语流行音乐
- **酷狗音乐 (kugou)**: 老牌音乐平台，以海量曲库和K歌功能著称
- **酷我音乐 (kuwo)**: 提供高品质音乐，支持多种音质选择，包括无损格式
- **咪咕音乐 (migu)**: 中国移动旗下音乐平台，拥有大量正版音乐资源
- **5sing原创音乐 (fivesing)**: 专注于原创音乐和翻唱作品的平台，适合寻找独立音乐人作品
- **Jamendo (jamendo)**: 国际免费音乐平台，所有音乐均可免费下载和使用
- **JOOX音乐 (joox)**: 腾讯国际版音乐平台，主要面向东南亚市场
- **千千音乐 (qianqian)**: 百度旗下音乐平台，整合了百度音乐资源
- **汽水音乐 (soda)**: 字节跳动旗下音乐平台，主打个性化推荐
- **Bilibili音频 (bilibili)**: 从B站视频中提取音频内容，包含大量二次创作和同人音乐

**注意**: 默认情况下排除 Bilibili 源，因为其内容多为视频音频，可能包含非音乐内容。如需使用可通过 `-s bilibili` 参数显式指定。

## 📚 使用指南

### Web 模式

启动服务后，在浏览器中打开 `http://localhost:8080` 即可使用。
- 在搜索框输入歌曲或歌手名。
- 在下方的复选框中选择你想要搜索的音乐平台。
- 点击“搜索”，结果会以表格形式呈现。
- 你可以点击“试听”播放在线音乐和歌词。
- 点击“下载”即可保存音乐文件。

### CLI 模式

#### 完整参数
```bash
# 查看所有可用参数
./music-dl --help
```

输出：
```
Search and download music from netease, qq, kugou, baidu and xiami.

Usage:
  music-dl [OPTIONS] [flags]
  music-dl [command]

Examples:
  music-dl -k "周杰伦"
  music-dl web

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  web         启动 Web 服务模式

Flags:
      --cover             同时下载歌词
      --filter string     按文件大小和歌曲时长过滤搜索结果
  -h, --help              help for music-dl
  -k, --keyword string    搜索关键字，歌名和歌手同时输入可以提高匹配
      --lyrics            同时下载歌词
      --nomerge           不对搜索结果列表排序和去重
  -n, --number int        Number of search results (default 10)
  -o, --outdir string     Output directory (default ".")
      --play              开启下载后自动播放功能
  -p, --playlist string   通过指定的歌单URL下载音乐
  -x, --proxy string      Proxy (e.g. http://127.0.0.1:1087)
  -s, --source strings    Supported music source (default [netease,qq,kugou,kuwo,migu])
  -u, --url string        通过指定的歌曲URL下载音乐
  -v, --verbose           Verbose mode
      --version           Show the version and exit.
```

#### 灵活的选择方式
在交互式下载中，你可以用多种方式指定下载序号：
- **单个选择**: `1`
- **多个选择**: `1 3 5` (用空格) 或 `1,3,5` (用逗号)
- **范围选择**: `1-3` (选择第1到第3首)
- **混合选择**: `1-3,5,7-9` (选择第1-3首、第5首、第7-9首)

## ❓ 常见问题 (FAQ)

### Q: 为什么有些歌搜不到或下载失败？
A: 可能是因为：
1.  歌曲需要 VIP 才能听。
2.  该音乐平台的接口最近有变动。
3.  你的网络不太稳定。
本工具会自动过滤掉大部分 VIP 歌曲，但无法保证 100% 成功。

### Q: 为什么默认不搜索 Bilibili？
A: B 站主要是视频，音频内容比较杂，为了保证搜索结果的纯粹性，默认排除了该源。如果你确实需要，可以在搜索时手动加上：`-s bilibili`。

### Q: Web 模式启动失败怎么办？
A: 请检查：
1.  启动时指定的端口（默认为 8080）是否被其他程序占用了。
2.  项目依赖是否已完整安装。

### Q: 如何下载歌曲的封面和歌词？
A: 本工具会自动尝试下载封面和歌词。下载成功后，封面会保存为 `歌手 - 歌名.jpg`，歌词会内嵌到音乐文件中或保存为 `.lrc` 文件。

### Q: 汽水音乐（Soda）的音频为什么要解密？
A: 因为汽水音乐对音频文件做了加密处理。本工具内置了解密程序，在下载后会自动完成解密，整个过程你无需关心，下载完成即可正常播放。

## 👨‍💻 给开发者

### 项目结构
```
go-music-dl/
├── cmd/
│   └── music-dl/
│       ├── main.go           # 程序入口
│       ├── root.go           # CLI 主命令逻辑
│       └── web.go            # Web 子命令逻辑
├── core/                     # 核心逻辑层
│   └── service.go           # 源映射管理和并发搜索
├── internal/
│   ├── cli/                  # CLI 交互逻辑 (Bubble Tea)
│   └── web/                  # Web 服务逻辑 (Gin)
├── pkg/
│   └── models/               # 扩展数据模型
├── go.mod
├── go.sum
└── README.md
```

### 贡献指南

欢迎为本项目贡献代码！
1.  Fork 本项目
2.  创建你的功能分支 (`git checkout -b feature/AmazingFeature`)
3.  提交你的更改 (`git commit -m 'Add some AmazingFeature'`)
4.  将分支推送到你的 Fork 仓库 (`git push origin feature/AmazingFeature`)
5.  提交一个 Pull Request

### 致谢

本项目离不开以下优秀开源项目的支持：
- [music-lib](https://github.com/guohuiyuan/music-lib): 核心音乐搜索库
- [Cobra](https://github.com/spf13/cobra): CLI 框架
- [Gin](https://github.com/gin-gonic/gin): Web 框架
- [Bubble Tea](https://github.com/charmbracelet/bubbletea): TUI 框架
- [Tailwind CSS](https://tailwindcss.com/): CSS 框架
- 以及 [musicdl](https://github.com/CharlesPikachu/musicdl) 和 [music-dl](https://github.com/0xHJK/music-dl) 带来的灵感。

## 📜 许可证

本项目基于 [GNU Affero General Public License v3.0](https://github.com/guohuiyuan/go-music-dl/blob/main/LICENSE) 许可。

## ⚠️ 免责声明

本项目仅供个人学习和技术研究使用，请在遵守相关法律法规和音乐平台用户协议的前提下使用。通过本工具下载的任何资源，请在 24 小时内删除。请支持正版音乐！