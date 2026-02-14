#!/bin/bash
# ---------------------------------------------------------
# 修复 Windows Git Bash 下路径自动转换导致的问题
export MSYS_NO_PATHCONV=1
# ---------------------------------------------------------

# Go-Music-DL 远程镜像部署脚本 (适配版)

set -e

# ================= 配置项 =================
# 镜像名称
IMAGE_NAME="guohuiyuan/go-music-dl:latest"
# 部署目录
WORK_DIR="music-dl"
# =========================================

echo "🎵 开始部署 Go-Music-DL (适配 Docker Compose 版)..."

# 1. 检查 Docker 环境
if ! command -v docker &> /dev/null; then
    echo "❌ Docker 未安装"
    exit 1
fi

if docker compose version &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker compose"
elif command -v docker-compose &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker-compose"
else
    echo "❌ 未找到 Docker Compose"
    exit 1
fi

# 2. 准备工作目录
if [ ! -d "$WORK_DIR" ]; then
    echo "📂 创建部署目录: $WORK_DIR"
    mkdir -p "$WORK_DIR"
fi

# !!! 进入目录 !!!
cd "$WORK_DIR"
echo "📂 已进入目录: $(pwd)"

# 3. 清理旧进程
echo "🧹 清理旧服务..."
$DOCKER_COMPOSE_CMD down 2>/dev/null || true

# 强力清理可能存在的同名容器
if docker ps -a --format '{{.Names}}' | grep -q "^music-dl$"; then
    echo "   ⚠️ 发现旧容器实例，正在强制删除..."
    docker rm -f music-dl
fi

# 4. 创建挂载目录/文件与权限 (关键适配点)
# -------------------------------------------------
# 适配点 A: 下载目录
if [ ! -d "downloads" ]; then
    echo "📁 创建下载目录 downloads/ ..."
    mkdir -p downloads
fi

# 适配点 B: Cookies 文件 (同步 docker-compose.yml 配置)
if [ ! -f "cookies.json" ]; then
    echo "🍪 创建空的 cookies.json ..."
    touch cookies.json
    echo "{}" > cookies.json
fi

echo "🔐 修正权限 (chmod 777 downloads & 666 cookies) ..."
# 目录给 777 以便容器内创建文件
chmod -R 777 downloads
# cookies 文件给 666 以便容器内读取/写入
chmod 666 cookies.json
# -------------------------------------------------

# 5. 生成 docker-compose.yml
# 适配点 C: 将 cookies.json 挂载写入配置
echo "📝 生成 docker-compose.yml..."
cat > docker-compose.yml <<EOF
services:
  music-dl:
    image: ${IMAGE_NAME}
    container_name: music-dl
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./downloads:/home/appuser/downloads
      - ./cookies.json:/home/appuser/cookies.json
    environment:
      - TZ=Asia/Shanghai
    user: "1000:1000"
EOF

# 6. 拉取并启动
echo "☁️  正在拉取最新镜像: $IMAGE_NAME ..."
$DOCKER_COMPOSE_CMD pull

echo "🚀 启动服务..."
$DOCKER_COMPOSE_CMD up -d

# 7. 检查状态
echo "⏳ 等待初始化 (3秒)..."
sleep 3

if docker ps | grep -q "music-dl"; then
    echo ""
    echo "✅ 部署成功！"
    echo "------------------------------------------------"
    echo "🎵 Web 访问: http://localhost:8080"
    echo "📂 本地目录: $(pwd)/downloads"
    echo "🍪 Cookies : $(pwd)/cookies.json"
    echo ""
    echo "👇 常用命令 (请先 cd $WORK_DIR):"
    echo "   查看日志: $DOCKER_COMPOSE_CMD logs -f"
    echo "   重启服务: $DOCKER_COMPOSE_CMD restart"
    echo "------------------------------------------------"
else
    echo ""
    echo "❌ 启动失败！"
    echo "请检查日志: cd $WORK_DIR && $DOCKER_COMPOSE_CMD logs"
    exit 1
fi