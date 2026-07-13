# Multi-stage build for the Dew faucet service (cmd/dewfaucet).
# Build from repo root:
#   docker build -f deploy/faucet/Dockerfile -t dew-faucet:local .

FROM golang:1.25-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
  -ldflags="-s -w -X github.com/dewnetwork/dew/version.Version=${VERSION}" \
  -o /out/dewfaucet ./cmd/dewfaucet

FROM debian:bookworm-slim
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/dewfaucet /usr/local/bin/dewfaucet

# Set standard environment defaults
ENV FAUCET_LISTEN=0.0.0.0:8080

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/dewfaucet"]
