# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk add --no-cache git

ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download

COPY pkg ./pkg
COPY proto ./proto
COPY services ./services

RUN CGO_ENABLED=0 GOOS=linux go build -o agent-service ./services/agent-service/cmd

# Runtime stage
FROM alpine:latest

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /build/agent-service .
COPY services/agent-service/config.yaml .

RUN mkdir -p /app/data /app/configs/agents

ENV PORT=50006
ENV STORE_PATH=/app/data/agent_contexts.db

EXPOSE 50006

CMD ["./agent-service", "-config", "config.yaml"]
