# syntax=docker/dockerfile:1

# Vineyard requires Git >= 2.48 for `git worktree add/repair --relative-paths`.
# Build Git from upstream source so the requirement holds regardless of Debian's apt version.
FROM mcr.microsoft.com/devcontainers/go:1.25-trixie AS git-builder
ARG GIT_VERSION=2.49.0
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get install -y --no-install-recommends \
        autoconf \
        build-essential \
        gettext \
        libcurl4-gnutls-dev \
        libexpat1-dev \
        libssl-dev \
        xz-utils \
        zlib1g-dev \
    && curl -fsSL "https://mirrors.edge.kernel.org/pub/software/scm/git/git-${GIT_VERSION}.tar.xz" -o /tmp/git.tar.xz \
    && mkdir -p /tmp/git-src \
    && tar -xf /tmp/git.tar.xz -C /tmp/git-src --strip-components=1 \
    && make -C /tmp/git-src -j"$(nproc)" prefix=/usr/local NO_TCLTK=YesPlease all \
    && make -C /tmp/git-src DESTDIR=/opt/git prefix=/usr/local NO_TCLTK=YesPlease install

FROM mcr.microsoft.com/devcontainers/go:1.25-trixie

COPY --from=git-builder /opt/git/usr/local/ /usr/local/

# System tools needed for Vineyard development, testing, TUI driving, and PRs.
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && mkdir -p -m 755 /etc/apt/keyrings \
    && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
        -o /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
        > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        fd-find \
        fonts-dejavu-core \
        gh \
        jq \
        libcurl3t64-gnutls \
        libexpat1 \
        procps \
        ripgrep \
        tmux \
        zlib1g \
    && ln -sf /usr/bin/fdfind /usr/local/bin/fd \
    && rm -rf /var/lib/apt/lists/*

# Project Go CLIs: grapes (issue tracker), vineyard (orchestrator), freeze (PR TUI screenshots).
# Use an isolated GOPATH/GOCACHE so /go stays cleanly writable by the non-root vscode user.
RUN GOPATH=/tmp/go-install GOCACHE=/tmp/go-cache GOBIN=/usr/local/bin \
        go install github.com/Mibokess/grapes@latest \
    && GOPATH=/tmp/go-install GOCACHE=/tmp/go-cache GOBIN=/usr/local/bin \
        go install github.com/mikeryanboss/vineyard@latest \
    && GOPATH=/tmp/go-install GOCACHE=/tmp/go-cache GOBIN=/usr/local/bin \
        go install github.com/charmbracelet/freeze@latest \
    && rm -rf /tmp/go-install /tmp/go-cache \
    && mkdir -p /go/pkg/mod \
    && chown -R vscode:vscode /go

USER vscode
ENV HOME=/home/vscode
ENV PATH="/home/vscode/.local/bin:/home/vscode/.gemini/antigravity-cli/bin:/home/vscode/.hunk/bin:/usr/local/go/bin:/go/bin:/usr/local/bin:/usr/bin:/bin"

# Pre-create mount target directories as vscode so named volumes initialize with UID 1000 ownership,
# and install Hunk (default diff_command), Claude Code, and Antigravity CLI (agy).
RUN mkdir -p \
        /home/vscode/.local/bin \
        /home/vscode/.cache/go-build \
        /home/vscode/.claude \
        /home/vscode/.config/gh \
        /home/vscode/.gemini \
    && curl -fsSL https://hunk.dev/install.sh | sh \
    && curl -fsSL https://claude.ai/install.sh | bash \
    && curl -fsSL https://antigravity.google/cli/install.sh | bash
