# Multi-stage build for Dew Guestbook SPA (examples/guestbook-web).
# From repo root:
#   docker build -f deploy/guestbook/Dockerfile \
#     --build-arg PUBLIC_RPC_URL=https://rpc-dew.fadosoft.com \
#     --build-arg PUBLIC_GUESTBOOK=0x… \
#     -t dew-guestbook:local .

FROM node:20-alpine AS build
WORKDIR /app

RUN npm install -g pnpm

COPY examples/guestbook-web/package.json examples/guestbook-web/pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile

COPY examples/guestbook-web/ ./

ARG PUBLIC_RPC_URL=https://rpc-dew.fadosoft.com
ARG PUBLIC_GUESTBOOK=0x83bB4E539BE46503481E66094b01b854990BF84a
ARG PUBLIC_EXPLORER_URL=https://explorer-dew.fadosoft.com
ARG PUBLIC_CHAIN_ID=2205

ENV PUBLIC_RPC_URL=$PUBLIC_RPC_URL
ENV PUBLIC_GUESTBOOK=$PUBLIC_GUESTBOOK
ENV PUBLIC_EXPLORER_URL=$PUBLIC_EXPLORER_URL
ENV PUBLIC_CHAIN_ID=$PUBLIC_CHAIN_ID

RUN pnpm build

FROM nginx:stable-alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY deploy/guestbook/nginx/dew-guestbook.docker.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
