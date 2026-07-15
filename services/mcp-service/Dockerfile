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

# Runtime - browser engine is Obscura (connected via CDP over websocket),
# so no local Chromium/Chrome is needed in this image. The local-Chromium
# fallback in pkg/browseragent only activates when OBSCURA_CDP_URL is unset.
FROM alpine:latest

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /build/mcp-service .
COPY services/mcp-service/config.yaml .

EXPOSE 50005

CMD ["./mcp-service"]