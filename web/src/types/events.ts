export interface SSEEvent {
  type: string
  portfolioId: string
  data: unknown
  timestamp: string
}

export interface SyncStartedData {
  portfolioId: string
}

export interface SyncCompletedData {
  portfolioId: string
  lastSyncAt: string
  syncedCount: number
  failedCount: number
}

export interface SyncFailedData {
  portfolioId: string
  error: string
}

export interface HoldingUpdate {
  symbol: string
  price: string
  value: string
}

export interface PriceUpdatedData {
  portfolioId: string
  holdings: HoldingUpdate[]
}
