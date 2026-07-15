FROM golang:1.24-alpine AS builder

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories || true

WORKDIR /build

ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum* ./
RUN go mod download || true

COPY pkg ./pkg
COPY proto ./proto
COPY services ./services

RUN CGO_ENABLED=0 GOOS=linux go build -o agent-service ./services/agent-service/cmd

# Runtime stage - reuse golang alpine to avoid TLS issues with separate alpine pull
FROM golang:1.24-alpine

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories || true

WORKDIR /app

COPY --from=builder /build/agent-service .
COPY services/agent-service/config.yaml .

RUN mkdir -p /app/data /app/configs/agents

ENV PORT=50006
ENV STORE_PATH=/app/data/agent_contexts.db

EXPOSE 50006

CMD ["./agent-service", "-config", "config.yaml"]
