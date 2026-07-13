# Multi-stage build for the Dew full node (cmd/dew).
# Build from repo root:
#   docker build -f deploy/node/Dockerfile -t dew:local .

FROM golang:1.25-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# VERSION is software semver (no required leading v); inject via build-arg on release.
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
  -ldflags="-s -w -X github.com/dewnetwork/dew/version.Version=${VERSION}" \
  -o /out/dew ./cmd/dew

FROM debian:bookworm-slim
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/dew /usr/local/bin/dew
# Select genesis at build time: genesis.json (dev) or genesis.public.json (public stack).
ARG GENESIS_FILE=genesis.json
COPY ${GENESIS_FILE} /etc/dew/genesis.json

EXPOSE 8545 30303
ENTRYPOINT ["dew"]
CMD ["run", "--genesis", "/etc/dew/genesis.json", "--http.addr", "0.0.0.0", "--http.port", "8545"]
