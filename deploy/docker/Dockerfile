# DopeAgent daemon image (Rust).
# Build context is the REPO ROOT (needs the rs/ workspace):
#   docker build -f deploy/docker/Dockerfile -t dope-agent .
# or just use deploy/docker/docker-compose.yml.

FROM rust:1.85-bookworm AS build
WORKDIR /src/crates
COPY crates/ ./
ARG DOPE_VERSION=docker
RUN cargo build --release -p dope-cli

FROM alpine:3.20
# ca-certificates: outbound HTTPS to LLM/providers. wget: container HEALTHCHECK.
RUN apk add --no-cache ca-certificates wget \
 && adduser -D -u 10001 dope \
 && mkdir -p /data && chown dope:dope /data
COPY --from=build /src/crates/target/release/dope-cli /usr/local/bin/dope

ENV DOPE_ENV=prod \
    DOPE_DATA_DIR=/data \
    DOPE_BIND_ADDR=0.0.0.0:19191 \
    DOPE_LOG_LEVEL=info
VOLUME ["/data"]
EXPOSE 19191
USER dope

HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:19191/healthz" >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/usr/local/bin/dope"]
