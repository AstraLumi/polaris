# The two build stages run on the building machine's own platform and the Go
# compiler targets the image's, so building for ARM needs no emulation.

# --- Stage 1: build the frontend ---
FROM --platform=$BUILDPLATFORM node:26-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# --- Stage 2: build the Go binary, embedding the compiled frontend ---
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS backend-build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app/backend
COPY backend/ ./
COPY --from=frontend-build /app/frontend/dist ./static
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o polaris .

# --- Stage 3: minimal runtime image ---
FROM alpine:3.24
# su-exec: how the entrypoint gives up root for the polaris user.
RUN apk add --no-cache ca-certificates su-exec  && addgroup -S -g 10001 polaris  && adduser -S -D -H -u 10001 -G polaris -s /sbin/nologin polaris
WORKDIR /app
COPY --from=backend-build /app/backend/polaris .
COPY docker/entrypoint.sh /app/entrypoint.sh
RUN chmod 755 /app/entrypoint.sh

ENV DATA_DIR=/data
ENV UPLOADS_DIR=/app/uploads
ENV PORT=8091

EXPOSE 8091
VOLUME ["/data", "/app/uploads"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD wget -qO- http://127.0.0.1:8091/api/health >/dev/null || exit 1

# Polaris runs as the polaris user, never as root (docker/entrypoint.sh).
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["./polaris"]
