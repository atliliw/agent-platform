# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

WORKDIR /build

COPY go.mod go.sum* ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download || true

COPY pkg ./pkg
COPY proto ./proto
COPY services ./services

RUN CGO_ENABLED=0 go build -o mcp-service ./services/mcp-service/cmd

# Runtime with Chromium for browser automation
FROM alpine:latest

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# Install Chromium and dependencies
RUN apk --no-cache add \
    ca-certificates \
    chromium \
    chromium-chromedriver \
    nss \
    freetype \
    harfbuzz \
    ttf-freefont \
    font-noto-cjk

# Set Chrome path
ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROMIUM_FLAGS="--no-sandbox --disable-gpu --disable-dev-shm-usage"
ENV CHROME_HEADLESS=true

WORKDIR /app
COPY --from=builder /build/mcp-service .
COPY services/mcp-service/config.yaml .

EXPOSE 50005

CMD ["./mcp-service"]