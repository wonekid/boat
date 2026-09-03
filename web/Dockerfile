# 多阶段构建：Node 构建前端，nginx 托管静态产物

# ---------- 构建阶段 ----------
FROM node:22-alpine AS build
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
FROM nginx:1.27-alpine
COPY --from=build /web/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
