# A fleet node, and the gateway that fronts it: one image, two entry points.
#
# The node is a real `spinloop daemon` supervising a fake engine — Imposter's
# native engine standing in for llama-server, so the stack needs no GPU and no
# model. The gateway is the same binary running `spinloop gateway` over the
# fleet file baked in beside it; compose points each service at its entry.

# Build spinloop from the working tree, so the stack tests THIS commit rather
# than a published artifact.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
# Only what the binary needs, so the image cannot depend on anything else in
# the tree and rebuilds stay cheap.
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/spinloop ./cmd/spinloop

FROM alpine:3.20
ARG TARGETARCH
# Pinned so a node image is reproducible; bump deliberately.
ARG IMPOSTER_VERSION=5.21.3

# procps gives the collector a vmstat whose output matches what it parses;
# BusyBox's differs, so CPU would otherwise be absent.
RUN apk add --no-cache ca-certificates curl bash procps

# Imposter's native engine — a standalone binary, which is what lets the daemon
# supervise it as a child process the way it would a real engine.
RUN curl -fsSL -o /tmp/imposter.tar.gz \
      "https://github.com/imposter-project/imposter-go/releases/download/v${IMPOSTER_VERSION}/imposter-go_linux_${TARGETARCH}.tar.gz" \
  && curl -fsSL -o /tmp/checksums.txt \
      "https://github.com/imposter-project/imposter-go/releases/download/v${IMPOSTER_VERSION}/checksums.txt" \
  && (cd /tmp && grep " imposter-go_linux_${TARGETARCH}.tar.gz\$" checksums.txt | sed 's/imposter-go_linux_.*/imposter.tar.gz/' | sha256sum -c -) \
  && tar -xzf /tmp/imposter.tar.gz -C /usr/local/bin imposter-go \
  && chmod +x /usr/local/bin/imposter-go \
  && rm -f /tmp/imposter.tar.gz /tmp/checksums.txt

COPY --from=build /out/spinloop /usr/local/bin/spinloop
COPY examples/gateway-docker/shim/llama-server /usr/local/bin/llama-server
COPY examples/gateway-docker/engine /opt/engine
# The gateway's own files: the two fleet files it can serve, and the nodes'
# Spinloop source beside them, so a wake resolves what a node runs from inside
# the container. The host-side fleet.yaml points at the same file where it
# lives in the tree, node/Spinloop.
COPY examples/gateway-docker/gateway /opt/gw
COPY examples/gateway-docker/node/Spinloop /opt/gw/Spinloop
RUN chmod +x /usr/local/bin/llama-server

# A container has no useful $HOME, so pin spinloop's config directory — the
# same reason the cloud instance's daemon unit pins it.
ENV SPINLOOP_CONFIG_DIR=/var/lib/spinloop
WORKDIR /opt/node

EXPOSE 4242
# The default entry is the node; the gateway services override it in compose.
# Bind the control API on all interfaces so the gateway container can reach
# it; a token is required for that, supplied per-node by compose.
CMD ["spinloop", "daemon", "--api-addr", "0.0.0.0:4242"]
