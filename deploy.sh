#!/bin/bash
# ---------------------------------------------------------
# 修复 Windows Git Bash 下路径自动转换导致的问题
export MSYS_NO_PATHCONV=1
# ---------------------------------------------------------

# Go-Music-DL Docker Compose 部署脚本

set -e

echo "🎵 开始部署 Go-Music-DL..."

# 1. 检查环境
if ! command -v docker &> /dev/null; then
    echo "❌ Docker 未安装"
    exit 1
fi

# 检查 Docker Compose 命令版本
if docker compose version &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker compose"
elif command -v docker-compose &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker-compose"
else
    echo "❌ 未找到 Docker Compose"
    exit 1
fi

# 2. 确认当前目录（必须在项目根目录）
if [ ! -f "Dockerfile" ]; then
    echo "❌ 错误：当前目录下未找到 Dockerfile。"
    echo "   请确保脚本在 go-music-dl 的源码根目录下运行。"
    exit 1
fi
echo "📂 当前工作目录: $(pwd)"

# 3. 清理旧进程
echo "🧹 正在检查并清理旧服务..."
$DOCKER_COMPOSE_CMD down 2>/dev/null || true

# 强力清理同名容器
if docker ps -a --format '{{.Names}}' | grep -q "^music-dl$"; then
    echo "   ⚠️ 发现旧的 music-dl 容器实例，正在强制删除..."
    docker rm -f music-dl
else
    echo "   ✅ 无残留旧容器"
fi

# 4. 创建目录与权限控制 (关键步骤)
# 容器内用户是 appuser (uid 1000)，必须确保宿主机目录可写
if [ ! -d "downloads" ]; then
    echo "📁 创建下载目录 downloads/ ..."
    mkdir -p downloads
fi

echo "🔐 修正目录权限 (chmod 777 downloads) ..."
# 简单粗暴但有效，防止 Permission denied
chmod -R 777 downloads

# 5. 生成 docker-compose.yml
echo "📝 生成 docker-compose.yml..."
cat > docker-compose.yml <<EOF
services:
  music-dl:
    build: 
      context: .
      dockerfile: Dockerfile
    image: go-music-dl:latest
    container_name: music-dl
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./downloads:/home/appuser/downloads
    environment:
      - TZ=Asia/Shanghai
    user: "1000:1000"
EOF

# 6. 启动服务
echo "🏗️ 开始构建并启动容器..."
# 添加 --build 参数确保每次代码变动后都会重新构建镜像
$DOCKER_COMPOSE_CMD up -d --build

# 7. 检查状态
echo "⏳ 等待初始化 (3秒)..."
sleep 3

if docker ps | grep -q "music-dl"; then
    echo ""
    echo "✅ 部署成功！"
    echo "------------------------------------------------"
    echo "🎵 Web 访问: http://localhost:8080"
    echo "📂 下载目录: $(pwd)/downloads"
    echo ""
    echo "👇 常用维护命令:"
    echo "   查看日志: $DOCKER_COMPOSE_CMD logs -f"
    echo "   停止服务: $DOCKER_COMPOSE_CMD down"
    echo "   重新构建: $DOCKER_COMPOSE_CMD up -d --build"
    echo "------------------------------------------------"
else
    echo ""
    echo "❌ 容器启动失败！"
    echo "请运行以下命令查看错误日志："
    echo "$DOCKER_COMPOSE_CMD logs"
    exit 1
fi