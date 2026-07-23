# Desktop container for Computer Use: Xvfb + XFCE + xdotool + scrot, with a Go
# sidecar that exposes the desktop primitives over HTTP for mcp-service.
#
# NOTE (per plan): Xvfb permissions and screenshot stability in a headless
# container must be verified at deploy time. The package list is the standard
# debian bookworm set; confirm the build succeeds on the target host.
FROM golang:1.22-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /desktop-sidecar ./services/desktop/cmd

FROM debian:bookworm-slim

# Use a CN mirror for apt (deployment server is in CN).
RUN sed -i 's|deb.debian.org|mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list.d/debian.sources 2>/dev/null || \
    sed -i 's|deb.debian.org|mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list 2>/dev/null || true

RUN apt-get update && apt-get install -y --no-install-recommends \
    xvfb \
    xauth \
    xdotool \
    scrot \
    xfce4 \
    xfce4-terminal \
    xfce4-calculator \
    fonts-noto-cjk \
    dumb-init \
    procps \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ENV DISPLAY=:99
EXPOSE 9100

COPY --from=builder /desktop-sidecar /usr/local/bin/desktop-sidecar
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/usr/bin/dumb-init", "--", "/entrypoint.sh"]
