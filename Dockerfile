# 使用官方 Go 镜像作为构建环境
FROM docker.m.daocloud.io/library/golang:1.25.6 AS builder

# 设置环境变量
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# 设置工作目录
WORKDIR /app

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./

# 复制 pkg 目录 (因为有 replace 指向 ./pkg)
COPY pkg ./pkg
COPY coreClass ./coreClass
# COPY Data ./Data

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用程序
RUN go build -o server main.go

# 使用轻量级镜像
FROM docker.m.daocloud.io/library/alpine:latest

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/server .
# 复制配置文件和静态资源
# COPY --from=builder /app/Data ./Data
# 如果有其他需要的目录，也需要复制
# COPY --from=builder /app/coreClass ./coreClass

# 暴露端口
EXPOSE 1001
EXPOSE 1002
EXPOSE 9009
EXPOSE 5555

# 运行应用程序
CMD ["./server"]
