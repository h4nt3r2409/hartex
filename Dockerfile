# syntax=docker/dockerfile:1
#
# Runtime-only image (nothing is compiled inside the image): install only the runtime tools and put in a **prebuilt Linux single binary**.
# The binary is cross-compiled by the CI binaries job (pure Go, no QEMU) and placed per target architecture at
# dist/<TARGETARCH>/artex in the build context. This way, in a multi-arch build only the apt layer for arm64 needs
# emulation, and Next/Go compilation is no longer emulated, which is much faster.
#
# To build the image manually on a local machine, prepare the binary yourself first:
#   cd web && npm run build:static && cd ..
#   mkdir -p server/webui && cp -r web/out server/webui/dist
#   CGO_ENABLED=0 GOARCH=amd64 go build -tags embedui -o dist/amd64/artex ./cmd/artex
#   docker build -t artex:local .
FROM python:3.12-slim-bookworm
ARG TARGETARCH
# Runtime tools: ripgrep / curl / vim plus a bundle of tools commonly used for recon (add or remove as needed).
# Node is installed from NodeSource as 20.x: bookworm's default apt nodejs is 18, and Playwright requires >=20.
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates ripgrep curl wget vim git jq unzip \
      dnsutils iputils-ping netcat-openbsd inetutils-telnet whois nmap \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*
# Pre-install Playwright MCP and CLI globally (so nothing is downloaded over the network via npx at runtime).
# @playwright/mcp: run the browser MCP directly with `npx @playwright/mcp` (already installed globally; -y/@latest not needed).
# @playwright/cli: provides playwright-cli; after install, verify it runs with --help.
# Next install playwright (which provides browser management) and pre-install chromium plus its system dependencies with --with-deps.
# This way the MCP/CLI inside the container work immediately on first start, and no browser is downloaded over the network.
RUN npm install -g @playwright/mcp@latest @playwright/cli@latest playwright@latest \
    && playwright-cli --help \
    && playwright install --with-deps chromium \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
# Prebuilt binary for the corresponding architecture (dist/amd64/artex or dist/arm64/artex)
COPY dist/${TARGETARCH}/artex /app/artex
# Supervising start script: decides whether to restart after the process exits based on the exit code, and the one-click update in the UI completes the replacement through this script.
# This script also forwards SIGTERM to artex: docker stop only sends the signal to PID 1, so
# without forwarding, artex never receives the signal, cannot shut down gracefully, and is force-killed after 10 seconds with SIGKILL.
COPY start.sh /app/start.sh
RUN chmod +x /app/artex /app/start.sh
COPY skills/ /app/skills/
# Persistence mount for data/ (SQLite + jwt.key)
VOLUME ["/app/data"]
EXPOSE 8787 8788
ENTRYPOINT ["/app/start.sh"]
CMD ["-addr", ":8787", "-proxy", ":8788"]
