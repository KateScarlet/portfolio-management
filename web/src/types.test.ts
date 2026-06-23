import { describe, it, expect } from "vitest"
import { AssetId, ASSET_DEFINITIONS, COMMODITY_SYMBOLS, CRYPTO_SYMBOLS } from "./types"

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
