import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import { ColorScheme } from "./types"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatCurrency(value: number): string {
  return new Intl.NumberFormat("zh-CN", {
    style: "currency",
    currency: "CNY",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value)
}

export function formatPercent(value: number): string {
  return new Intl.NumberFormat("zh-CN", {
    style: "percent",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value)
}

export function getProfitColor(isPositive: boolean, colorScheme: ColorScheme): string {
  if (colorScheme === "red-up") {
    return isPositive ? "text-red-600" : "text-emerald-600"
  }
  return isPositive ? "text-emerald-600" : "text-orange-600"
}
