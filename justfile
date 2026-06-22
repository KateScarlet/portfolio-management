set shell := ["brush", "-c"]

# 默认构建全部
default: build

go_binary := "server"
frontend_dir := "web"
dist_dir := frontend_dir / "dist"

# 构建全部（前端 + Go）
build: build-frontend build-go build-go-windows

# 构建 Go 后端
build-go:
    go build -trimpath -ldflags="-s -w" -o {{go_binary}} .

# 交叉编译 Windows 版本 (amd64)
build-go-windows:
    GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o {{go_binary}}.exe .

# 构建前端
build-frontend:
    cd {{frontend_dir}} && pnpm build

# 构建并运行生产模式
run: build
    ./{{go_binary}}

# 同时启动 Go 后端 + Vite 前端开发服务器
dev:
    just dev-go & just dev-frontend

# 仅启动 Go 后端
dev-go:
    air

# 仅启动 Vite 前端
dev-frontend:
    cd {{frontend_dir}} && pnpm dev

# 清理构建产物 + database
clean:
    rm -f {{go_binary}}
    rm -rf {{dist_dir}}
    rm -f portfolio.db

# 整理 Go 依赖
tidy:
    go mod tidy

# 代码检查（全部）
lint: lint-go lint-frontend

# Go 代码检查
lint-go:
    golangci-lint run

# 前端代码检查
lint-frontend:
    cd {{frontend_dir}} && pnpm lint
    cd {{frontend_dir}} && pnpm typecheck

# 代码格式化
fmt: fmt-go fmt-frontend

# Go 代码格式化
fmt-go:
    gofmt -s -w .

# 前端代码格式化
fmt-frontend:
    cd {{frontend_dir}} && pnpm format

# 前端格式检查
fmt-check:
    cd {{frontend_dir}} && pnpm format:check
