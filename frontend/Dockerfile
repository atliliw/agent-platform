# 构建阶段
FROM node:20-alpine AS builder
WORKDIR /app
# 使用淘宝镜像
RUN npm config set registry https://registry.npmmirror.com
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# 生产阶段
FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]