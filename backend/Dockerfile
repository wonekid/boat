# 多阶段构建：编译 Go 后端，运行时使用内置 python3 的镜像
# 部分运维脚本/插件依赖 python3 执行环境，故运行时基础镜像选择 python:3.12-slim

# ---------- 构建阶段 ----------
FROM golang:1.24 AS builder
WORKDIR /build
# Go 模块代理：默认国内镜像 goproxy.cn，可由 compose build.args 覆盖
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
# 先拉取依赖，利用 Docker 层缓存
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 静态编译，便于在 slim 镜像中直接运行
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /boat ./cmd/server
# osp-agent：部署到执行机的代理程序（同时产出到 /app/dist 供控制台「接入指引」脚本下载）
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /osp-agent ./cmd/agent

# ---------- 运行时阶段 ----------
FROM python:3.12-slim
WORKDIR /app
# ca-certificates 用于 HTTPS 出访；openssh-client 便于在容器内调试远端主机
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates openssh-client \
    && rm -rf /var/lib/apt/lists/*
# 拷贝编译产物与配置
COPY --from=builder /boat /usr/local/bin/boat
COPY --from=builder /osp-agent /app/dist/osp-agent
COPY configs /app/configs
# RSA 密钥、堡垒机密钥、录像与日志在运行时由程序自动生成/写入，确保目录可写
RUN mkdir -p /app/data/records /app/logs \
    && chmod -R 777 /app/configs /app/data /app/logs
EXPOSE 8080 2222 9090
ENTRYPOINT ["boat"]
CMD ["-c", "configs/config.yaml"]
