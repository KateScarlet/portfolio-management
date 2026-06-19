import { describe, it, expect } from "vitest"
import { cn, formatCurrency, formatPercent } from "./utils"

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
})
