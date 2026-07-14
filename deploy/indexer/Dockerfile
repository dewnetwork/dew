# Multi-stage build for dewindex (history indexer sidecar).
# Build from repo root:
#   docker build -f deploy/indexer/Dockerfile -t dew-indexer:local .

FROM golang:1.25-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
  -ldflags="-s -w -X github.com/dewnetwork/dew/version.Version=${VERSION}" \
  -o /out/dewindex ./cmd/dewindex

FROM debian:bookworm-slim
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/dewindex /usr/local/bin/dewindex

EXPOSE 8550
ENTRYPOINT ["dewindex"]
CMD ["-rpc", "http://dew-node:8545", "-db", "/var/lib/dewindex/indexer.db", "-http.addr", "0.0.0.0:8550"]
