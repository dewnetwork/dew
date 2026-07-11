# Multi-stage build for the Dew block explorer SPA (explorer/).
# Build from repo root:
#   docker build -f deploy/explorer/Dockerfile \
#     --build-arg PUBLIC_RPC_URL=https://rpc.example.com \
#     --build-arg PUBLIC_CHAIN_ID=2205 \
#     --build-arg PUBLIC_EXPLORER_BASE=https://explorer.example.com \
#     -t dew-explorer:local .

FROM node:20-alpine AS build
WORKDIR /app

# Install pnpm
RUN npm install -g pnpm

# Copy project manifests from explorer folder
COPY explorer/package.json explorer/pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile

# Copy the frontend application source
COPY explorer/ ./

# Build-time env for Vite (PUBLIC_* only — no secrets)
ARG PUBLIC_RPC_URL=http://127.0.0.1:8545
ARG PUBLIC_CHAIN_ID=2205
ARG PUBLIC_EXPLORER_BASE=
ARG PUBLIC_DEW_PRICE_USD=
ARG PUBLIC_DEW_PRICE_BTC=
ARG PUBLIC_DEW_PRICE_CHANGE_24H=
ARG PUBLIC_DEW_MARKET_CAP_USD=
ARG PUBLIC_DEW_TOTAL_SUPPLY=

ENV PUBLIC_RPC_URL=$PUBLIC_RPC_URL
ENV PUBLIC_CHAIN_ID=$PUBLIC_CHAIN_ID
ENV PUBLIC_EXPLORER_BASE=$PUBLIC_EXPLORER_BASE
ENV PUBLIC_DEW_PRICE_USD=$PUBLIC_DEW_PRICE_USD
ENV PUBLIC_DEW_PRICE_BTC=$PUBLIC_DEW_PRICE_BTC
ENV PUBLIC_DEW_PRICE_CHANGE_24H=$PUBLIC_DEW_PRICE_CHANGE_24H
ENV PUBLIC_DEW_MARKET_CAP_USD=$PUBLIC_DEW_MARKET_CAP_USD
ENV PUBLIC_DEW_TOTAL_SUPPLY=$PUBLIC_DEW_TOTAL_SUPPLY

# Build production static bundle
RUN pnpm build

# Serve stage via Nginx
FROM nginx:stable-alpine
COPY --from=build /app/dist /usr/share/nginx/html

# SPA: client-side routes + static assets
COPY deploy/explorer/nginx/dew-explorer.docker.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
