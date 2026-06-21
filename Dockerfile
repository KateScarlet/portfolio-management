# ---- Build frontend ----
FROM node:24.17 AS frontend
RUN corepack enable && corepack prepare pnpm@latest --activate
WORKDIR /app
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# ---- Build backend ----
FROM golang:1.26.4 AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o server .

# ---- Runtime ----
FROM chainguard/wolfi-base AS runtime
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /app/server .
COPY --from=frontend /app/dist ./web/dist
EXPOSE 3000
ENTRYPOINT ["./server"]
