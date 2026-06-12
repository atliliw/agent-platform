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

RUN CGO_ENABLED=0 go build -o chat-service ./services/chat-service/cmd

FROM alpine:latest
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/chat-service .
COPY services/chat-service/config.yaml .
EXPOSE 50001
CMD ["./chat-service"]