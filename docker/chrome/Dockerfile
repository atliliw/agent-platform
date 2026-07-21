# Headed Chromium + Xvfb container.
# Provides a non-headless browser over CDP for sites whose anti-bot requires a
# real headed browser (e.g. Xiaohongshu search: the page's axios interceptor
# only generates the x-s-common signature when the event loop runs normally,
# which Obscura's stealth headless engine freezes).
FROM debian:bookworm-slim

# Use a CN mirror for apt (deployment server is in CN).
RUN sed -i 's|deb.debian.org|mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list.d/debian.sources 2>/dev/null || \
    sed -i 's|deb.debian.org|mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list 2>/dev/null || true

RUN apt-get update && apt-get install -y --no-install-recommends \
    chromium \
    xvfb \
    xauth \
    dumb-init \
    nginx \
    procps \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ENV DISPLAY=:99
EXPOSE 9222

COPY nginx.conf /etc/nginx/nginx.conf
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/usr/bin/dumb-init", "--", "/entrypoint.sh"]
