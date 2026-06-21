# 投资组合管理系统 - 前端

基于 Harry Browne 投资组合模型的投资跟踪工具前端。

## 技术栈

- React 19 + TypeScript
- Vite
- Tailwind CSS 4
- Recharts（图表）
- Vitest（测试）

## 开发

### 安装依赖
```bash
pnpm install
```

### 启动开发服务器
```bash
pnpm dev
```

### 构建生产版本
```bash
pnpm build
```

### 运行测试
```bash
pnpm test
```

### 代码检查
```bash
pnpm lint
pnpm format
pnpm typecheck
```

## 项目结构

- `src/` - 源代码
  - `components/` - React 组件
  - `api.ts` - API 客户端
  - `types.ts` - TypeScript 类型定义
  - `utils.ts` - 工具函数
  - `usePortfolio.ts` - 组合数据管理 Hook
