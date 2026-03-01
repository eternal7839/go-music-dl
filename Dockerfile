# Build stage
FROM golang:1.25 AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o music-dl ./cmd/music-dl

# Runtime stage
FROM alpine:latest

# 替换为阿里云镜像源，解决 TLS 连接错误和速度慢的问题
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# Install ca-certificates for HTTPS requests and tzdata for timezone support
RUN apk --no-cache add ca-certificates tzdata

# Set timezone to Asia/Shanghai
ENV TZ=Asia/Shanghai

# Create a non-root user
RUN adduser -D -s /bin/sh appuser

# Set working directory
WORKDIR /home/appuser/

# Copy the binary from builder stage
COPY --from=builder /app/music-dl .

# Change ownership to non-root user (包含刚刚创建的 data 目录)
RUN chown -R appuser:appuser /home/appuser/

# Switch to non-root user
USER appuser

# Expose port 8080
EXPOSE 8080

# Run the web server by default (videogen buttons disabled, add flags to enable)
CMD ["./music-dl", "web", "--port", "8080", "--no-browser"]