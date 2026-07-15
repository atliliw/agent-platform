# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

WORKDIR /build

COPY go.mod go.sum* ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download || true

COPY services/mcp-demo-server ./services/mcp-demo-server

RUN CGO_ENABLED=0 go build -o mcp-demo-server ./services/mcp-demo-server/cmd

# Runtime
FROM alpine:latest

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /build/mcp-demo-server .

ENV PORT=50009
EXPOSE 50009

CMD ["./mcp-demo-server"]
