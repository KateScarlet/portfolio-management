set shell := ["brush", "-c"]

# 默认构建全部
default: build

go_binary := "server.exe"
frontend_dir := "web"
dist_dir := frontend_dir / "dist"

# 构建全部（前端 + Go）
build: build-frontend build-go

# 构建 Go 后端
build-go:
    go build -o {{go_binary}} .

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

# TypeScript 类型检查
lint:
    cd {{frontend_dir}} && pnpm exec tsc --noEmit
