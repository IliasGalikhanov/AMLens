FROM node:24-alpine AS build
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM caddy:2-alpine
RUN setcap -r /usr/bin/caddy && mkdir -p /data /config /srv && chown -R 10001:10001 /data /config /srv
COPY --from=build /src/dist /srv
COPY deploy/Caddyfile /etc/caddy/Caddyfile
COPY LICENSE THIRD_PARTY_NOTICES.md /usr/share/licenses/amlens/
COPY third_party/ /usr/share/licenses/amlens/third_party/
USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
