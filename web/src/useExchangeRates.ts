import { useState, useEffect, useRef } from "react"
import { AvailableFund } from "./types"
import * as api from "./api"
import { useToast } from "./components/toast-context"

export function useExchangeRates(
  availableFunds: AvailableFund[],
  holdingCurrencies: string[],
  displayCurrency: string = "CNY"
) {
  const [exchangeRates, setExchangeRates] = useState<Record<string, string>>({
    [displayCurrency]: "1",
  })
  const { showToast } = useToast()
  const prevDisplayCurrency = useRef(displayCurrency)

  useEffect(() => {
    if (prevDisplayCurrency.current !== displayCurrency) {
      prevDisplayCurrency.current = displayCurrency
    }

    const allCurrencies = [...availableFunds.map((f) => f.currency), ...holdingCurrencies].filter(
      (c) => c !== displayCurrency
    )
    const unique = [...new Set(allCurrencies)]
    if (unique.length === 0) return

    let cancelled = false

    Promise.all(
      unique.map(async (cur) => {
        try {
          const res = await api.fetchExchangeRate(`${cur}${displayCurrency}`)
          return { currency: cur, rate: res.rate, ok: true }
        } catch {
          return { currency: cur, rate: "0", ok: false }
        }
      })
    ).then((results) => {
      if (cancelled) return

      const failed = results.filter((r) => !r.ok)
      if (failed.length > 0) {
        showToast(`汇率获取失败: ${failed.map((r) => r.currency).join(", ")}`, "error")
      }

      const rates: Record<string, string> = { [displayCurrency]: "1" }
      results.forEach((r) => {
        if (r.ok) rates[r.currency] = r.rate
      })
      setExchangeRates(rates)
    })

    return () => {
      cancelled = true
    }
  }, [availableFunds, holdingCurrencies, displayCurrency, showToast])

  return exchangeRates
}
