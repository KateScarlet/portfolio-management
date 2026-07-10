import { describe, it, expect } from "vitest"
import {
  cn,
  formatCurrency,
  formatPercent,
  toDecimal,
  formatPrice,
  formatCurrencyByCode,
  getProfitColor,
} from "./utils"

describe("cn", () => {
  it("merges class names", () => {
    const result = cn("text-sm", "text-lg")
    expect(result).toBe("text-lg")
  })

  it("handles conditional classes", () => {
    const hidden = false
    const result = cn("base", hidden && "hidden", "extra")
    expect(result).toContain("base")
    expect(result).toContain("extra")
    expect(result).not.toContain("hidden")
  })
})

describe("toDecimal", () => {
  it("converts positive number", () => {
    expect(toDecimal(123).toString()).toBe("123")
  })

  it("converts negative number", () => {
    expect(toDecimal(-42.5).toString()).toBe("-42.5")
  })

  it("converts string number", () => {
    expect(toDecimal("3.14").toString()).toBe("3.14")
  })

  it("returns Decimal(0) for undefined", () => {
    expect(toDecimal(undefined).toString()).toBe("0")
  })

  it("returns Decimal(0) for null", () => {
    expect(toDecimal(null).toString()).toBe("0")
  })

  it("returns Decimal(0) for empty string", () => {
    expect(toDecimal("").toString()).toBe("0")
  })
})

describe("formatPrice", () => {
  it("formats CNY with ¥ symbol and 4 decimals", () => {
    const result = formatPrice(1234.56, "CNY")
    expect(result).toMatch(/^¥1,234\.5600$/)
  })

  it("formats USD with $ symbol", () => {
    const result = formatPrice(100, "USD")
    expect(result).toMatch(/^\$100\.0000$/)
  })

  it("formats HKD with HK$ symbol", () => {
    const result = formatPrice(50, "HKD")
    expect(result).toMatch(/^HK\$50\.0000$/)
  })

  it("uses currency code as prefix for unknown currency", () => {
    const result = formatPrice(100, "BTC")
    expect(result).toMatch(/^BTC100\.0000$/)
  })

  it("handles zero", () => {
    const result = formatPrice(0, "CNY")
    expect(result).toBe("¥0.0000")
  })

  it("handles undefined", () => {
    const result = formatPrice(undefined, "USD")
    expect(result).toBe("$0.0000")
  })

  it("handles null", () => {
    const result = formatPrice(null, "USD")
    expect(result).toBe("$0.0000")
  })

  it("handles negative values", () => {
    const result = formatPrice(-50, "CNY")
    expect(result).toBe("¥-50.0000")
  })
})

describe("formatCurrencyByCode", () => {
  it("formats CNY with ¥ symbol", () => {
    const result = formatCurrencyByCode(1234.56, "CNY")
    expect(result).toMatch(/^¥1,234\.56$/)
  })

  it("formats USD with $ symbol", () => {
    const result = formatCurrencyByCode(100, "USD")
    expect(result).toBe("$100.00")
  })

  it("formats with thousands separator", () => {
    const result = formatCurrencyByCode(1234567.89, "CNY")
    expect(result).toBe("¥1,234,567.89")
  })
})

describe("formatCurrency", () => {
  it("formats positive values", () => {
    const result = formatCurrency(1234.56)
    expect(result).toContain("1")
    expect(result).toContain("234")
    expect(result).toContain("56")
  })

  it("formats zero", () => {
    const result = formatCurrency(0)
    expect(result).toContain("0")
  })

  it("formats negative values", () => {
    const result = formatCurrency(-100)
    expect(result).toContain("100")
  })
})

describe("formatPercent", () => {
  it("formats decimal as percent", () => {
    const result = formatPercent(0.05)
    expect(result).toContain("5")
  })

  it("formats zero", () => {
    const result = formatPercent(0)
    expect(result).toContain("0")
  })

  it("formats negative percent", () => {
    const result = formatPercent(-0.1)
    expect(result).toContain("10")
  })
})

describe("getProfitColor", () => {
  it("returns red for positive in red-up scheme", () => {
    expect(getProfitColor(true, "red-up")).toBe("text-red-600")
  })

  it("returns green for negative in red-up scheme", () => {
    expect(getProfitColor(false, "red-up")).toBe("text-emerald-600")
  })

  it("returns green for positive in green-up scheme", () => {
    expect(getProfitColor(true, "green-up")).toBe("text-emerald-600")
  })

  it("returns orange for negative in green-up scheme", () => {
    expect(getProfitColor(false, "green-up")).toBe("text-orange-600")
  })
})
