import type { LucideIcon } from "lucide-react"
import { CandlestickChart, ScrollText, Banknote, Gem } from "lucide-react"

export type AssetId = "stocks" | "bonds" | "cash" | "commodities"

export interface AvailableFund {
  currency: string
  amount: string
}

export interface FundTransaction {
  id: string
  userId: string
  portfolioId: string
  type:
    | "transfer_in"
    | "transfer_out"
    | "transfer"
    | "convert"
    | "buy"
    | "sell"
    | "dividend"
    | "dividend_reinvest"
  amount: string
  currency: string
  targetPortfolioId?: string
  targetAmount?: string
  targetCurrency?: string
  exchangeRate?: string
  holdingId?: string
  note?: string
  createdAt: number
}

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

export interface Account {
  id: string
  userId: string
  name: string
  description?: string
  broker?: string
  createdAt: number
}

export interface HoldingWithAccount extends Holding {
  accountName: string
}

export interface MergedHoldingAccount {
  holdingId: string
  accountId: string
  accountName: string
  shares: string
  cost: string
  value: string
  lots?: HoldingLot[]
}

export interface MergedHolding extends Holding {
  accounts: MergedHoldingAccount[]
}

export interface PortfolioSummaryItem {
  id: string
  name: string
  total: string
  principal: string
  assets: Record<AssetId, string>
}

export interface PortfolioSummary {
  total: string
  principal: string
  assets: Record<AssetId, string>
  portfolios: PortfolioSummaryItem[]
}

export interface AssetInfo {
  id: AssetId
  name: string
  description: string
  color: string
  targetPct: number
  icon: LucideIcon
}

export interface HoldingLot {
  id: string
  type?: string // 'sell' for sell records, empty/undefined for buy
  date: string
  shares: string
  costPrice?: string
  cost?: string
  valueAdded?: string
  fee?: string // 手续费
}

export interface Holding {
  id: string
  portfolioId?: string
  assetId: AssetId
  symbol: string // empty if manual value
  name?: string
  market?: string // e.g. "US", "CN", "HK", "FUND", "COMMODITY_CN", "COMMODITY_INTL", "CRYPTO"
  currency: string
  shares: string
  price: string
  costPrice?: string // Avg cost per share for calculated holdings
  value: string // Current total value
  cost?: string // Total cost basis
  totalDividends?: string // 累计分红金额
  date?: string // Original purchase date for the lot being added
  fee?: string // 手续费（仅用于创建时传递）
  lots?: HoldingLot[]
  accountId?: string // 所属账户ID
}

export interface HoldingSnapshot {
  assetId: AssetId
  symbol: string
  name: string
  currency: string
  shares: string
  price: string
  costPrice: string
  value: string
  cost: string
}

export interface PortfolioRecord {
  id: string
  timestamp: number
  assets: Record<AssetId, string>
  holdings: HoldingSnapshot[]
  total: string
  principal: string
}

export type ColorScheme = "green-up" | "red-up"

export interface Settings {
  driftThreshold: number // 漂移阈值百分比，如 5 表示 5%
  syncInterval: number // 同步间隔（分钟），0 = 关闭
  colorScheme: ColorScheme // 红涨绿跌 or 绿涨红跌
  displayCurrency: string // 显示币种，如 "CNY"、"USD"
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
  syncInterval: 30,
  colorScheme: "green-up",
  displayCurrency: "CNY",
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
    icon: CandlestickChart,
  },
  bonds: {
    id: "bonds",
    name: "长期债券",
    description: "提供通货紧缩保护（如TLT）",
    color: "#868E96", // gray
    targetPct: 25,
    icon: ScrollText,
  },
  cash: {
    id: "cash",
    name: "现金",
    description: "提供流动性和衰退保护（如SHV, 货基）",
    color: "#E9ECEF", // silver/cash
    targetPct: 25,
    icon: Banknote,
  },
  commodities: {
    id: "commodities",
    name: "商品",
    description: "提供通货膨胀保护（如黄金, 能源, 农产品等）",
    color: "#D4AF37",
    targetPct: 25,
    icon: Gem,
  },
}

export const COMMODITY_CN_SYMBOLS = [
  { symbol: "au9999", name: "黄金9999" },
  { symbol: "agtd", name: "白银T+D" },
  { symbol: "scm", name: "原油" },
  { symbol: "cum", name: "沪铜" },
] as const

export const COMMODITY_INTL_SYMBOLS = [
  { symbol: "GC=F", name: "黄金" },
  { symbol: "SI=F", name: "白银" },
  { symbol: "CL=F", name: "原油 (WTI)" },
  { symbol: "NG=F", name: "天然气" },
  { symbol: "HG=F", name: "铜" },
] as const

export const COMMODITY_SYMBOLS = [...COMMODITY_CN_SYMBOLS, ...COMMODITY_INTL_SYMBOLS] as const

export const CRYPTO_SYMBOLS = [
  { symbol: "BTC-USD", name: "Bitcoin" },
  { symbol: "ETH-USD", name: "Ethereum" },
  { symbol: "BNB-USD", name: "BNB" },
  { symbol: "SOL-USD", name: "SOL" },
  { symbol: "XRP-USD", name: "XRP" },
  { symbol: "DOGE-USD", name: "Dogecoin" },
  { symbol: "ADA-USD", name: "Cardano" },
  { symbol: "DOT-USD", name: "Polkadot" },
  { symbol: "AVAX-USD", name: "Avalanche" },
  { symbol: "LINK-USD", name: "Chainlink" },
] as const

export const MARKET_OPTIONS = [
  { code: "US", name: "美股" },
  { code: "CN", name: "A股" },
  { code: "HK", name: "港股" },
  { code: "FUND", name: "场外基金" },
  { code: "COMMODITY_CN", name: "商品(国内)" },
  { code: "COMMODITY_INTL", name: "商品(国际)" },
  { code: "CRYPTO", name: "加密货币" },
  { code: "EXCHANGE", name: "汇率" },
] as const

export interface MarketSourceConfig {
  available: Record<string, string[]>
  config: Record<string, string[]> | null
  sourceNames: Record<string, string>
}

export interface SourceTestResult {
  success: boolean
  name?: string
  price?: string
  currency?: string
  rate?: string
  latency?: number
  error?: string
  symbol?: string
}

export interface TestSourcesResult {
  results: Record<string, SourceTestResult>
}

export interface SourceTestComplete {
  total: number
  success: number
  failed: number
}

export interface Dividend {
  id: string
  userId: string
  portfolioId: string
  holdingId: string
  assetId: string
  symbol: string
  amount: string
  taxWithheld: string
  netAmount: string
  currency: string
  sharesHeld: string
  dividendPerShare: string
  exDate?: number
  payDate?: number
  reinvest: boolean
  reinvestPrice: string
  reinvestShares: string
  holdingLotId?: string
  fundTxId?: string
  note?: string
  createdAt: number
}
