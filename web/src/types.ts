export type AssetId = 'stocks' | 'bonds' | 'cash' | 'gold';

export interface AssetInfo {
  id: AssetId;
  name: string;
  description: string;
  color: string;
  targetPct: number;
}

export interface HoldingLot {
  id: string;
  type?: string;        // 'sell' for sell records, empty/undefined for buy
  date: number;
  shares: number;
  costPrice?: number;
  cost?: number;
  valueAdded?: number;
}

export interface Holding {
  id: string;
  assetId: AssetId;
  symbol: string;      // empty if manual value
  name?: string;
  shares: number;
  price: number;
  costPrice?: number;  // Avg cost per share for calculated holdings
  value: number;       // Current total value
  cost?: number;       // Total cost basis
  date?: number;       // Original purchase date for the lot being added
  lots?: HoldingLot[];
}

export interface PortfolioRecord {
  id: string;
  timestamp: number;
  assets: Record<AssetId, number>; // total value per asset category at the time
  total: number;
  principal?: number;
  // We don't necessarily need to store holdings in history if we only want aggregate history, but it's okay.
}

export interface Settings {
  driftThreshold: number; // 漂移阈值百分比，如 5 表示 5%
  syncInterval: number;   // 同步间隔（分钟），0 = 关闭
}

export const DEFAULT_SETTINGS: Settings = {
  driftThreshold: 5,
  syncInterval: 60,
};

export interface SyncStatus {
  lastSyncAt: string;     // ISO timestamp
  lastSyncErr?: string;
  syncing: boolean;
}

export const ASSET_DEFINITIONS: Record<AssetId, AssetInfo> = {
  stocks: {
    id: 'stocks',
    name: '股票',
    description: '提供高增长潜力（如VTI, SPY）',
    color: '#1A1A1A', // black
    targetPct: 25,
  },
  bonds: {
    id: 'bonds',
    name: '长期债券',
    description: '提供通货紧缩保护（如TLT）',
    color: '#868E96', // gray
    targetPct: 25,
  },
  cash: {
    id: 'cash',
    name: '现金',
    description: '提供流动性和衰退保护（如SHV, 货基）',
    color: '#E9ECEF', // silver/cash
    targetPct: 25,
  },
  gold: {
    id: 'gold',
    name: '商品',
    description: '提供通货膨胀保护（如黄金, 能源, 农产品等）',
    color: '#D4AF37', // gold
    targetPct: 25,
  },
};

export const COMMODITY_SYMBOLS = [
  { symbol: 'GC=F', name: '黄金' },
  { symbol: 'SI=F', name: '白银' },
  { symbol: 'CL=F', name: '原油 (WTI)' },
  { symbol: 'NG=F', name: '天然气' },
  { symbol: 'HG=F', name: '铜' },
] as const;
