set shell := ["brush", "-c"]

# 默认构建全部
default: build

go_binary := "bin/server"
frontend_dir := "web"
dist_dir := frontend_dir / "dist"

# 构建全部（前端 + Go）
build: build-frontend build-backend build-backend-windows

# 构建 Go 后端
build-backend:
    go build -trimpath -ldflags="-s -w" -o {{go_binary}} ./cmd/server

# 交叉编译 Windows 版本 (amd64)
build-backend-windows:
    GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o {{go_binary}}.exe ./cmd/server

# 构建前端
build-frontend:
    cd {{frontend_dir}} && vp build

# 构建并运行生产模式
run: build
    ./{{go_binary}}

# 同时启动 Go 后端 + Vite 前端开发服务器
dev:
    just dev-backend & just dev-frontend

# 仅启动 Go 后端
dev-backend:
    air

# 仅启动 Vite 前端
dev-frontend:
    cd {{frontend_dir}} && vp dev

# 清理构建产物 + database
clean:
    rm -rf bin/
    rm -rf {{dist_dir}}

# 整理 Go 依赖
tidy:
    go mod tidy

# 代码检查（全部）
lint: lint-backend lint-frontend

# Go 代码检查
lint-backend:
    golangci-lint run

# 前端代码检查
lint-frontend:
    cd {{frontend_dir}} && lint lint .

# 代码格式化
fmt: fmt-backend fmt-frontend

# Go 代码格式化
fmt-backend:
    gofmt -s -w .

# 前端代码格式化
fmt-frontend:
    cd {{frontend_dir}} && vp fmt --ignore-path .oxfmtignore .

# 前端格式检查
fmt-check:
    cd {{frontend_dir}} && vp fmt --check --ignore-path .oxfmtignore .
