FROM golang:1.25-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /amlens ./cmd/amlens

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -g 10001 app && adduser -D -u 10001 -G app app \
    && mkdir -p /var/lib/amlens && chown app:app /var/lib/amlens
COPY --from=build /amlens /usr/local/bin/amlens
COPY LICENSE THIRD_PARTY_NOTICES.md /usr/share/licenses/amlens/
COPY third_party/ /usr/share/licenses/amlens/third_party/
USER 10001:10001
WORKDIR /var/lib/amlens
ENV ANALYSIS_DATABASE=/var/lib/amlens/analyses.db ANALYSIS_STORAGE_DIR=/tmp/uploads
EXPOSE 8000
HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=5 CMD wget -q -O /dev/null http://127.0.0.1:8000/api/health || exit 1
ENTRYPOINT ["amlens"]
CMD ["serve", "--addr", "0.0.0.0:8000"]
