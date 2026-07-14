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
# Optional P1e sidecar (path B: https://explorer-dew.fadosoft.com/indexer)
ARG PUBLIC_INDEXER_URL=
ARG PUBLIC_DEW_PRICE_USD=
ARG PUBLIC_DEW_PRICE_BTC=
ARG PUBLIC_DEW_PRICE_CHANGE_24H=
ARG PUBLIC_DEW_MARKET_CAP_USD=
ARG PUBLIC_DEW_TOTAL_SUPPLY=
# Path B mock basket default (ops surface — override when redeploying tokens).
ARG PUBLIC_KNOWN_TOKENS=USDT:0x43d08b71B0A5626a9D0eB607C2aE5277255cFbC8,USDC:0xacDd0BF26C96879b4c4CeE8F92928f1760F07119,DAI:0x9813B1738eb2982F3D0C1E22F9C7c4F07B670a1B,WETH:0x5b88b2ab38920197328f9D90BaAf29cb91F7E3CB,WBTC:0xdB6E09cb954Ae645C815Bc941bDfD85D4144166E

ENV PUBLIC_RPC_URL=$PUBLIC_RPC_URL
ENV PUBLIC_CHAIN_ID=$PUBLIC_CHAIN_ID
ENV PUBLIC_EXPLORER_BASE=$PUBLIC_EXPLORER_BASE
ENV PUBLIC_INDEXER_URL=$PUBLIC_INDEXER_URL
ENV PUBLIC_DEW_PRICE_USD=$PUBLIC_DEW_PRICE_USD
ENV PUBLIC_DEW_PRICE_BTC=$PUBLIC_DEW_PRICE_BTC
ENV PUBLIC_DEW_PRICE_CHANGE_24H=$PUBLIC_DEW_PRICE_CHANGE_24H
ENV PUBLIC_DEW_MARKET_CAP_USD=$PUBLIC_DEW_MARKET_CAP_USD
ENV PUBLIC_DEW_TOTAL_SUPPLY=$PUBLIC_DEW_TOTAL_SUPPLY
ENV PUBLIC_KNOWN_TOKENS=$PUBLIC_KNOWN_TOKENS

# Build production static bundle
RUN pnpm build

# Serve stage via Nginx
FROM nginx:stable-alpine
COPY --from=build /app/dist /usr/share/nginx/html

# SPA: client-side routes + static assets
COPY deploy/explorer/nginx/dew-explorer.docker.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
