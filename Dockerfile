# ---- Build frontend ----
FROM ghcr.io/voidzero-dev/vite-plus:latest AS frontend
WORKDIR /app
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    vp install --frozen-lockfile
COPY web/ ./
RUN vp build

# ---- Build backend ----
FROM golang:1.26.5 AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
COPY --from=frontend /app/dist ./web/dist
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

# ---- Runtime ----
FROM chainguard/static:latest AS runtime
WORKDIR /app
COPY --from=backend /app/bin/server .
EXPOSE 3000
ENTRYPOINT ["./server"]
