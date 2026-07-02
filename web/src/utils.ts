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

export function formatCurrency(value: string | number | undefined | null): string {
  const d = toDecimal(value)
  return new Intl.NumberFormat("zh-CN", {
    style: "currency",
    currency: "CNY",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(d.toNumber())
}

export function formatCurrencyByCode(value: string | number | undefined | null, currency: string): string {
  const d = toDecimal(value)
  return new Intl.NumberFormat("zh-CN", {
    style: "currency",
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(d.toNumber())
}

export function formatPercent(value: string | number | undefined | null): string {
  const d = toDecimal(value)
  return new Intl.NumberFormat("zh-CN", {
    style: "percent",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(d.toNumber())
}

export function getProfitColor(isPositive: boolean, colorScheme: ColorScheme): string {
  if (colorScheme === "red-up") {
    return isPositive ? "text-red-600" : "text-emerald-600"
  }
  return isPositive ? "text-emerald-600" : "text-orange-600"
}
