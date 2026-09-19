# The official spinloop agent image: opencode, Pi and gh, nothing of any
# operator's own harness configuration baked in — the docker dispatch
# backend mounts a scoped, per-launch config in at runtime (see
# internal/orchestrator/dispatch_docker.go and images/agent/README.md).
#
# Build args pin the harness versions; the CI workflow that publishes this
# image passes the versions current at release time.
FROM node:22-bookworm-slim

ARG OPENCODE_VERSION=1.18.31
ARG PI_VERSION=0.85.1

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl gnupg ripgrep \
    && mkdir -p -m 755 /etc/apt/keyrings \
    && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
        -o /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
        > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update && apt-get install -y --no-install-recommends gh \
    && rm -rf /var/lib/apt/lists/*

# opencode: a single binary off its GitHub releases — no npm package.
RUN arch="$(dpkg --print-architecture)" \
    && case "$arch" in \
         amd64) oc_arch=x64 ;; \
         arm64) oc_arch=arm64 ;; \
         *) echo "opencode has no release for architecture $arch" >&2; exit 1 ;; \
       esac \
    && curl -fsSL -o /tmp/opencode.tar.gz \
        "https://github.com/anomalyco/opencode/releases/download/v${OPENCODE_VERSION}/opencode-linux-${oc_arch}.tar.gz" \
    && tar -xzf /tmp/opencode.tar.gz -C /usr/local/bin opencode \
    && chmod +x /usr/local/bin/opencode \
    && rm /tmp/opencode.tar.gz

# Pi: npm's global bin directory is already on the node image's PATH.
RUN npm install -g "@earendil-works/pi-coding-agent@${PI_VERSION}" \
    && npm cache clean --force

# A fixed, unprivileged user and home: the docker dispatch backend mounts a
# launch's scoped config directly at a path under it (opencode:
# $HOME/.config/opencode/opencode.json, Pi: $HOME/.pi/agent/models.json).
RUN useradd --create-home --home-dir /home/agent --shell /bin/sh agent
USER agent
ENV HOME=/home/agent
WORKDIR /workspace
