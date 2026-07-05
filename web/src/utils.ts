import Decimal from "decimal.js"
import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import { ColorScheme } from "./types"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function toDecimal(value: string | number | undefined | null): Decimal {
  if (value === undefined || value === null || value === "") return new Decimal(0)
  return new Decimal(value)
}

const CURRENCY_SYMBOLS: Record<string, string> = {
  CNY: "¥",
  USD: "$",
  HKD: "HK$",
  EUR: "€",
  GBP: "£",
  JPY: "¥",
}

function formatDecimal(value: string | number | undefined | null, decimals: number = 2): string {
  const d = toDecimal(value)
  if (d.isNaN() || !d.isFinite()) return `0.${"0".repeat(decimals)}`
  const isNeg = d.isNeg()
  const fixed = d.abs().toFixed(decimals)
  const [intPart, decPart = "0".repeat(decimals)] = fixed.split(".")
  const formatted = intPart.replace(/\B(?=(\d{3})+(?!\d))/g, ",")
  return `${isNeg ? "-" : ""}${formatted}.${decPart}`
}

export function formatPrice(value: string | number | undefined | null, currency: string): string {
  const symbol = CURRENCY_SYMBOLS[currency] || currency
  return `${symbol}${formatDecimal(value, 4)}`
}

export function formatCurrency(value: string | number | undefined | null): string {
  return `${CURRENCY_SYMBOLS["CNY"]}${formatDecimal(value)}`
}

export function formatCurrencyByCode(
  value: string | number | undefined | null,
  currency: string
): string {
  const symbol = CURRENCY_SYMBOLS[currency] || currency
  return `${symbol}${formatDecimal(value)}`
}

export function formatPercent(value: string | number | undefined | null): string {
  const d = toDecimal(value)
  if (d.isNaN() || !d.isFinite()) return "0.00%"
  return `${d.times(100).toFixed(2)}%`
}

export function getProfitColor(isPositive: boolean, colorScheme: ColorScheme): string {
  if (colorScheme === "red-up") {
    return isPositive ? "text-red-600" : "text-emerald-600"
  }
  return isPositive ? "text-emerald-600" : "text-orange-600"
}
