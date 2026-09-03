# 多阶段构建：Node 构建前端，nginx 托管静态产物

# ---------- 构建阶段 ----------
# 注意：Node 22/24 的 Alpine(musl) 镜像存在 npm 已知缺陷，npm ci/install 会稳定触发
# "Exit handler never called!" 崩溃（2026 仍 open）。改用 glibc 的 Debian 镜像可根治。
FROM node:26-slim AS build
WORKDIR /web
# 通过环境变量注入 npm 配置：国内镜像 + 重试/超时
# 避免构建时偶发网络抖动触发 npm 内部 "Exit handler never called" 崩溃
ENV npm_config_registry=https://registry.npmmirror.com
ENV npm_config_fetch_retries=5
ENV npm_config_fetch_retry_mintimeout=20000
ENV npm_config_fetch_retry_maxtimeout=120000
ENV npm_config_fetch_timeout=600000
COPY package.json package-lock.json ./
# 使用 npm ci（基于 package-lock.json 确定性安装，元数据请求更少、更稳）
RUN npm ci --no-audit --no-fund
COPY . .
RUN npm run build

# ---------- 运行时阶段 ----------
FROM nginx:1.30.4-alpine
COPY --from=build /web/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
