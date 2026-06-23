export type AssetId = "stocks" | "bonds" | "cash" | "commodities"

export interface UserInfo {
  id: string
  username: string
  role: "admin" | "user"
}

export interface Portfolio {
  id: string
  userId: string
  name: string
  description?: string
  isDefault: boolean
  createdAt: number
}

export interface PortfolioSummaryItem {
  id: string
  name: string
  total: number
  principal: number
  assets: Record<AssetId, number>
}

export interface PortfolioSummary {
  total: number
  principal: number
  assets: Record<AssetId, number>
  portfolios: PortfolioSummaryItem[]
}

export interface AssetInfo {
  id: AssetId
  name: string
  description: string
  color: string
  targetPct: number
}

export interface HoldingLot {
  id: string
  type?: string // 'sell' for sell records, empty/undefined for buy
  date: number
  shares: number
  costPrice?: number
  cost?: number
  valueAdded?: number
  fee?: number // 手续费
}

export interface Holding {
  id: string
  assetId: AssetId
  symbol: string // empty if manual value
  name?: string
  shares: number
  price: number
  costPrice?: number // Avg cost per share for calculated holdings
  value: number // Current total value
  cost?: number // Total cost basis
  date?: number // Original purchase date for the lot being added
  fee?: number // 手续费（仅用于创建时传递）
  lots?: HoldingLot[]
  deductFromCash?: boolean // 从现金扣除本金（仅用于创建时传递）
}

export interface HoldingSnapshot {
  assetId: AssetId
  symbol: string
  name: string
  shares: number
  price: number
  costPrice: number
  value: number
  cost: number
}

export interface PortfolioRecord {
  id: string
  timestamp: number
  assets: Record<AssetId, number>
  holdings: HoldingSnapshot[]
  total: number
  principal: number
}

export type ColorScheme = "green-up" | "red-up"

export interface Settings {
  driftThreshold: number // 漂移阈值百分比，如 5 表示 5%
  syncInterval: number // 同步间隔（分钟），0 = 关闭
  colorScheme: ColorScheme // 红涨绿跌 or 绿涨红跌
  // Target allocation
  targetStocks: number
  targetBonds: number
  targetCash: number
  targetCommodities: number
  // Telegram
  telegramBotToken: string
  telegramChatID: string
  telegramEnabled: boolean
  telegramPriceAlert: boolean
  telegramDriftAlert: boolean
  telegramSummary: boolean
  telegramPriceThreshold: number // 价格波动阈值百分比
  telegramSummaryInterval: string // "daily" | "weekly" | "off"
}

export const DEFAULT_SETTINGS: Settings = {
  driftThreshold: 5,
  syncInterval: 60,
  colorScheme: "green-up",
  targetStocks: 25,
  targetBonds: 25,
  targetCash: 25,
  targetCommodities: 25,
  telegramBotToken: "",
  telegramChatID: "",
  telegramEnabled: false,
  telegramPriceAlert: true,
  telegramDriftAlert: true,
  telegramSummary: true,
  telegramPriceThreshold: 5,
  telegramSummaryInterval: "daily",
}

export interface SyncStatus {
  lastSyncAt: string // ISO timestamp
  lastSyncErr?: string
  syncing: boolean
}

export const ASSET_DEFINITIONS: Record<AssetId, AssetInfo> = {
  stocks: {
    id: "stocks",
    name: "股票",
    description: "提供高增长潜力（如VTI, SPY）",
    color: "#1A1A1A", // black
    targetPct: 25,
  },
  bonds: {
    id: "bonds",
    name: "长期债券",
    description: "提供通货紧缩保护（如TLT）",
    color: "#868E96", // gray
    targetPct: 25,
  },
  cash: {
    id: "cash",
    name: "现金",
    description: "提供流动性和衰退保护（如SHV, 货基）",
    color: "#E9ECEF", // silver/cash
    targetPct: 25,
  },
  commodities: {
    id: "commodities",
    name: "商品",
    description: "提供通货膨胀保护（如黄金, 能源, 农产品等）",
    color: "#D4AF37",
    targetPct: 25,
  },
}

export const COMMODITY_SYMBOLS = [
  { symbol: "GC=F", name: "黄金" },
  { symbol: "SI=F", name: "白银" },
  { symbol: "CL=F", name: "原油 (WTI)" },
  { symbol: "NG=F", name: "天然气" },
  { symbol: "HG=F", name: "铜" },
] as const

export const CRYPTO_SYMBOLS = [
  { symbol: "BTC-USD", name: "Bitcoin" },
  { symbol: "ETH-USD", name: "Ethereum" },
  { symbol: "BNB-USD", name: "BNB" },
  { symbol: "SOL-USD", name: "Solana" },
  { symbol: "XRP-USD", name: "XRP" },
  { symbol: "DOGE-USD", name: "Dogecoin" },
  { symbol: "ADA-USD", name: "Cardano" },
  { symbol: "DOT-USD", name: "Polkadot" },
  { symbol: "AVAX-USD", name: "Avalanche" },
  { symbol: "LINK-USD", name: "Chainlink" },
] as const
