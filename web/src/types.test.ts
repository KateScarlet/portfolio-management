import { describe, it, expect } from "vitest"
import {
  AssetId,
  ASSET_DEFINITIONS,
  COMMODITY_SYMBOLS,
  CRYPTO_SYMBOLS,
  MARKET_OPTIONS,
  MarketSourceConfig,
} from "./types"

describe("ASSET_DEFINITIONS", () => {
  it("has all four asset types", () => {
    expect(ASSET_DEFINITIONS.stocks).toBeDefined()
    expect(ASSET_DEFINITIONS.bonds).toBeDefined()
    expect(ASSET_DEFINITIONS.cash).toBeDefined()
    expect(ASSET_DEFINITIONS.commodities).toBeDefined()
  })

  it("each asset has required fields", () => {
    const ids: AssetId[] = ["stocks", "bonds", "cash", "commodities"]
    for (const id of ids) {
      const def = ASSET_DEFINITIONS[id]
      expect(def.id).toBe(id)
      expect(def.name).toBeTruthy()
      expect(def.description).toBeTruthy()
      expect(def.color).toBeTruthy()
      expect(def.targetPct).toBe(25)
    }
  })

  it("target percentages sum to 100", () => {
    const total = Object.values(ASSET_DEFINITIONS).reduce((sum, d) => sum + d.targetPct, 0)
    expect(total).toBe(100)
  })
})

describe("COMMODITY_SYMBOLS", () => {
  it("has entries", () => {
    expect(COMMODITY_SYMBOLS.length).toBeGreaterThan(0)
  })

  it("each has symbol and name", () => {
    for (const c of COMMODITY_SYMBOLS) {
      expect(c.symbol).toBeTruthy()
      expect(c.name).toBeTruthy()
    }
  })
})

describe("CRYPTO_SYMBOLS", () => {
  it("has entries", () => {
    expect(CRYPTO_SYMBOLS.length).toBeGreaterThan(0)
  })

  it("each has symbol and name", () => {
    for (const c of CRYPTO_SYMBOLS) {
      expect(c.symbol).toBeTruthy()
      expect(c.name).toBeTruthy()
    }
  })
})

describe("MARKET_OPTIONS", () => {
  it("includes all required markets", () => {
    const codes = MARKET_OPTIONS.map((m) => m.code)
    expect(codes).toContain("US")
    expect(codes).toContain("CN")
    expect(codes).toContain("HK")
    expect(codes).toContain("FUND")
    expect(codes).toContain("COMMODITY_CN")
    expect(codes).toContain("COMMODITY_INTL")
    expect(codes).toContain("CRYPTO")
    expect(codes).toContain("EXCHANGE")
  })

  it("each option has code and name", () => {
    for (const opt of MARKET_OPTIONS) {
      expect(opt.code).toBeTruthy()
      expect(opt.name).toBeTruthy()
    }
  })

  it("EXCHANGE option has correct properties", () => {
    const exchange = MARKET_OPTIONS.find((m) => m.code === "EXCHANGE")
    expect(exchange).toBeDefined()
    expect(exchange?.name).toBe("汇率")
  })

  it("has unique codes", () => {
    const codes = MARKET_OPTIONS.map((m) => m.code)
    const uniqueCodes = new Set(codes)
    expect(codes.length).toBe(uniqueCodes.size)
  })
})

describe("MarketSourceConfig", () => {
  it("can be constructed with valid data", () => {
    const config: MarketSourceConfig = {
      available: {
        US: ["eastmoney", "sina", "yahoo"],
        CN: ["eastmoney", "tencent", "sina", "yahoo"],
        EXCHANGE: ["eastmoney", "sina", "yahoo"],
      },
      config: {
        US: ["yahoo", "sina"],
        EXCHANGE: ["eastmoney", "yahoo"],
      },
      sourceNames: {
        eastmoney: "东方财富",
        sina: "新浪财经",
        yahoo: "雅虎财经",
      },
    }

    expect(config.available.US).toContain("eastmoney")
    expect(config.available.EXCHANGE).toContain("eastmoney")
    expect(config.config?.US).toEqual(["yahoo", "sina"])
    expect(config.config?.EXCHANGE).toEqual(["eastmoney", "yahoo"])
    expect(config.sourceNames.eastmoney).toBe("东方财富")
  })

  it("config can be null", () => {
    const config: MarketSourceConfig = {
      available: { US: ["eastmoney"] },
      config: null,
      sourceNames: { eastmoney: "东方财富" },
    }
    expect(config.config).toBeNull()
  })
})
