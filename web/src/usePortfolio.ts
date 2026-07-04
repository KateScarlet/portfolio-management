import { useState, useEffect, useCallback } from "react"
import { AssetId, Holding, MergedHolding, PortfolioRecord } from "./types"
import * as api from "./api"
import { toDecimal } from "./utils"

export function usePortfolio(
  portfolioId: string | null,
  displayCurrency: string = "CNY",
  exchangeRates: Record<string, number> = {}
) {
  const [holdings, setHoldings] = useState<MergedHolding[]>([])
  const [history, setHistory] = useState<PortfolioRecord[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    if (!portfolioId) {
      return
    }
    const fetch = async () => {
      setLoading(true)
      try {
        const [h, r] = await Promise.all([
          api.fetchHoldings(portfolioId),
          api.fetchRecords(portfolioId),
        ])
        if (!cancelled) {
          setHoldings(h)
          setHistory(r)
        }
      } catch (e) {
        console.error(e)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    fetch()
    return () => {
      cancelled = true
    }
  }, [portfolioId])

  const assets: Record<AssetId, string> = { stocks: "0", bonds: "0", cash: "0", commodities: "0" }
  holdings.forEach((h) => {
    const rate = h.currency === displayCurrency ? 1 : exchangeRates[h.currency]
    const convertedValue = rate ? toDecimal(h.value).times(rate) : toDecimal(h.value)
    assets[h.assetId] = toDecimal(assets[h.assetId]).plus(convertedValue).toString()
  })

  const refetchHoldings = useCallback(async () => {
    if (!portfolioId) return
    try {
      const h = await api.fetchHoldings(portfolioId)
      setHoldings(h)
    } catch (e) {
      console.error("Failed to refetch holdings", e)
    }
  }, [portfolioId])

  const addHolding = useCallback(
    async (holding: Omit<Holding, "id">) => {
      if (!portfolioId) return
      await api.createHolding(portfolioId, holding)
      await refetchHoldings()
    },
    [portfolioId, refetchHoldings]
  )

  const updateHolding = useCallback(
    async (id: string, updates: Partial<Holding>) => {
      if (!portfolioId) return
      try {
        await api.updateHolding(portfolioId, id, updates)
        await refetchHoldings()
      } catch (e) {
        console.error("Failed to update holding", e)
      }
    },
    [portfolioId, refetchHoldings]
  )

  const removeHolding = useCallback(
    async (id: string) => {
      if (!portfolioId) return
      try {
        await api.deleteHolding(portfolioId, id)
        await refetchHoldings()
      } catch (e) {
        console.error("Failed to remove holding", e)
      }
    },
    [portfolioId, refetchHoldings]
  )

  const saveRecord = useCallback(async () => {
    if (!portfolioId) return
    try {
      const result = await api.createRecord(portfolioId, displayCurrency)
      setHistory((prev) => [result, ...prev])
    } catch (e) {
      console.error("Failed to save record", e)
    }
  }, [portfolioId, displayCurrency])

  const deleteRecord = useCallback(
    async (id: string) => {
      if (!portfolioId) return
      try {
        await api.deleteRecord(portfolioId, id)
        setHistory((prev) => prev.filter((r) => r.id !== id))
      } catch (e) {
        console.error("Failed to delete record", e)
      }
    },
    [portfolioId]
  )

  return {
    holdings,
    setHoldings,
    assets,
    history,
    loading,
    addHolding,
    updateHolding,
    removeHolding,
    saveRecord,
    deleteRecord,
  }
}
