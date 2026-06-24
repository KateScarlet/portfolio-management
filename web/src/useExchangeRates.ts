import { useState, useEffect } from "react"
import { AvailableFund } from "./types"
import * as api from "./api"
import { useToast } from "./components/toast-context"

export function useExchangeRates(availableFunds: AvailableFund[]) {
  const [exchangeRates, setExchangeRates] = useState<Record<string, number>>({ CNY: 1 })
  const { showToast } = useToast()

  useEffect(() => {
    const currencies = availableFunds.filter((f) => f.currency !== "CNY").map((f) => f.currency)
    const unique = [...new Set(currencies)]
    if (unique.length === 0) return

    let cancelled = false

    Promise.all(
      unique.map(async (cur) => {
        try {
          const res = await api.fetchExchangeRate(`${cur}CNY`)
          return { currency: cur, rate: res.rate, ok: true }
        } catch {
          return { currency: cur, rate: 0, ok: false }
        }
      })
    ).then((results) => {
      if (cancelled) return

      const failed = results.filter((r) => !r.ok)
      if (failed.length > 0) {
        showToast(`汇率获取失败: ${failed.map((r) => r.currency).join(", ")}`, "error")
      }

      const rates: Record<string, number> = { CNY: 1 }
      results.forEach((r) => {
        if (r.ok) rates[r.currency] = r.rate
      })
      setExchangeRates((prev) => ({ ...prev, ...rates }))
    })

    return () => {
      cancelled = true
    }
  }, [availableFunds, showToast])

  return exchangeRates
}
