# Portfolio Management

个人投资组合管理系统，用于跟踪多市场、多币种的投资持仓，支持自动价格同步和实时通知推送。

## 功能特性

- **多投资组合 & 多账户** — 支持创建多个投资组合和券商账户，独立管理
- **多市场行情** — A股、港股、美股、加密货币、基金、大宗商品，多数据源自动回退
- **精确记账** — 基于交易批次（Lot）的记账模型，持仓成本、收益、分红自动计算
- **资金管理** — 入金、出金、跨组合转账、币种换算，完整资金流水
- **实时同步** — SSE 实时推送价格更新，可配置自动同步间隔
- **通知推送** — Telegram / Bark 推送价格预警、偏离预警、组合周报
- **多种认证** — 用户名密码、OIDC/SSO、WebAuthn 通行密钥
- **再平衡面板** — 目标资产配置 vs 实际配置可视化，偏离自动告警
- **多币种支持** — 持仓、资金、净值均支持多币种，可配置显示货币

## 快速开始

### Docker Compose（推荐）

```bash
# 克隆项目
git clone <repo-url> && cd portfolio-management

# 启动
docker compose up -d
```

首次访问 `http://localhost:3000` 时，Web 引导界面会指引你完成数据库配置和管理员账户创建。

### 本地开发

```bash
# 后端
cp config.yaml.example config/config.yaml
# 编辑 config/config.yaml 填写数据库连接
go run ./cmd/server

# 前端（另开终端）
cd web && pnpm install && pnpm dev
```

## 配置

配置文件位于 `config/config.yaml`，首次启动可通过 Web 界面自动生成。

```yaml
# 数据库
database:
  type: postgres
  dsn: postgres://user:pass@localhost:5432/portfolio?sslmode=disable

# OIDC/SSO（可选）
oidc:
  enabled: true
  issuer: https://your-provider.example.com
  clientID: your-client-id
  clientSecret: your-client-secret
  redirectURL: http://localhost:3000/api/auth/oidc/callback
```

## 行情数据源

系统支持以下数据源，每个市场可配置多个源按优先级回退：

| 数据源 | 支持市场 |
|--------|----------|
| 东方财富 | A股、港股、美股、基金、国内大宗商品 |
| 雅虎财经 | 美股、港股、A股、加密货币、国际大宗商品、汇率 |
| 新浪财经 | 美股、A股、港股、汇率 |
| 腾讯财经 | A股、港股、基金 |
| CoinGecko | 加密货币 |

## 通知渠道

| 渠道 | 说明 |
|------|------|
| Telegram | 通过 Bot API 推送，支持文本和图文消息 |
| Bark | iOS 推送通知，支持自建服务端 |

