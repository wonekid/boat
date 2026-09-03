# 多阶段构建：Node 构建前端，nginx 托管静态产物

# ---------- 构建阶段 ----------
FROM node:22-alpine AS build
WORKDIR /web
COPY package.json package-lock.json* ./
RUN npm config set registry https://registry.npmmirror.com \
    && npm install
COPY . .
RUN npm run build

# ---------- 运行时阶段 ----------
FROM nginx:1.27-alpine
COPY --from=build /web/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
