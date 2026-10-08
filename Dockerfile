# --- Stage 1: build the frontend ---
FROM node:20-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# --- Stage 2: build the Go binary, embedding the compiled frontend ---
FROM golang:1.22-alpine AS backend-build
WORKDIR /app/backend
COPY backend/ ./
COPY --from=frontend-build /app/frontend/dist ./static
# go.sum should be committed; if it isn't yet, generate it here.
RUN if [ ! -f go.sum ]; then go mod tidy; fi \
    && CGO_ENABLED=0 go build -o polaris .

# --- Stage 3: minimal runtime image ---
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend-build /app/backend/polaris .

ENV DATA_DIR=/data
ENV UPLOADS_DIR=/app/uploads
ENV PORT=8091

EXPOSE 8091
VOLUME ["/data", "/app/uploads"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD wget -qO- http://127.0.0.1:8091/api/health >/dev/null || exit 1

CMD ["./polaris"]
